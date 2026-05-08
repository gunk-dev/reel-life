package radarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/movie/lookup" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("term") != "inception" {
			t.Errorf("unexpected term: %s", r.URL.Query().Get("term"))
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("missing or wrong API key header")
		}
		json.NewEncoder(w).Encode([]Movie{
			{ID: 1, Title: "Inception", Year: 2010, TMDBID: 27205},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	results, err := client.Search(context.Background(), "inception")
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Inception" {
		t.Errorf("Title = %q, want %q", results[0].Title, "Inception")
	}
}

func TestAdd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/movie" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req AddMovieRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.TMDBID != 27205 {
			t.Errorf("TMDBID = %d, want 27205", req.TMDBID)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Movie{ID: 42, Title: req.Title, TMDBID: req.TMDBID})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	movie, err := client.Add(context.Background(), AddMovieRequest{
		Title:  "Inception",
		TMDBID: 27205,
	})
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if movie.ID != 42 {
		t.Errorf("ID = %d, want 42", movie.ID)
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
				{ID: 1, Title: "Inception.2010.1080p", Status: "downloading"},
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
				{ID: 1, SourceTitle: "Inception.2010.1080p", EventType: "grabbed"},
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

func TestGetMovie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/movie/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(Movie{ID: 42, Title: "Inception", Year: 2010})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	movie, err := client.GetMovie(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetMovie() error: %v", err)
	}
	if movie.Title != "Inception" {
		t.Errorf("Title = %q, want %q", movie.Title, "Inception")
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

func TestGetRootFolders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/rootfolder" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]RootFolder{
			{Path: "/movies", FreeSpace: 1000000, TotalSpace: 5000000},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	folders, err := client.GetRootFolders(context.Background())
	if err != nil {
		t.Fatalf("GetRootFolders() error: %v", err)
	}
	if len(folders) != 1 || folders[0].Path != "/movies" {
		t.Errorf("unexpected folders: %+v", folders)
	}
}

func TestGetDownloadClients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/downloadclient" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]DownloadClient{
			{Name: "qBittorrent", Enable: true, Protocol: "torrent", Priority: 1},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	clients, err := client.GetDownloadClients(context.Background())
	if err != nil {
		t.Fatalf("GetDownloadClients() error: %v", err)
	}
	if len(clients) != 1 || clients[0].Name != "qBittorrent" {
		t.Errorf("unexpected clients: %+v", clients)
	}
}

func TestGetBlocklist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/blocklist" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("pageSize") != "10" {
			t.Errorf("pageSize = %s, want 10", r.URL.Query().Get("pageSize"))
		}
		json.NewEncoder(w).Encode(BlocklistPage{
			TotalRecords: 1,
			Records: []BlocklistItem{
				{ID: 1, MovieID: 42, SourceTitle: "bad.release"},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	bl, err := client.GetBlocklist(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetBlocklist() error: %v", err)
	}
	if bl.TotalRecords != 1 {
		t.Errorf("TotalRecords = %d, want 1", bl.TotalRecords)
	}
}

func TestManualSearch(t *testing.T) {
	// Fixture matches the real Radarr /api/v3/release response shape:
	// "quality" is a nested object, not a string.
	const fixture = `[
		{
			"guid": "abc123",
			"title": "Inception.2010.1080p.BluRay.x264",
			"indexer": "NZBgeek",
			"indexerId": 3,
			"quality": {
				"quality": {
					"id": 7,
					"name": "Bluray-1080p",
					"source": "bluray",
					"resolution": 1080,
					"modifier": "none"
				},
				"revision": {
					"version": 1,
					"real": 0,
					"isRepack": false
				}
			},
			"size": 5000000000,
			"age": 2,
			"rejected": false
		}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/release" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("movieId") != "42" {
			t.Errorf("movieId = %s, want 42", r.URL.Query().Get("movieId"))
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
	if len(releases) != 1 || releases[0].GUID != "abc123" {
		t.Errorf("unexpected releases: %+v", releases)
	}
	got := releases[0].Quality.Quality.Name
	if got != "Bluray-1080p" {
		t.Errorf("Quality.Quality.Name = %q, want %q", got, "Bluray-1080p")
	}
	if releases[0].Quality.Quality.Resolution != 1080 {
		t.Errorf("Quality.Quality.Resolution = %d, want 1080", releases[0].Quality.Quality.Resolution)
	}
	if releases[0].Quality.Revision.Version != 1 {
		t.Errorf("Quality.Revision.Version = %d, want 1", releases[0].Quality.Revision.Version)
	}
}

func TestUpdateMovie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/movie/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var movie Movie
		json.NewDecoder(r.Body).Decode(&movie)
		if !movie.Monitored {
			t.Errorf("expected Monitored=true")
		}
		json.NewEncoder(w).Encode(movie)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	movie := &Movie{ID: 42, Title: "Inception", Monitored: true}
	updated, err := client.UpdateMovie(context.Background(), movie)
	if err != nil {
		t.Fatalf("UpdateMovie() error: %v", err)
	}
	if !updated.Monitored {
		t.Errorf("expected Monitored=true after update")
	}
}

func TestDeleteMovie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/movie/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("deleteFiles") != "true" {
			t.Errorf("deleteFiles = %s, want true", r.URL.Query().Get("deleteFiles"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	err := client.DeleteMovie(context.Background(), 42, true)
	if err != nil {
		t.Fatalf("DeleteMovie() error: %v", err)
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
		if cmd.Name != CommandMoviesSearch {
			t.Errorf("Name = %q, want %s", cmd.Name, CommandMoviesSearch)
		}
		if len(cmd.MovieIDs) != 1 || cmd.MovieIDs[0] != 42 {
			t.Errorf("MovieIDs = %v, want [42]", cmd.MovieIDs)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	err := client.Command(context.Background(), CommandRequest{Name: CommandMoviesSearch, MovieIDs: []int{42}})
	if err != nil {
		t.Fatalf("Command() error: %v", err)
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
		if req.GUID != "abc123" {
			t.Errorf("GUID = %q, want abc123", req.GUID)
		}
		if req.IndexerID != 5 {
			t.Errorf("IndexerID = %d, want 5", req.IndexerID)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	err := client.GrabRelease(context.Background(), "abc123", 5)
	if err != nil {
		t.Fatalf("GrabRelease() error: %v", err)
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
