package remediation

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/patflynn/reel-life/internal/events"
	"github.com/patflynn/reel-life/internal/simarr"
	"github.com/patflynn/reel-life/internal/sonarr"
)

func TestPersistentRunnerBlocksInterruptedOrCompletedAttempts(t *testing.T) {
	for _, stage := range []string{"planned", "executed", "resolved", "verification_failed"} {
		t.Run(stage, func(t *testing.T) {
			stack := simarr.NewSonarr([]sonarr.QueueItem{{ID: 91, Status: "failed"}})
			defer stack.Close()
			ledger := events.NewFileRecorder(filepath.Join(t.TempDir(), "events.jsonl"))
			record := func(kind, outcome string) {
				t.Helper()
				if err := ledger.Record(context.Background(), events.Event{Type: kind, Outcome: outcome, Operation: failedDownloadPolicy, CorrelationID: "sonarr.queue.91", Timestamp: time.Now().Add(-2 * time.Hour)}); err != nil {
					t.Fatal(err)
				}
			}
			record("remediation.planned", "approved_by_policy")
			if stage != "planned" {
				record("remediation.action", "executed")
			}
			if stage == "resolved" {
				record("remediation.verified", "resolved")
			}
			if stage == "verification_failed" {
				record("remediation.verification", "escalated")
			}
			// More attempts and an expired cooldown must not authorize a retry when
			// the previous attempt is interrupted, ambiguous, or already resolved.
			runner, err := NewPersistent(sonarr.NewClient(stack.URL(), "test-key"), &fakeNotifier{}, ledger, slog.Default(), 3, time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			runner.RunOnce(context.Background())
			count, _ := stack.DeleteResult()
			if count != 0 {
				t.Fatalf("repeated %s attempt: %d mutations", stage, count)
			}
		})
	}
}

func TestPersistentRunnerRestoresBudgetAndAllowsRecordedFailureAfterCooldown(t *testing.T) {
	stack := simarr.NewSonarr([]sonarr.QueueItem{{ID: 91, Status: "failed"}})
	defer stack.Close()
	stack.FailDeletes(http.StatusServiceUnavailable)
	ledger := events.NewFileRecorder(filepath.Join(t.TempDir(), "events.jsonl"))
	for _, e := range []events.Event{
		{Type: "remediation.planned", Outcome: "approved_by_policy"},
		{Type: "remediation.action", Outcome: "failed"},
		{Type: "remediation.verification", Outcome: "escalated"},
	} {
		e.Operation, e.CorrelationID, e.Timestamp = failedDownloadPolicy, "sonarr.queue.91", time.Now().Add(-2*time.Hour)
		if err := ledger.Record(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	newRunner := func() *Runner {
		t.Helper()
		runner, err := NewPersistent(sonarr.NewClient(stack.URL(), "test-key"), &fakeNotifier{}, ledger, slog.Default(), 2, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		return runner
	}
	newRunner().RunOnce(context.Background())
	newRunner().RunOnce(context.Background())
	count, _ := stack.DeleteResult()
	if count != 1 {
		t.Fatalf("new mutations = %d, want one remaining attempt", count)
	}
}

func TestPersistentRunnerSerializesConcurrentPolls(t *testing.T) {
	stack := simarr.NewSonarr([]sonarr.QueueItem{{ID: 91, Status: "failed"}})
	defer stack.Close()
	stack.RetainDeleted()
	ledger := events.NewFileRecorder(filepath.Join(t.TempDir(), "events.jsonl"))
	runner, err := NewPersistent(sonarr.NewClient(stack.URL(), "test-key"), &fakeNotifier{}, ledger, slog.Default(), 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); runner.RunOnce(context.Background()) }()
	}
	wg.Wait()
	count, _ := stack.DeleteResult()
	if count != 1 {
		t.Fatalf("concurrent mutations = %d, want one", count)
	}
}

type failingRecorder struct {
	failType string
	delegate *events.FileRecorder
}

func (r failingRecorder) Record(ctx context.Context, event events.Event) error {
	if event.Type == r.failType {
		return errors.New("injected evidence failure")
	}
	return r.delegate.Record(ctx, event)
}

func TestRunnerDoesNotMutateWithoutPreActionEvidence(t *testing.T) {
	for _, kind := range []string{"remediation.detected", "remediation.planned"} {
		t.Run(kind, func(t *testing.T) {
			stack := simarr.NewSonarr([]sonarr.QueueItem{{ID: 91, Status: "failed"}})
			defer stack.Close()
			ledger := events.NewFileRecorder(filepath.Join(t.TempDir(), "events.jsonl"))
			runner := New(sonarr.NewClient(stack.URL(), "test-key"), &fakeNotifier{}, failingRecorder{kind, ledger}, slog.Default(), 3, 0)
			runner.RunOnce(context.Background())
			runner.RunOnce(context.Background())
			count, _ := stack.DeleteResult()
			if count != 0 {
				t.Fatalf("mutations without %s evidence = %d", kind, count)
			}
		})
	}
}

func TestPersistentRunnerRejectsUnreadableOrIncompleteHistory(t *testing.T) {
	for _, contents := range []string{
		"not json\n",
		`{"schema_version":1,"type":"remediation.planned"}`,
		`{"schema_version":2,"type":"remediation.planned"}` + "\n",
		`{"schema_version":1,"type":"remediation.planned","operation":"sonarr.failed-download","outcome":"approved_by_policy"}` + "\n",
		`{"schema_version":1,"timestamp":"2026-09-13T00:00:00Z","type":"remediation.action","operation":"sonarr.failed-download","correlation_id":"sonarr.queue.91","outcome":"failed"}` + "\n",
	} {
		t.Run(contents, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "events.jsonl")
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			runner, err := NewPersistent(nil, &fakeNotifier{}, events.NewFileRecorder(path), slog.Default(), 1, 0)
			if err == nil || runner != nil {
				t.Fatal("accepted unsafe recovery history")
			}
		})
	}
	// Opening a directory succeeds on Unix, but reading its contents must fail.
	if runner, err := NewPersistent(nil, &fakeNotifier{}, events.NewFileRecorder(t.TempDir()), slog.Default(), 1, 0); err == nil || runner != nil {
		t.Fatal("accepted unreadable ledger")
	}
}

// An append can reach disk even when its caller sees an error (for example,
// during sync or close). Recovery must still treat the plan as a reservation.
type uncertainPlanRecorder struct{ ledger *events.FileRecorder }

func (r uncertainPlanRecorder) Record(ctx context.Context, e events.Event) error {
	if err := r.ledger.Record(ctx, e); err != nil {
		return err
	}
	if e.Type == "remediation.planned" {
		return errors.New("injected post-append failure")
	}
	return nil
}

func TestPersistentRunnerBlocksPlanWhoseWriteReturnedAnError(t *testing.T) {
	stack := simarr.NewSonarr([]sonarr.QueueItem{{ID: 91, Status: "failed"}})
	defer stack.Close()
	client := sonarr.NewClient(stack.URL(), "test-key")
	ledger := events.NewFileRecorder(filepath.Join(t.TempDir(), "events.jsonl"))
	first := New(client, &fakeNotifier{}, uncertainPlanRecorder{ledger}, slog.Default(), 3, 0)
	first.RunOnce(context.Background())
	first.RunOnce(context.Background())
	restarted, err := NewPersistent(client, &fakeNotifier{}, ledger, slog.Default(), 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	restarted.RunOnce(context.Background())
	count, _ := stack.DeleteResult()
	if count != 0 {
		t.Fatalf("uncertain reservation caused %d mutations", count)
	}
}
