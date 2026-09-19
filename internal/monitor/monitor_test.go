package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/patflynn/reel-life/internal/events"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/patflynn/reel-life/internal/sonarr"
)

// mockSonarr implements sonarr.Client for testing.
type mockSonarr struct {
	healthFn func() ([]sonarr.HealthCheck, error)
}

func (m *mockSonarr) Search(_ context.Context, _ string) ([]sonarr.Series, error) {
	return nil, nil
}
func (m *mockSonarr) Add(_ context.Context, _ sonarr.AddSeriesRequest) (*sonarr.Series, error) {
	return nil, nil
}
func (m *mockSonarr) Queue(_ context.Context) (*sonarr.QueuePage, error) { return nil, nil }
func (m *mockSonarr) History(_ context.Context, _ int) (*sonarr.HistoryPage, error) {
	return nil, nil
}
func (m *mockSonarr) Health(ctx context.Context) ([]sonarr.HealthCheck, error) {
	return m.healthFn()
}
func (m *mockSonarr) RemoveFailed(_ context.Context, _ int, _ bool) error { return nil }
func (m *mockSonarr) GetSeries(_ context.Context, _ int) (*sonarr.Series, error) {
	return nil, nil
}
func (m *mockSonarr) GetEpisodes(_ context.Context, _ int, _ ...int) ([]sonarr.Episode, error) {
	return nil, nil
}
func (m *mockSonarr) GetLogs(_ context.Context, _ int, _ string) ([]sonarr.LogRecord, error) {
	return nil, nil
}
func (m *mockSonarr) ManualSearch(_ context.Context, _ int) ([]sonarr.Release, error) {
	return nil, nil
}
func (m *mockSonarr) GetQualityProfiles(_ context.Context) ([]sonarr.QualityProfile, error) {
	return nil, nil
}
func (m *mockSonarr) GetBlocklist(_ context.Context, _ int) (*sonarr.BlocklistPage, error) {
	return nil, nil
}
func (m *mockSonarr) GetRootFolders(_ context.Context) ([]sonarr.RootFolder, error) {
	return nil, nil
}
func (m *mockSonarr) GetDownloadClients(_ context.Context) ([]sonarr.DownloadClient, error) {
	return nil, nil
}
func (m *mockSonarr) UpdateSeries(_ context.Context, series *sonarr.Series) (*sonarr.Series, error) {
	return series, nil
}
func (m *mockSonarr) Command(_ context.Context, _ sonarr.CommandRequest) (*sonarr.CommandResource, error) {
	return &sonarr.CommandResource{}, nil
}
func (m *mockSonarr) DeleteSeries(_ context.Context, _ int, _ bool) error { return nil }
func (m *mockSonarr) DeleteBlocklistItem(_ context.Context, _ int) error  { return nil }
func (m *mockSonarr) GrabRelease(_ context.Context, _ string, _ int) (*sonarr.Release, error) {
	return &sonarr.Release{}, nil
}
func (m *mockSonarr) MonitorEpisodes(_ context.Context, _ []int, _ bool) error { return nil }
func (m *mockSonarr) GetLanguageProfiles(_ context.Context) ([]sonarr.LanguageProfile, error) {
	return nil, nil
}

// mockNotifier records sent messages.
type mockNotifier struct {
	mu       sync.Mutex
	messages []string
	threads  []string
}

func (n *mockNotifier) Send(_ context.Context, msg string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.messages = append(n.messages, msg)
	return nil
}

func (n *mockNotifier) SendThread(_ context.Context, msg string, thread string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.messages = append(n.messages, msg)
	n.threads = append(n.threads, thread)
	return nil
}

func (n *mockNotifier) SendAdmin(_ context.Context, msg string, thread string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.messages = append(n.messages, msg)
	n.threads = append(n.threads, thread)
	return nil
}

