package remediation

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/patflynn/reel-life/internal/events"
	"github.com/patflynn/reel-life/internal/sonarr"
)

type reconciliationClient struct {
	fakeSonarr
	queueErr error
}

func (c *reconciliationClient) Queue(ctx context.Context) (*sonarr.QueuePage, error) {
	if c.queueErr != nil {
		return nil, c.queueErr
	}
	return c.fakeSonarr.Queue(ctx)
}

func TestReconciliationAcrossRestarts(t *testing.T) {
	for _, executed := range []bool{false, true} {
		for _, present := range []bool{false, true} {
			name := "planned"
			if executed {
				name = "executed"
			}
			if present {
				name += "-present"
			}
			t.Run(name, func(t *testing.T) {
				ctx := context.Background()
				ledger := events.NewFileRecorder(filepath.Join(t.TempDir(), "events.jsonl"))
				seed := func(typ, outcome string) {
					t.Helper()
					if err := ledger.Record(ctx, events.Event{Type: typ, Outcome: outcome, Operation: failedDownloadPolicy, CorrelationID: "sonarr.queue.91", Timestamp: time.Now().Add(-time.Hour)}); err != nil {
						t.Fatal(err)
					}
				}
				seed("remediation.planned", "approved_by_policy")
				if executed {
					seed("remediation.action", "executed")
					seed("remediation.verification", "escalated")
				}
				client := &reconciliationClient{}
				if present {
					client.records = []sonarr.QueueItem{{ID: 91, Status: "failed"}}
				}
				notifier := &fakeNotifier{}
				newRunner := func() *Runner {
					t.Helper()
					r, err := NewPersistent(client, notifier, ledger, slog.Default(), 5, 0)
					if err != nil {
						t.Fatal(err)
					}
					return r
				}
				// Failed reads cannot resolve an incident or authorize a repeat mutation.
				client.queueErr = errors.New("unavailable")
				newRunner().RunOnce(ctx)
				client.queueErr = nil
				newRunner().RunOnce(ctx)
				newRunner().RunOnce(ctx)
				count := func(typ string) int {
					n := 0
					if err := ledger.Replay(func(e events.Event) error {
						if e.Type == typ {
							n++
						}
						return nil
					}); err != nil {
						t.Fatal(err)
					}
					return n
				}
				if count("remediation.reconciled") != 1 {
					t.Fatal("observation missing or repeated after restart")
				}
				expected := 0
				if executed && !present {
					expected = 1
				}
				if count("remediation.verified") != expected {
					t.Fatal("unproven or missing resolution")
				}
				if present {
					client.records = nil
					newRunner().RunOnce(ctx)
					newRunner().RunOnce(ctx)
					if count("remediation.reconciled") != 2 {
						t.Fatal("absence transition missing or duplicated")
					}
				}
				if len(notifier.messages) != 1 {
					t.Fatalf("notifications=%d", len(notifier.messages))
				}
				// Queue ID reuse must not reauthorize a mutation, even after reconciliation.
				client.records = []sonarr.QueueItem{{ID: 91, Status: "failed"}}
				newRunner().RunOnce(ctx)
				if client.removals != 0 {
					t.Fatal("reconciliation repeated a mutation")
				}
			})
		}
	}
}

func TestReconciliationEvidenceFailureRetainsPending(t *testing.T) {
	for _, typ := range []string{"remediation.reconciled", "remediation.verified"} {
		t.Run(typ, func(t *testing.T) {
			ctx := context.Background()
			ledger := events.NewFileRecorder(filepath.Join(t.TempDir(), "events.jsonl"))
			client := &fakeSonarr{}
			notifier := &fakeNotifier{}
			r := New(client, notifier, failingRecorder{typ, ledger}, slog.Default(), 5, 0)
			r.pending["sonarr.queue.91"] = reconciliation{executed: true}
			r.blocked["sonarr.queue.91"] = true
			r.RunOnce(ctx)
			if len(r.pending) != 1 || len(notifier.messages) != 0 {
				t.Fatal("failed evidence lost pending work or announced success")
			}
			r.events = ledger
			r.RunOnce(ctx)
			if len(r.pending) != 0 || len(notifier.messages) != 1 || client.removals != 0 {
				t.Fatal("read-only reconciliation did not recover")
			}
		})
	}
}
