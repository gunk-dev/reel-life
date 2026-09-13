package evaluation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/patflynn/reel-life/internal/evaluation"
	"github.com/patflynn/reel-life/internal/events"
	"github.com/patflynn/reel-life/internal/remediation"
	"github.com/patflynn/reel-life/internal/simarr"
	"github.com/patflynn/reel-life/internal/sonarr"
)

type scenario struct {
	Name          string             `json:"name"`
	Queue         []sonarr.QueueItem `json:"queue"`
	Polls         int                `json:"polls"`
	MaxAttempts   int                `json:"max_attempts"`
	Cooldown      string             `json:"cooldown"`
	DeleteStatus  int                `json:"delete_status"`
	QueueStatuses []int              `json:"queue_statuses"`
	RetainDeleted bool               `json:"retain_deleted"`
	Want          struct {
		Deletes      int      `json:"deletes"`
		RemainingIDs []int    `json:"remaining_ids"`
		Detected     int      `json:"detected"`
		Resolved     int      `json:"resolved"`
		Escalated    int      `json:"escalated"`
		Messages     []string `json:"messages"`
		Events       []string `json:"events"`
	} `json:"want"`
}

type recorder struct {
	ledger      bytes.Buffer
	transitions []string
}

func (r *recorder) Record(_ context.Context, event events.Event) error {
	r.transitions = append(r.transitions, event.Type+":"+event.Outcome)
	return json.NewEncoder(&r.ledger).Encode(event)
}

type notifier struct{ messages []string }

func (n *notifier) Send(context.Context, string) error               { return nil }
func (n *notifier) SendThread(context.Context, string, string) error { return nil }
func (n *notifier) SendAdmin(_ context.Context, message, _ string) error {
	n.messages = append(n.messages, message)
	return nil
}

// TestFrozenRemediation runs the versioned product contract through the real
// HTTP client, policy runner, evidence encoding, and outcome report.
func TestFrozenRemediation(t *testing.T) {
	data, err := os.ReadFile("testdata/remediation-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []scenario
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("empty evaluation suite")
	}
	names := make(map[string]bool)
	for _, tc := range cases {
		if tc.Name == "" || names[tc.Name] {
			t.Fatalf("missing or duplicate scenario name %q", tc.Name)
		}
		names[tc.Name] = true
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Polls < 1 {
				t.Fatal("scenario must run at least one poll")
			}
			cooldown, err := time.ParseDuration(tc.Cooldown)
			if err != nil {
				t.Fatal(err)
			}
			stack := simarr.NewSonarr(tc.Queue)
			defer stack.Close()
			stack.FailDeletes(tc.DeleteStatus)
			stack.QueueStatuses(tc.QueueStatuses...)
			if tc.RetainDeleted {
				stack.RetainDeleted()
			}
			client := sonarr.NewClient(stack.URL(), "evaluation-key")
			evidence := &recorder{}
			notifications := &notifier{}
			runner := remediation.New(client, notifications, evidence, slog.New(slog.NewTextHandler(io.Discard, nil)), tc.MaxAttempts, cooldown)
			for i := 0; i < tc.Polls; i++ {
				runner.RunOnce(context.Background())
			}
			deletes, blocklisted := stack.DeleteResult()
			if deletes != tc.Want.Deletes {
				t.Errorf("delete requests = %d, want %d", deletes, tc.Want.Deletes)
			}
			if deletes > 0 && !blocklisted {
				t.Error("failed release was not blocklisted")
			}
			queue, err := client.Queue(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var remainingIDs []int
			for _, item := range queue.Records {
				remainingIDs = append(remainingIDs, item.ID)
			}
			slices.Sort(remainingIDs)
			if !slices.Equal(remainingIDs, tc.Want.RemainingIDs) {
				t.Errorf("remaining queue IDs = %v, want %v", remainingIDs, tc.Want.RemainingIDs)
			}
			if !reflect.DeepEqual(evidence.transitions, tc.Want.Events) {
				t.Errorf("transitions = %v, want %v", evidence.transitions, tc.Want.Events)
			}
			if len(notifications.messages) != len(tc.Want.Messages) {
				t.Errorf("notifications = %v, want %v", notifications.messages, tc.Want.Messages)
			} else {
				for i, fragment := range tc.Want.Messages {
					if !strings.Contains(notifications.messages[i], fragment) {
						t.Errorf("notification %q does not contain %q", notifications.messages[i], fragment)
					}
				}
			}
			report, err := evaluation.Build(&evidence.ledger)
			if err != nil {
				t.Fatal(err)
			}
			if report.RemediationsDetected != tc.Want.Detected || report.RemediationsResolved != tc.Want.Resolved || report.RemediationsEscalated != tc.Want.Escalated {
				t.Errorf("incident outcomes = detected:%d resolved:%d escalated:%d, want %d/%d/%d", report.RemediationsDetected, report.RemediationsResolved, report.RemediationsEscalated, tc.Want.Detected, tc.Want.Resolved, tc.Want.Escalated)
			}
		})
	}
}
