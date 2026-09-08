package remediation

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/patflynn/reel-life/internal/simarr"
	"github.com/patflynn/reel-life/internal/sonarr"
)

func TestIntegrationRealSonarrClientResolvesFailedDownload(t *testing.T) {
	stack := simarr.NewSonarr([]sonarr.QueueItem{{ID: 91, SeriesID: 4, Title: "broken.release", TrackedDownloadState: "downloadFailed"}})
	defer stack.Close()
	notifier := &fakeNotifier{}
	recorder := &memoryRecorder{}

	New(sonarr.NewClient(stack.URL(), "test-key"), notifier, recorder, slog.Default(), 1, time.Hour).RunOnce(context.Background())

	count, blocklisted := stack.DeleteResult()
	if count != 1 || !blocklisted {
		t.Fatalf("delete count=%d blocklisted=%v", count, blocklisted)
	}
	if len(notifier.messages) != 1 || !strings.Contains(notifier.messages[0], "verified") {
		t.Fatalf("unexpected notification: %#v", notifier.messages)
	}
}

func TestIntegrationDownstreamFailureEscalatesWithoutRetryStorm(t *testing.T) {
	stack := simarr.NewSonarr([]sonarr.QueueItem{{ID: 91, Title: "broken.release", Status: "failed"}})
	defer stack.Close()
	stack.FailDeletes(http.StatusServiceUnavailable)
	notifier := &fakeNotifier{}
	runner := New(sonarr.NewClient(stack.URL(), "test-key"), notifier, &memoryRecorder{}, slog.Default(), 1, 0)

	runner.RunOnce(context.Background())
	runner.RunOnce(context.Background())

	count, _ := stack.DeleteResult()
	if count != 1 {
		t.Fatalf("delete count=%d, want one bounded attempt", count)
	}
	if len(notifier.messages) != 1 || !strings.Contains(notifier.messages[0], "couldn't remove") {
		t.Fatalf("unexpected escalation: %#v", notifier.messages)
	}
}
