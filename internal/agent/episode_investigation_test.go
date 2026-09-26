package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/patflynn/reel-life/internal/sonarr"
)

// Exercise dispatch with real Sonarr HTTP parsing and prohibit accidental writes.
func TestEpisodeInvestigationAndRequestedSearchHTTP(t *testing.T) {
	for _, mode := range []string{"diagnose", "search", "queued", "ambiguous", "file", "unmonitored", "future", "queue-failure", "command-failure", "command-rejected"} {
		t.Run(mode, func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method != "GET" {
					writes++
					if mode == "diagnose" || r.Method != "POST" || r.URL.Path != "/api/v3/command" {
						t.Errorf("unexpected mutation %s %s", r.Method, r.URL.Path)
					}
					var cmd sonarr.CommandRequest
					if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
						t.Error(err)
					}
					if cmd.Name != "EpisodeSearch" || len(cmd.EpisodeIDs) != 1 || cmd.EpisodeIDs[0] != 2 || cmd.SeriesID != 0 {
						t.Errorf("wrong search scope: %+v", cmd)
					}
					if mode == "command-failure" {
						http.Error(w, "unavailable", 503)
						return
					}
					if mode == "command-rejected" {
						_, _ = w.Write([]byte(`{"id":42,"status":"failed"}`))
						return
					}
					_, _ = w.Write([]byte(`{"id":42,"name":"EpisodeSearch","status":"queued"}`))
					return
				}
				var data any
				switch r.URL.Path {
				case "/api/v3/series/1":
					data = sonarr.Series{ID: 1, Monitored: true}
				case "/api/v3/episode":
					if r.URL.Query().Get("seriesId") != "1" {
						t.Error("unscoped episode lookup")
					}
					ep := sonarr.Episode{ID: 2, SeriesID: 1, Monitored: mode != "unmonitored", HasFile: mode == "file", AirDateUTC: "2020-01-01T00:00:00Z"}
					if mode == "future" {
						ep.AirDateUTC = "2099-01-01T00:00:00Z"
					}
					data = []sonarr.Episode{ep}
				case "/api/v3/queue":
					if mode == "queue-failure" {
						http.Error(w, "unavailable", 503)
						return
					}
					q := sonarr.QueuePage{Page: 1, PageSize: 100, Records: []sonarr.QueueItem{}}
					if mode == "queued" || mode == "ambiguous" {
						id := 2
						if mode == "ambiguous" {
							id = 0
						}
						q.Records = []sonarr.QueueItem{{ID: 8, SeriesID: 1, EpisodeID: id}}
						q.TotalRecords = 1
					}
					data = q
				case "/api/v3/history":
					data = sonarr.HistoryPage{}
				case "/api/v3/release":
					if r.URL.Query().Get("episodeId") != "2" {
						t.Error("unscoped release search")
					}
					data = []sonarr.Release{{GUID: "candidate"}}
				default:
					t.Errorf("unexpected endpoint: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(data)
			}))
			defer server.Close()
			a := &Agent{sonarr: sonarr.NewClient(server.URL, "test"), logger: slog.Default()}
			name := "trigger_episode_search"
			if mode == "diagnose" {
				name = "investigate_episode"
			}
			result, handled := a.dispatchSonarr(context.Background(), name, json.RawMessage(`{"series_id":1,"episode_id":2}`))
			success := mode == "diagnose" || mode == "search"
			if !handled || result.Success != success {
				t.Fatalf("result=%+v handled=%v", result, handled)
			}
			wantWrites := 0
			if mode == "search" || mode == "command-failure" || mode == "command-rejected" {
				wantWrites = 1
			}
			if writes != wantWrites {
				t.Fatalf("writes=%d want %d", writes, wantWrites)
			}
			if mode == "command-failure" && result.Retryable {
				t.Fatal("ambiguous acceptance encouraged a repeat command")
			}
		})
	}
	if IsMutative("investigate_episode") || !IsMutative("trigger_episode_search") || IsDestructive("trigger_episode_search") {
		t.Fatal("wrong tool classification")
	}
}