func TestMonitorAlertsOnNewIssues(t *testing.T) {
	sonarrMock := &mockSonarr{
		healthFn: func() ([]sonarr.HealthCheck, error) {
			return []sonarr.HealthCheck{
				{Source: "IndexerStatusCheck", Type: "warning", Message: "Indexer unavailable"},
			}, nil
		},
	}
	notifier := &mockNotifier{}
	mon := New(sonarrMock, notifier, 50*time.Millisecond, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	mon.Run(ctx)

	notifier.mu.Lock()
	defer notifier.mu.Unlock()

	if len(notifier.messages) == 0 {
		t.Fatal("expected at least one alert message")
	}
	if len(notifier.threads) == 0 || notifier.threads[0] != "sonarr-health" {
		t.Error("expected alert in sonarr-health thread")
	}
}

func TestMonitorDoesNotRepeatAlerts(t *testing.T) {
	sonarrMock := &mockSonarr{
		healthFn: func() ([]sonarr.HealthCheck, error) {
			return []sonarr.HealthCheck{
				{Source: "DiskCheck", Type: "error", Message: "Low disk space"},
			}, nil
		},
	}
	notifier := &mockNotifier{}
	mon := New(sonarrMock, notifier, 50*time.Millisecond, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	mon.Run(ctx)

	notifier.mu.Lock()
	defer notifier.mu.Unlock()

	// Should only alert once even though health returns same issue multiple times
	if len(notifier.messages) != 1 {
		t.Errorf("expected exactly 1 alert, got %d", len(notifier.messages))
	}
}

func TestMonitorNoAlertWhenHealthy(t *testing.T) {
	sonarrMock := &mockSonarr{
		healthFn: func() ([]sonarr.HealthCheck, error) {
			return []sonarr.HealthCheck{}, nil
		},
	}
	notifier := &mockNotifier{}
	mon := New(sonarrMock, notifier, 50*time.Millisecond, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	mon.Run(ctx)

	notifier.mu.Lock()
	defer notifier.mu.Unlock()

	if len(notifier.messages) != 0 {
		t.Errorf("expected no alerts when healthy, got %d", len(notifier.messages))
	}
}

type evidenceRecorder struct {
	entries []events.Event
	fail    bool
}

func (r *evidenceRecorder) Record(_ context.Context, e events.Event) error {
	if r.fail {
		return fmt.Errorf("disk unavailable")
	}
	r.entries = append(r.entries, e)
	return nil
}

type remediatorFunc func(context.Context)

func (f remediatorFunc) RunOnce(ctx context.Context) { f(ctx) }

type failingNotifier struct{ mockNotifier }

func (*failingNotifier) SendAdmin(context.Context, string, string) error {
	return fmt.Errorf("PRIVATE_CANARY")
}

func TestPollEvidence(t *testing.T) {
	for _, mode := range []string{"healthy", "health-failed", "notification-failed", "notification-succeeded", "recorder-failed"} {
		t.Run(mode, func(t *testing.T) {
			recorder := &evidenceRecorder{fail: mode == "recorder-failed"}
			client := &mockSonarr{healthFn: func() ([]sonarr.HealthCheck, error) {
				if mode == "health-failed" {
					return nil, fmt.Errorf("PRIVATE_CANARY")
				}
				if mode == "notification-failed" || mode == "notification-succeeded" {
					return []sonarr.HealthCheck{{Type: "warning", Source: "PRIVATE_CANARY", Message: "PRIVATE_CANARY"}}, nil
				}
				return nil, nil
			}}
			mon := New(client, &failingNotifier{}, time.Minute, slog.Default())
			if mode == "notification-succeeded" {
				mon.notifier = &mockNotifier{}
			}
			mon.SetEventRecorder(recorder)
			called := false
			mon.SetRemediator(remediatorFunc(func(context.Context) {
				called = true
				for _, e := range recorder.entries {
					if e.Type == "monitor.poll.completed" {
						t.Fatal("poll completed before remediation returned")
					}
				}
			}))
			mon.check(context.Background())
			if !called {
				t.Fatal("remediator skipped")
			}
			if mode == "recorder-failed" {
				return
			}
			want := []string{"monitor.poll.started:", "monitor.health:succeeded", "monitor.poll.completed:"}
			if mode == "health-failed" {
				want[1] = "monitor.health:failed"
			}
			if mode == "notification-failed" {
				want = []string{want[0], want[1], "monitor.notification:failed", want[2]}
			}
			if mode == "notification-succeeded" {
				want = []string{want[0], want[1], "monitor.notification:succeeded", want[2]}
			}
			var got []string
			for _, e := range recorder.entries {
				got = append(got, e.Type+":"+e.Outcome)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v want %v", got, want)
			}
			data, _ := json.Marshal(recorder.entries)
			if strings.Contains(string(data), "PRIVATE_CANARY") {
				t.Fatal("private data in evidence")
			}
		})
	}
}
