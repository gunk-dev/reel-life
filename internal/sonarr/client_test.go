package sonarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/series/lookup" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("term") != "breaking bad" {
			t.Errorf("unexpected term: %s", r.URL.Query().Get("term"))
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("missing or wrong API key header")
		}
		json.NewEncoder(w).Encode([]Series{
			{ID: 1, Title: "Breaking Bad", Year: 2008, TVDBID: 81189},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	results, err := client.Search(context.Background(), "breaking bad")
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Breaking Bad" {
		t.Errorf("Title = %q, want %q", results[0].Title, "Breaking Bad")
	}
}

func TestAdd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/series" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req AddSeriesRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.TVDBID != 81189 {
			t.Errorf("TVDBID = %d, want 81189", req.TVDBID)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Series{ID: 42, Title: req.Title, TVDBID: req.TVDBID})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	series, err := client.Add(context.Background(), AddSeriesRequest{
		Title:  "Breaking Bad",
		TVDBID: 81189,
	})
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if series.ID != 42 {
		t.Errorf("ID = %d, want 42", series.ID)
	}
}

func TestQueue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/queue" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(QueuePage{
			TotalRecords: 1,
			Records: []QueueItem{
				{ID: 1, Title: "S01E01", Status: "downloading"},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	queue, err := client.Queue(context.Background())
	if err != nil {
		t.Fatalf("Queue() error: %v", err)
	}
	if queue.TotalRecords != 1 {
		t.Errorf("TotalRecords = %d, want 1", queue.TotalRecords)
	}
}

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]HealthCheck{
			{Source: "IndexerStatusCheck", Type: "warning", Message: "Indexer is unavailable"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	checks, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error: %v", err)
	}
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Type != "warning" {
		t.Errorf("Type = %q, want %q", checks[0].Type, "warning")
	}
}

func TestRemoveFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/queue/123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("blocklist") != "true" {
			t.Errorf("blocklist = %s, want true", r.URL.Query().Get("blocklist"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	err := client.RemoveFailed(context.Background(), 123, true)
	if err != nil {
		t.Fatalf("RemoveFailed() error: %v", err)
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	_, err := client.Health(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestGetSeries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/series/1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(Series{ID: 1, Title: "Breaking Bad", EpisodeCount: 62, EpisodeFileCount: 62, SizeOnDisk: 100000})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	series, err := client.GetSeries(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetSeries() error: %v", err)
	}
	if series.Title != "Breaking Bad" || series.EpisodeCount != 62 {
		t.Errorf("unexpected series: %+v", series)
	}
}

func TestGetEpisodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/episode" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("seriesId") != "1" {
			t.Errorf("seriesId = %s, want 1", r.URL.Query().Get("seriesId"))
		}
		json.NewEncoder(w).Encode([]Episode{
			{ID: 1, SeasonNumber: 1, EpisodeNumber: 1, Title: "Pilot", HasFile: true},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	episodes, err := client.GetEpisodes(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetEpisodes() error: %v", err)
	}
	if len(episodes) != 1 || episodes[0].Title != "Pilot" {
		t.Errorf("unexpected episodes: %+v", episodes)
	}
}

func TestGetLogs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/log" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("sortDirection") != "descending" {
			t.Errorf("sortDirection = %s, want descending", r.URL.Query().Get("sortDirection"))
		}
		if r.URL.Query().Get("pageSize") != "10" {
			t.Errorf("pageSize = %s, want 10", r.URL.Query().Get("pageSize"))
		}
		if r.URL.Query().Get("filterValue") != "error" {
			t.Errorf("filterValue = %s, want error", r.URL.Query().Get("filterValue"))
		}
		json.NewEncoder(w).Encode(LogPage{
			Records: []LogRecord{
				{Time: "2026-01-01", Level: "error", Logger: "Test", Message: "test error"},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	logs, err := client.GetLogs(context.Background(), 10, "error")
	if err != nil {
		t.Fatalf("GetLogs() error: %v", err)
	}
	if len(logs) != 1 || logs[0].Level != "error" {
		t.Errorf("unexpected logs: %+v", logs)
	}
}

func TestManualSearch(t *testing.T) {
	// Fixture matches the real Sonarr /api/v3/release response shape:
	// "quality" is a nested object, not a string.
	const fixture = `[
		{
			"guid": "xyz789",
			"indexerId": 5,
			"title": "Breaking.Bad.S01E01.720p.HDTV.x264",
			"indexer": "NZBgeek",
			"quality": {
				"quality": {
					"id": 4,
					"name": "HDTV-720p",
					"source": "television",
					"resolution": 720,
					"modifier": "none"
				},
				"revision": {
					"version": 1,
					"real": 0,
					"isRepack": false
				}
			},
			"size": 1400000000,
			"age": 1,
			"rejected": false
		}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/release" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("episodeId") != "42" {
			t.Errorf("episodeId = %s, want 42", r.URL.Query().Get("episodeId"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixture))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	releases, err := client.ManualSearch(context.Background(), 42)
	if err != nil {
		t.Fatalf("ManualSearch() error: %v", err)
	}
	if len(releases) != 1 || releases[0].Indexer != "NZBgeek" {
		t.Fatalf("unexpected releases: %+v", releases)
	}
	got := releases[0].Quality.Definition.Name
	if got != "HDTV-720p" {
		t.Errorf("Quality.Definition.Name = %q, want %q", got, "HDTV-720p")
	}
	if releases[0].Quality.Definition.Resolution != 720 {
		t.Errorf("Quality.Definition.Resolution = %d, want 720", releases[0].Quality.Definition.Resolution)
	}
	if releases[0].Quality.Revision.Version != 1 {
		t.Errorf("Quality.Revision.Version = %d, want 1", releases[0].Quality.Revision.Version)
	}
}

func TestGetQualityProfiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/qualityprofile" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]QualityProfile{
			{ID: 1, Name: "HD-1080p", Cutoff: 7},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	profiles, err := client.GetQualityProfiles(context.Background())
	if err != nil {
		t.Fatalf("GetQualityProfiles() error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "HD-1080p" {
		t.Errorf("unexpected profiles: %+v", profiles)
	}
}

func TestGetBlocklist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/blocklist" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("pageSize") != "5" {
			t.Errorf("pageSize = %s, want 5", r.URL.Query().Get("pageSize"))
		}
		json.NewEncoder(w).Encode(BlocklistPage{
			TotalRecords: 1,
			Records: []BlocklistItem{
				{ID: 1, SeriesID: 1, SourceTitle: "Bad.Release", Date: "2026-01-01"},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	page, err := client.GetBlocklist(context.Background(), 5)
	if err != nil {
		t.Fatalf("GetBlocklist() error: %v", err)
	}
	if page.TotalRecords != 1 {
		t.Errorf("TotalRecords = %d, want 1", page.TotalRecords)
	}
}

func TestGetRootFolders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/rootfolder" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]RootFolder{
			{Path: "/tv", FreeSpace: 500000000000, TotalSpace: 1000000000000},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	folders, err := client.GetRootFolders(context.Background())
	if err != nil {
		t.Fatalf("GetRootFolders() error: %v", err)
	}
	if len(folders) != 1 || folders[0].Path != "/tv" {
		t.Errorf("unexpected folders: %+v", folders)
	}
}

func TestGetDownloadClients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/downloadclient" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]DownloadClient{
			{Name: "SABnzbd", Enable: true, Protocol: "usenet", Priority: 1},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	clients, err := client.GetDownloadClients(context.Background())
	if err != nil {
		t.Fatalf("GetDownloadClients() error: %v", err)
	}
	if len(clients) != 1 || clients[0].Name != "SABnzbd" {
		t.Errorf("unexpected clients: %+v", clients)
	}
}

func TestUpdateSeries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/series/1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var s Series
		json.NewDecoder(r.Body).Decode(&s)
		if len(s.Seasons) != 2 {
			t.Fatalf("expected 2 seasons, got %d", len(s.Seasons))
		}
		if !s.Seasons[1].Monitored {
			t.Errorf("expected season 2 to be monitored")
		}

		json.NewEncoder(w).Encode(s)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	series := &Series{
		ID:    1,
		Title: "Breaking Bad",
		Seasons: []Season{
			{SeasonNumber: 1, Monitored: true},
			{SeasonNumber: 2, Monitored: true},
		},
	}
	updated, err := client.UpdateSeries(context.Background(), series)
	if err != nil {
		t.Fatalf("UpdateSeries() error: %v", err)
	}
	if updated.Title != "Breaking Bad" {
		t.Errorf("Title = %q, want %q", updated.Title, "Breaking Bad")
	}
	if len(updated.Seasons) != 2 {
		t.Errorf("expected 2 seasons, got %d", len(updated.Seasons))
	}
}

func TestHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/history" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("pageSize") != "5" {
			t.Errorf("pageSize = %s, want 5", r.URL.Query().Get("pageSize"))
		}
		json.NewEncoder(w).Encode(HistoryPage{
			TotalRecords: 1,
			Records: []HistoryRecord{
				{ID: 1, SourceTitle: "Test.S01E01", EventType: "grabbed"},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	history, err := client.History(context.Background(), 5)
	if err != nil {
		t.Fatalf("History() error: %v", err)
	}
	if history.TotalRecords != 1 {
		t.Errorf("TotalRecords = %d, want 1", history.TotalRecords)
	}
}

func TestCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/command" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var cmd CommandRequest
		json.NewDecoder(r.Body).Decode(&cmd)
		if cmd.Name != "SeriesSearch" {
			t.Errorf("Name = %q, want SeriesSearch", cmd.Name)
		}
		if cmd.SeriesID != 5 {
			t.Errorf("SeriesID = %d, want 5", cmd.SeriesID)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(CommandResource{
			ID:     42,
			Name:   "SeriesSearch",
			Status: "queued",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	res, err := client.Command(context.Background(), CommandRequest{Name: "SeriesSearch", SeriesID: 5})
	if err != nil {
		t.Fatalf("Command() error: %v", err)
	}
	if res.ID != 42 {
		t.Errorf("ID = %d, want 42", res.ID)
	}
	if res.Name != "SeriesSearch" {
		t.Errorf("Name = %q, want SeriesSearch", res.Name)
	}
	if res.Status != "queued" {
		t.Errorf("Status = %q, want queued", res.Status)
	}
}

func TestDeleteSeries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/series/7" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("deleteFiles") != "true" {
			t.Errorf("deleteFiles = %s, want true", r.URL.Query().Get("deleteFiles"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	err := client.DeleteSeries(context.Background(), 7, true)
	if err != nil {
		t.Fatalf("DeleteSeries() error: %v", err)
	}
}

func TestDeleteBlocklistItem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/blocklist/99" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	err := client.DeleteBlocklistItem(context.Background(), 99)
	if err != nil {
		t.Fatalf("DeleteBlocklistItem() error: %v", err)
	}
}

func TestGrabRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/release" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req GrabReleaseRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.GUID != "abc-123" {
			t.Errorf("GUID = %q, want abc-123", req.GUID)
		}
		if req.IndexerID != 2 {
			t.Errorf("IndexerID = %d, want 2", req.IndexerID)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Release{
			GUID:      "abc-123",
			IndexerID: 2,
			Title:     "Test.Release.S01E01.720p",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	rel, err := client.GrabRelease(context.Background(), "abc-123", 2)
	if err != nil {
		t.Fatalf("GrabRelease() error: %v", err)
	}
	if rel.GUID != "abc-123" {
		t.Errorf("GUID = %q, want abc-123", rel.GUID)
	}
	if rel.Title != "Test.Release.S01E01.720p" {
		t.Errorf("Title = %q, want Test.Release.S01E01.720p", rel.Title)
	}
}
