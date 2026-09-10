package remediation

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/patflynn/reel-life/internal/events"
	"github.com/patflynn/reel-life/internal/sonarr"
)

type fakeSonarr struct {
	mu        sync.Mutex
	records   []sonarr.QueueItem
	removeErr error
	removals  int
	blocklist bool
}

func (f *fakeSonarr) Queue(context.Context) (*sonarr.QueuePage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := append([]sonarr.QueueItem(nil), f.records...)
	return &sonarr.QueuePage{Records: items}, nil
}

func (f *fakeSonarr) RemoveFailed(_ context.Context, id int, blocklist bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removals++
	f.blocklist = blocklist
	if f.removeErr != nil {
		return f.removeErr
	}
	for i, item := range f.records {
		if item.ID == id {
			f.records = append(f.records[:i], f.records[i+1:]...)
			break
		}
	}
	return nil
}

type fakeNotifier struct {
	mu       sync.Mutex
	messages []string
}

func (n *fakeNotifier) Send(context.Context, string) error               { return nil }
func (n *fakeNotifier) SendThread(context.Context, string, string) error { return nil }
func (n *fakeNotifier) SendAdmin(_ context.Context, message, _ string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.messages = append(n.messages, message)
	return nil
}

type memoryRecorder struct {
	mu     sync.Mutex
	events []events.Event
}

func (r *memoryRecorder) Record(_ context.Context, event events.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func TestRunnerRemovesBlocklistsAndVerifiesFailedDownload(t *testing.T) {
	client := &fakeSonarr{records: []sonarr.QueueItem{{ID: 42, SeriesID: 7, Title: "bad.release", TrackedDownloadState: "downloadFailed"}}}
	notifier := &fakeNotifier{}
	recorder := &memoryRecorder{}
	New(client, notifier, recorder, slog.Default(), 1, time.Hour).RunOnce(context.Background())

	if client.removals != 1 || !client.blocklist {
		t.Fatalf("removals=%d blocklist=%v", client.removals, client.blocklist)
	}
	if len(notifier.messages) != 1 {
		t.Fatalf("notifications=%d, want 1", len(notifier.messages))
	}
	if got := recorder.events[len(recorder.events)-1]; got.Type != "remediation.verified" || got.Outcome != "resolved" {
		t.Fatalf("last event=%+v", got)
	}
}

func TestRunnerIgnoresNonFailedDownloads(t *testing.T) {
	client := &fakeSonarr{records: []sonarr.QueueItem{{ID: 1, Status: "downloading", TrackedDownloadStatus: "ok"}}}
	New(client, &fakeNotifier{}, &memoryRecorder{}, slog.Default(), 1, 0).RunOnce(context.Background())
	if client.removals != 0 {
		t.Fatalf("removals=%d, want 0", client.removals)
	}
}

func TestRunnerEscalatesAndBoundsFailedAction(t *testing.T) {
	client := &fakeSonarr{
		records:   []sonarr.QueueItem{{ID: 42, Title: "bad.release", Status: "failed"}},
		removeErr: errors.New("sonarr unavailable"),
	}
	notifier := &fakeNotifier{}
	runner := New(client, notifier, &memoryRecorder{}, slog.Default(), 1, 0)
	runner.RunOnce(context.Background())
	runner.RunOnce(context.Background())
	if client.removals != 1 {
		t.Fatalf("removals=%d, want bounded single attempt", client.removals)
	}
	if len(notifier.messages) != 1 {
		t.Fatalf("notifications=%d, want 1", len(notifier.messages))
	}
}
