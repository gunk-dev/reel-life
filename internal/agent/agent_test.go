package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patflynn/reel-life/internal/notebook"
	"github.com/patflynn/reel-life/internal/overseerr"
	"github.com/patflynn/reel-life/internal/prowlarr"
	"github.com/patflynn/reel-life/internal/radarr"
	"github.com/patflynn/reel-life/internal/sonarr"
	"github.com/patflynn/reel-life/internal/weather"
)

// mockSonarr implements sonarr.Client for agent testing.
type mockSonarr struct {
	searchResult          []sonarr.Series
	healthResult          []sonarr.HealthCheck
	queueResult           *sonarr.QueuePage
	historyResult         *sonarr.HistoryPage
	seriesResult          *sonarr.Series
	episodesResult        []sonarr.Episode
	logsResult            []sonarr.LogRecord
	releasesResult        []sonarr.Release
	qualityProfilesResult []sonarr.QualityProfile
	blocklistResult       *sonarr.BlocklistPage
	rootFoldersResult     []sonarr.RootFolder
	downloadClientsResult []sonarr.DownloadClient
}

func (m *mockSonarr) Search(_ context.Context, _ string) ([]sonarr.Series, error) {
	return m.searchResult, nil
}
func (m *mockSonarr) Add(_ context.Context, req sonarr.AddSeriesRequest) (*sonarr.Series, error) {
	return &sonarr.Series{ID: 1, Title: req.Title, TVDBID: req.TVDBID}, nil
}
func (m *mockSonarr) Queue(_ context.Context) (*sonarr.QueuePage, error) {
	return m.queueResult, nil
}
func (m *mockSonarr) History(_ context.Context, _ int) (*sonarr.HistoryPage, error) {
	return m.historyResult, nil
}
func (m *mockSonarr) Health(_ context.Context) ([]sonarr.HealthCheck, error) {
	return m.healthResult, nil
}
func (m *mockSonarr) RemoveFailed(_ context.Context, _ int, _ bool) error {
	return nil
}
func (m *mockSonarr) GetSeries(_ context.Context, _ int) (*sonarr.Series, error) {
	return m.seriesResult, nil
}
func (m *mockSonarr) GetEpisodes(_ context.Context, _ int, _ ...int) ([]sonarr.Episode, error) {
	return m.episodesResult, nil
}
func (m *mockSonarr) GetLogs(_ context.Context, _ int, _ string) ([]sonarr.LogRecord, error) {
	return m.logsResult, nil
}
func (m *mockSonarr) ManualSearch(_ context.Context, _ int) ([]sonarr.Release, error) {
	return m.releasesResult, nil
}
func (m *mockSonarr) GetQualityProfiles(_ context.Context) ([]sonarr.QualityProfile, error) {
	return m.qualityProfilesResult, nil
}
func (m *mockSonarr) GetBlocklist(_ context.Context, _ int) (*sonarr.BlocklistPage, error) {
	return m.blocklistResult, nil
}
func (m *mockSonarr) GetRootFolders(_ context.Context) ([]sonarr.RootFolder, error) {
	return m.rootFoldersResult, nil
}
func (m *mockSonarr) GetDownloadClients(_ context.Context) ([]sonarr.DownloadClient, error) {
	return m.downloadClientsResult, nil
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

// mockRadarr implements radarr.Client for agent testing.
type mockRadarr struct {
	searchResult  []radarr.Movie
	healthResult  []radarr.HealthCheck
	queueResult   *radarr.QueuePage
	historyResult *radarr.HistoryPage
}

func (m *mockRadarr) Search(_ context.Context, _ string) ([]radarr.Movie, error) {
	return m.searchResult, nil
}
func (m *mockRadarr) Add(_ context.Context, req radarr.AddMovieRequest) (*radarr.Movie, error) {
	return &radarr.Movie{ID: 1, Title: req.Title, TMDBID: req.TMDBID}, nil
}
func (m *mockRadarr) Queue(_ context.Context) (*radarr.QueuePage, error) {
	return m.queueResult, nil
}
func (m *mockRadarr) History(_ context.Context, _ int) (*radarr.HistoryPage, error) {
	return m.historyResult, nil
}
func (m *mockRadarr) Health(_ context.Context) ([]radarr.HealthCheck, error) {
	return m.healthResult, nil
}
func (m *mockRadarr) RemoveFailed(_ context.Context, _ int, _ bool) error {
	return nil
}
func (m *mockRadarr) GetMovie(_ context.Context, id int) (*radarr.Movie, error) {
	return &radarr.Movie{ID: id, Title: "Test Movie"}, nil
}
func (m *mockRadarr) GetQualityProfiles(_ context.Context) ([]radarr.QualityProfile, error) {
	return []radarr.QualityProfile{{ID: 1, Name: "HD"}}, nil
}
func (m *mockRadarr) GetRootFolders(_ context.Context) ([]radarr.RootFolder, error) {
	return []radarr.RootFolder{{Path: "/movies"}}, nil
}
func (m *mockRadarr) GetDownloadClients(_ context.Context) ([]radarr.DownloadClient, error) {
	return []radarr.DownloadClient{{Name: "qBit", Enable: true}}, nil
}
func (m *mockRadarr) GetBlocklist(_ context.Context, _ int) (*radarr.BlocklistPage, error) {
	return &radarr.BlocklistPage{}, nil
}
func (m *mockRadarr) ManualSearch(_ context.Context, _ int) ([]radarr.Release, error) {
	return []radarr.Release{}, nil
}
func (m *mockRadarr) UpdateMovie(_ context.Context, movie *radarr.Movie) (*radarr.Movie, error) {
	return movie, nil
}
func (m *mockRadarr) DeleteMovie(_ context.Context, _ int, _ bool) error {
	return nil
}
func (m *mockRadarr) Command(_ context.Context, _ radarr.CommandRequest) error {
	return nil
}
func (m *mockRadarr) GrabRelease(_ context.Context, _ string, _ int) error {
	return nil
}
func (m *mockRadarr) DeleteBlocklistItem(_ context.Context, _ int) error {
	return nil
}
func (m *mockRadarr) GetLanguageProfiles(_ context.Context) ([]radarr.LanguageProfile, error) {
	return nil, nil
}
func (m *mockRadarr) GetCustomFormats(_ context.Context) ([]radarr.CustomFormat, error) {
	return nil, nil
}

// mockProwlarr implements prowlarr.Client for agent testing.
type mockProwlarr struct {
	indexers       []prowlarr.Indexer
	stats          *prowlarr.IndexerStats
	health         []prowlarr.HealthCheck
	searchResult   []prowlarr.SearchResult
	testAllResult  []prowlarr.IndexerTestResult
	updatedIndexer *prowlarr.Indexer
	deletedID      int
}

func (m *mockProwlarr) ListIndexers(_ context.Context) ([]prowlarr.Indexer, error) {
	return m.indexers, nil
}
func (m *mockProwlarr) TestIndexer(_ context.Context, _ int) error {
	return nil
}
func (m *mockProwlarr) TestAllIndexers(_ context.Context) ([]prowlarr.IndexerTestResult, error) {
	return m.testAllResult, nil
}
func (m *mockProwlarr) UpdateIndexer(_ context.Context, indexer *prowlarr.Indexer) (*prowlarr.Indexer, error) {
	m.updatedIndexer = indexer
	return indexer, nil
}
func (m *mockProwlarr) DeleteIndexer(_ context.Context, id int) error {
	m.deletedID = id
	return nil
}
func (m *mockProwlarr) GetIndexerStats(_ context.Context) (*prowlarr.IndexerStats, error) {
	return m.stats, nil
}
func (m *mockProwlarr) CheckHealth(_ context.Context) ([]prowlarr.HealthCheck, error) {
	return m.health, nil
}
func (m *mockProwlarr) Search(_ context.Context, _ string) ([]prowlarr.SearchResult, error) {
	return m.searchResult, nil
}

// mockOverseerr implements overseerr.Client for agent testing.
type mockOverseerr struct {
	requests     *overseerr.RequestPage
	requestCount *overseerr.RequestCount
	searchResult *overseerr.SearchResults
}

func (m *mockOverseerr) ListRequests(_ context.Context, _ string, _, _ int) (*overseerr.RequestPage, error) {
	return m.requests, nil
}
func (m *mockOverseerr) ApproveRequest(_ context.Context, _ int) error {
	return nil
}
func (m *mockOverseerr) DeclineRequest(_ context.Context, _ int) error {
	return nil
}
func (m *mockOverseerr) GetRequestCount(_ context.Context) (*overseerr.RequestCount, error) {
	return m.requestCount, nil
}
func (m *mockOverseerr) GetRequest(_ context.Context, _ int) (*overseerr.Request, error) {
	return nil, nil
}
func (m *mockOverseerr) DeleteRequest(_ context.Context, _ int) error {
	return nil
}
func (m *mockOverseerr) RetryRequest(_ context.Context, _ int) (*overseerr.Request, error) {
	return nil, nil
}
func (m *mockOverseerr) SearchMedia(_ context.Context, _ string, _ int) (*overseerr.SearchResults, error) {
	return m.searchResult, nil
}

func newTestAgent() *Agent {
	return &Agent{sonarr: &mockSonarr{}, radarr: &mockRadarr{}, prowlarr: &mockProwlarr{}, overseerr: &mockOverseerr{}, logger: slog.Default()}
}

func newTestAgentWithNotebook(t *testing.T) *Agent {
	t.Helper()
	nb := notebook.NewFileNotebook(filepath.Join(t.TempDir(), "notebook.json"))
	return &Agent{sonarr: &mockSonarr{}, radarr: &mockRadarr{}, prowlarr: &mockProwlarr{}, overseerr: &mockOverseerr{}, notebook: nb, logger: slog.Default()}
}

func TestDispatchSearchSeries(t *testing.T) {
	mock := &mockSonarr{
		searchResult: []sonarr.Series{
			{ID: 1, Title: "Breaking Bad", Year: 2008, TVDBID: 81189},
		},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(searchSeriesInput{Term: "breaking bad"})
	result, isErr := a.dispatchTool(context.Background(), "search_series", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var series []sonarr.Series
	if err := json.Unmarshal([]byte(result), &series); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(series) != 1 || series[0].Title != "Breaking Bad" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDispatchCheckHealth(t *testing.T) {
	mock := &mockSonarr{
		healthResult: []sonarr.HealthCheck{
			{Source: "IndexerCheck", Type: "warning", Message: "test warning"},
		},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(struct{}{})
	result, isErr := a.dispatchTool(context.Background(), "check_health", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var checks []sonarr.HealthCheck
	if err := json.Unmarshal([]byte(result), &checks); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(checks))
	}
}

func TestDispatchRemoveFailed(t *testing.T) {
	a := newTestAgent()

	input, _ := json.Marshal(removeFailedInput{ID: 42, Blocklist: true})
	result, isErr := a.dispatchTool(context.Background(), "remove_failed", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var status map[string]string
	json.Unmarshal([]byte(result), &status)
	if status["status"] != "removed" {
		t.Errorf("expected status=removed, got %s", result)
	}
}

func TestDispatchUnknownTool(t *testing.T) {
	a := &Agent{sonarr: &mockSonarr{}, radarr: &mockRadarr{}, prowlarr: &mockProwlarr{}, overseerr: &mockOverseerr{}}

	result, isErr := a.dispatchTool(context.Background(), "nonexistent", json.RawMessage("{}"))
	if !isErr {
		t.Fatal("expected error for unknown tool")
	}
	if result == "" {
		t.Fatal("expected error message")
	}
}

func TestDispatchGetQueue(t *testing.T) {
	mock := &mockSonarr{
		queueResult: &sonarr.QueuePage{
			TotalRecords: 2,
			Records: []sonarr.QueueItem{
				{ID: 1, Title: "S01E01", Status: "downloading"},
				{ID: 2, Title: "S01E02", Status: "queued"},
			},
		},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(struct{}{})
	result, isErr := a.dispatchTool(context.Background(), "get_queue", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var queue sonarr.QueuePage
	if err := json.Unmarshal([]byte(result), &queue); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if queue.TotalRecords != 2 {
		t.Errorf("expected 2 records, got %d", queue.TotalRecords)
	}
}

func TestDispatchAddSeries(t *testing.T) {
	a := newTestAgent()

	input, _ := json.Marshal(addSeriesInput{
		Title:            "Breaking Bad",
		TVDBID:           81189,
		QualityProfileID: 1,
		RootFolderPath:   "/tv",
	})
	result, isErr := a.dispatchTool(context.Background(), "add_series", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var series sonarr.Series
	if err := json.Unmarshal([]byte(result), &series); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if series.Title != "Breaking Bad" {
		t.Errorf("Title = %q, want %q", series.Title, "Breaking Bad")
	}
}

func TestDispatchGetHistory(t *testing.T) {
	mock := &mockSonarr{
		historyResult: &sonarr.HistoryPage{
			TotalRecords: 1,
			Records: []sonarr.HistoryRecord{
				{ID: 1, SourceTitle: "Test.S01E01", EventType: "grabbed"},
			},
		},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(getHistoryInput{PageSize: 10})
	result, isErr := a.dispatchTool(context.Background(), "get_history", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var history sonarr.HistoryPage
	if err := json.Unmarshal([]byte(result), &history); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if history.TotalRecords != 1 {
		t.Errorf("expected 1 record, got %d", history.TotalRecords)
	}
}

func TestDispatchSearchMovies(t *testing.T) {
	mock := &mockRadarr{
		searchResult: []radarr.Movie{
			{ID: 1, Title: "Inception", Year: 2010, TMDBID: 27205},
		},
	}
	a := newTestAgent()
	a.radarr = mock

	input, _ := json.Marshal(searchMoviesInput{Term: "inception"})
	result, isErr := a.dispatchTool(context.Background(), "search_movies", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var movies []radarr.Movie
	if err := json.Unmarshal([]byte(result), &movies); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(movies) != 1 || movies[0].Title != "Inception" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDispatchAddMovie(t *testing.T) {
	a := newTestAgent()

	input, _ := json.Marshal(addMovieInput{
		Title:            "Inception",
		TMDBID:           27205,
		QualityProfileID: 1,
		RootFolderPath:   "/movies",
	})
	result, isErr := a.dispatchTool(context.Background(), "add_movie", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var movie radarr.Movie
	if err := json.Unmarshal([]byte(result), &movie); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if movie.Title != "Inception" {
		t.Errorf("Title = %q, want %q", movie.Title, "Inception")
	}
}

func TestDispatchGetMovieQueue(t *testing.T) {
	mock := &mockRadarr{
		queueResult: &radarr.QueuePage{
			TotalRecords: 1,
			Records: []radarr.QueueItem{
				{ID: 1, Title: "Inception.2010.1080p", Status: "downloading"},
			},
		},
	}
	a := newTestAgent()
	a.radarr = mock

	input, _ := json.Marshal(struct{}{})
	result, isErr := a.dispatchTool(context.Background(), "get_movie_queue", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var queue radarr.QueuePage
	if err := json.Unmarshal([]byte(result), &queue); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if queue.TotalRecords != 1 {
		t.Errorf("expected 1 record, got %d", queue.TotalRecords)
	}
}

func TestDispatchCheckMovieHealth(t *testing.T) {
	mock := &mockRadarr{
		healthResult: []radarr.HealthCheck{
			{Source: "IndexerCheck", Type: "warning", Message: "test warning"},
		},
	}
	a := newTestAgent()
	a.radarr = mock

	input, _ := json.Marshal(struct{}{})
	result, isErr := a.dispatchTool(context.Background(), "check_movie_health", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var checks []radarr.HealthCheck
	if err := json.Unmarshal([]byte(result), &checks); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(checks))
	}
}

func TestDispatchRemoveFailedMovie(t *testing.T) {
	a := newTestAgent()

	input, _ := json.Marshal(removeFailedMovieInput{ID: 42, Blocklist: true})
	result, isErr := a.dispatchTool(context.Background(), "remove_failed_movie", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var status map[string]string
	json.Unmarshal([]byte(result), &status)
	if status["status"] != "removed" {
		t.Errorf("expected status=removed, got %s", result)
	}
}

func TestDispatchGetMovieHistory(t *testing.T) {
	mock := &mockRadarr{
		historyResult: &radarr.HistoryPage{
			TotalRecords: 1,
			Records: []radarr.HistoryRecord{
				{ID: 1, SourceTitle: "Inception.2010.1080p", EventType: "grabbed"},
			},
		},
	}
	a := newTestAgent()
	a.radarr = mock

	input, _ := json.Marshal(getMovieHistoryInput{PageSize: 10})
	result, isErr := a.dispatchTool(context.Background(), "get_movie_history", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var history radarr.HistoryPage
	if err := json.Unmarshal([]byte(result), &history); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if history.TotalRecords != 1 {
		t.Errorf("expected 1 record, got %d", history.TotalRecords)
	}
}

func TestDispatchListIndexers(t *testing.T) {
	mock := &mockProwlarr{
		indexers: []prowlarr.Indexer{
			{ID: 1, Name: "NZBgeek", Enable: true, Protocol: "usenet", Priority: 25},
		},
	}
	a := newTestAgent()
	a.prowlarr = mock

	input, _ := json.Marshal(struct{}{})
	result, isErr := a.dispatchTool(context.Background(), "list_indexers", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var indexers []prowlarr.Indexer
	if err := json.Unmarshal([]byte(result), &indexers); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(indexers) != 1 || indexers[0].Name != "NZBgeek" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDispatchTestIndexer(t *testing.T) {
	a := newTestAgent()

	input, _ := json.Marshal(testIndexerInput{ID: 5})
	result, isErr := a.dispatchTool(context.Background(), "test_indexer", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var status map[string]string
	json.Unmarshal([]byte(result), &status)
	if status["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", result)
	}
}

func TestDispatchGetIndexerStats(t *testing.T) {
	mock := &mockProwlarr{
		stats: &prowlarr.IndexerStats{
			Indexers: []prowlarr.IndexerStatEntry{
				{IndexerID: 1, IndexerName: "NZBgeek", NumberOfQueries: 100},
			},
		},
	}
	a := newTestAgent()
	a.prowlarr = mock

	input, _ := json.Marshal(struct{}{})
	result, isErr := a.dispatchTool(context.Background(), "get_indexer_stats", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var stats prowlarr.IndexerStats
	if err := json.Unmarshal([]byte(result), &stats); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(stats.Indexers) != 1 {
		t.Errorf("expected 1 indexer stat, got %d", len(stats.Indexers))
	}
}

func TestDispatchCheckIndexerHealth(t *testing.T) {
	mock := &mockProwlarr{
		health: []prowlarr.HealthCheck{
			{Source: "IndexerStatusCheck", Type: "warning", Message: "test"},
		},
	}
	a := newTestAgent()
	a.prowlarr = mock

	input, _ := json.Marshal(struct{}{})
	result, isErr := a.dispatchTool(context.Background(), "check_indexer_health", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var checks []prowlarr.HealthCheck
	if err := json.Unmarshal([]byte(result), &checks); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(checks))
	}
}

func TestDispatchSearchIndexers(t *testing.T) {
	mock := &mockProwlarr{
		searchResult: []prowlarr.SearchResult{
			{GUID: "abc123", IndexerID: 1, Title: "Breaking.Bad.S01E01", Size: 1400000000},
		},
	}
	a := newTestAgent()
	a.prowlarr = mock

	input, _ := json.Marshal(searchIndexersInput{Query: "breaking bad"})
	result, isErr := a.dispatchTool(context.Background(), "search_indexers", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var results []prowlarr.SearchResult
	if err := json.Unmarshal([]byte(result), &results); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Breaking.Bad.S01E01" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDispatchOverseerrNotConfigured(t *testing.T) {
	a := &Agent{sonarr: &mockSonarr{}, radarr: &mockRadarr{}, prowlarr: &mockProwlarr{}}

	for _, tool := range []string{"list_requests", "approve_request", "decline_request", "get_request_count", "search_media"} {
		result, isErr := a.dispatchTool(context.Background(), tool, json.RawMessage("{}"))
		if !isErr {
			t.Errorf("%s: expected error when overseerr not configured", tool)
		}
		if result == "" {
			t.Errorf("%s: expected error message", tool)
		}
	}
}

func TestDispatchListRequests(t *testing.T) {
	mock := &mockOverseerr{
		requests: &overseerr.RequestPage{
			Results: []overseerr.Request{
				{ID: 1, Status: 1, Type: "movie"},
			},
		},
	}
	a := newTestAgent()
	a.overseerr = mock

	input, _ := json.Marshal(listRequestsInput{Filter: "all", Take: 10})
	result, isErr := a.dispatchTool(context.Background(), "list_requests", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var page overseerr.RequestPage
	if err := json.Unmarshal([]byte(result), &page); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(page.Results) != 1 {
		t.Errorf("expected 1 request, got %d", len(page.Results))
	}
}

func TestDispatchApproveRequest(t *testing.T) {
	a := newTestAgent()

	input, _ := json.Marshal(approveRequestInput{ID: 1})
	result, isErr := a.dispatchTool(context.Background(), "approve_request", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var status map[string]string
	json.Unmarshal([]byte(result), &status)
	if status["status"] != "approved" {
		t.Errorf("expected status=approved, got %s", result)
	}
}

func TestDispatchDeclineRequest(t *testing.T) {
	a := newTestAgent()

	input, _ := json.Marshal(declineRequestInput{ID: 1})
	result, isErr := a.dispatchTool(context.Background(), "decline_request", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var status map[string]string
	json.Unmarshal([]byte(result), &status)
	if status["status"] != "declined" {
		t.Errorf("expected status=declined, got %s", result)
	}
}

func TestDispatchGetRequestCount(t *testing.T) {
	mock := &mockOverseerr{
		requestCount: &overseerr.RequestCount{
			Pending: 5, Approved: 10, Declined: 2, Total: 17,
		},
	}
	a := newTestAgent()
	a.overseerr = mock

	input, _ := json.Marshal(struct{}{})
	result, isErr := a.dispatchTool(context.Background(), "get_request_count", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var count overseerr.RequestCount
	if err := json.Unmarshal([]byte(result), &count); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if count.Total != 17 {
		t.Errorf("expected total=17, got %d", count.Total)
	}
}

func TestDispatchSearchMedia(t *testing.T) {
	mock := &mockOverseerr{
		searchResult: &overseerr.SearchResults{
			TotalResults: 1,
			Results: []overseerr.SearchResult{
				{ID: 123, Title: "Inception", MediaType: "movie"},
			},
		},
	}
	a := newTestAgent()
	a.overseerr = mock

	input, _ := json.Marshal(searchMediaInput{Query: "inception"})
	result, isErr := a.dispatchTool(context.Background(), "search_media", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var results overseerr.SearchResults
	if err := json.Unmarshal([]byte(result), &results); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if results.TotalResults != 1 || results.Results[0].Title != "Inception" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDispatchNotebookNotConfigured(t *testing.T) {
	a := &Agent{sonarr: &mockSonarr{}, radarr: &mockRadarr{}, prowlarr: &mockProwlarr{}, overseerr: &mockOverseerr{}}

	for _, tool := range []string{"notebook_write", "notebook_read", "notebook_list", "notebook_delete"} {
		result, isErr := a.dispatchTool(context.Background(), tool, json.RawMessage("{}"))
		if !isErr {
			t.Errorf("%s: expected error when notebook not configured", tool)
		}
		if result == "" {
			t.Errorf("%s: expected error message", tool)
		}
	}
}

func TestDispatchNotebookWrite(t *testing.T) {
	a := newTestAgentWithNotebook(t)

	input, _ := json.Marshal(notebookWriteInput{
		Type:    "reference",
		Title:   "Test Note",
		Content: "Some content",
	})
	result, isErr := a.dispatchTool(context.Background(), "notebook_write", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var status map[string]string
	json.Unmarshal([]byte(result), &status)
	if status["status"] != "saved" {
		t.Errorf("expected status=saved, got %s", result)
	}
}

func TestDispatchNotebookWriteDuplicateTitle(t *testing.T) {
	a := newTestAgentWithNotebook(t)
	ctx := context.Background()

	// Write initial note.
	input, _ := json.Marshal(notebookWriteInput{
		Type:    "reference",
		Title:   "User Preferences",
		Content: "likes sci-fi",
	})
	a.dispatchTool(ctx, "notebook_write", input)

	// Try to write another note with the same title (no ID = new note).
	input, _ = json.Marshal(notebookWriteInput{
		Type:    "reference",
		Title:   "User Preferences",
		Content: "likes horror",
	})
	result, isErr := a.dispatchTool(ctx, "notebook_write", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	// Should return a warning with existing_id.
	var resp map[string]string
	json.Unmarshal([]byte(result), &resp)
	if resp["warning"] == "" || resp["existing_id"] == "" {
		t.Errorf("expected duplicate warning, got: %s", result)
	}
}

func TestDispatchNotebookRead(t *testing.T) {
	a := newTestAgentWithNotebook(t)
	ctx := context.Background()

	// Write a note first.
	a.notebook.Write(ctx, notebook.Note{
		ID:      "read-me",
		Type:    notebook.Reference,
		Title:   "Readable",
		Content: "hello",
	})

	input, _ := json.Marshal(notebookReadInput{ID: "read-me"})
	result, isErr := a.dispatchTool(ctx, "notebook_read", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var note notebook.Note
	if err := json.Unmarshal([]byte(result), &note); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if note.Title != "Readable" || note.Content != "hello" {
		t.Errorf("unexpected note: %+v", note)
	}
}

func TestDispatchNotebookList(t *testing.T) {
	a := newTestAgentWithNotebook(t)
	ctx := context.Background()

	a.notebook.Write(ctx, notebook.Note{ID: "1", Type: notebook.Pinned, Title: "Pinned", Content: "p"})
	a.notebook.Write(ctx, notebook.Note{ID: "2", Type: notebook.Reference, Title: "Ref", Content: "r"})

	// List all.
	input, _ := json.Marshal(notebookListInput{})
	result, isErr := a.dispatchTool(ctx, "notebook_list", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var summaries []notebook.NoteSummary
	json.Unmarshal([]byte(result), &summaries)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	// List filtered.
	input, _ = json.Marshal(notebookListInput{Type: "pinned"})
	result, isErr = a.dispatchTool(ctx, "notebook_list", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	json.Unmarshal([]byte(result), &summaries)
	if len(summaries) != 1 || summaries[0].Title != "Pinned" {
		t.Errorf("unexpected filtered result: %s", result)
	}
}

func TestDispatchNotebookDelete(t *testing.T) {
	a := newTestAgentWithNotebook(t)
	ctx := context.Background()

	a.notebook.Write(ctx, notebook.Note{ID: "del-me", Type: notebook.Reference, Title: "Delete", Content: "x"})

	input, _ := json.Marshal(notebookDeleteInput{ID: "del-me"})
	result, isErr := a.dispatchTool(ctx, "notebook_delete", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var status map[string]string
	json.Unmarshal([]byte(result), &status)
	if status["status"] != "deleted" {
		t.Errorf("expected status=deleted, got %s", result)
	}
}

func TestBuildSystemPromptNoWeather(t *testing.T) {
	a := newTestAgent()
	prompt := a.buildSystemPrompt(context.Background())
	if !strings.Contains(prompt, "Today's date is") {
		t.Error("expected date in prompt")
	}
	if strings.Contains(prompt, "Current location") {
		t.Error("did not expect location when weather client is nil")
	}
}

func TestBuildSystemPromptToolDiscipline(t *testing.T) {
	a := newTestAgent()
	prompt := a.buildSystemPrompt(context.Background())
	wantSubstrings := []string{
		"Tool discipline",
		"NEVER fabricate tool results",
		"Do NOT report past-tense success",
		"call the tool for each one",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(prompt, want) {
			t.Errorf("expected prompt to contain %q, but it did not", want)
		}
	}
}

func TestBuildSystemPromptWithWeather(t *testing.T) {
	// Create a mock weather server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"current": map[string]any{
				"temperature_2m": -5.0,
				"weather_code":   71,
			},
		})
	}))
	defer srv.Close()

	wc := weather.NewClient(45.5, -73.5, "Montreal, QC")
	wc.SetHTTPClient(&http.Client{
		Transport: &testRewriteTransport{base: srv.URL},
	})

	a := newTestAgent()
	a.weather = wc
	prompt := a.buildSystemPrompt(context.Background())
	if !strings.Contains(prompt, "Current location: Montreal, QC.") {
		t.Errorf("expected location in prompt, got: %s", prompt[:200])
	}
	if !strings.Contains(prompt, "Weather: -5°C, snow.") {
		t.Errorf("expected weather in prompt, got: %s", prompt[:200])
	}
}

func TestBuildSystemPromptWeatherUnavailable(t *testing.T) {
	// Server that always fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	wc := weather.NewClient(45.5, -73.5, "Montreal, QC")
	wc.SetHTTPClient(&http.Client{
		Transport: &testRewriteTransport{base: srv.URL},
	})

	a := newTestAgent()
	a.weather = wc
	prompt := a.buildSystemPrompt(context.Background())
	if !strings.Contains(prompt, "Current location: Montreal, QC.") {
		t.Errorf("expected location-only in prompt, got: %s", prompt[:200])
	}
	if strings.Contains(prompt, "Weather:") {
		t.Error("did not expect weather when fetch fails")
	}
}

// testRewriteTransport redirects all requests to a test server.
type testRewriteTransport struct {
	base string
}

func (t *testRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = t.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req)
}

func TestDispatchNotebookWriteInvalidType(t *testing.T) {
	a := newTestAgentWithNotebook(t)

	input, _ := json.Marshal(notebookWriteInput{
		Type:    "invalid",
		Title:   "Bad",
		Content: "x",
	})
	_, isErr := a.dispatchTool(context.Background(), "notebook_write", input)
	if !isErr {
		t.Fatal("expected error for invalid note type")
	}
}

func TestDispatchGetSeriesDetail(t *testing.T) {
	mock := &mockSonarr{
		seriesResult: &sonarr.Series{ID: 1, Title: "Breaking Bad", EpisodeCount: 62, EpisodeFileCount: 62, SizeOnDisk: 100000},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(getSeriesDetailInput{SeriesID: 1})
	result, isErr := a.dispatchTool(context.Background(), "get_series_detail", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var series sonarr.Series
	if err := json.Unmarshal([]byte(result), &series); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if series.Title != "Breaking Bad" || series.EpisodeCount != 62 {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDispatchGetEpisodes(t *testing.T) {
	mock := &mockSonarr{
		episodesResult: []sonarr.Episode{
			{ID: 1, SeasonNumber: 1, EpisodeNumber: 1, Title: "Pilot", HasFile: true},
		},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(getEpisodesInput{SeriesID: 1})
	result, isErr := a.dispatchTool(context.Background(), "get_episodes", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var episodes []sonarr.Episode
	if err := json.Unmarshal([]byte(result), &episodes); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(episodes) != 1 || episodes[0].Title != "Pilot" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDispatchGetLogs(t *testing.T) {
	mock := &mockSonarr{
		logsResult: []sonarr.LogRecord{
			{Time: "2026-01-01", Level: "error", Logger: "SonarrLogger", Message: "test error"},
		},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(getLogsInput{PageSize: 10, Level: "error"})
	result, isErr := a.dispatchTool(context.Background(), "get_logs", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var logs []sonarr.LogRecord
	if err := json.Unmarshal([]byte(result), &logs); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(logs) != 1 || logs[0].Level != "error" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDispatchManualSearch(t *testing.T) {
	mock := &mockSonarr{
		releasesResult: []sonarr.Release{
			{Title: "Breaking.Bad.S01E01.1080p", Indexer: "NZBgeek", Size: 1400000000, Rejected: false},
		},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(manualSearchInput{EpisodeID: 1})
	result, isErr := a.dispatchTool(context.Background(), "manual_search", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var releases []sonarr.Release
	if err := json.Unmarshal([]byte(result), &releases); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(releases) != 1 || releases[0].Indexer != "NZBgeek" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDispatchGetQualityProfiles(t *testing.T) {
	mock := &mockSonarr{
		qualityProfilesResult: []sonarr.QualityProfile{
			{ID: 1, Name: "HD-1080p", Cutoff: 7},
		},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(struct{}{})
	result, isErr := a.dispatchTool(context.Background(), "get_quality_profiles", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var profiles []sonarr.QualityProfile
	if err := json.Unmarshal([]byte(result), &profiles); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "HD-1080p" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDispatchGetBlocklist(t *testing.T) {
	mock := &mockSonarr{
		blocklistResult: &sonarr.BlocklistPage{
			TotalRecords: 1,
			Records: []sonarr.BlocklistItem{
				{ID: 1, SeriesID: 1, SourceTitle: "Bad.Release", Date: "2026-01-01"},
			},
		},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(getBlocklistInput{PageSize: 10})
	result, isErr := a.dispatchTool(context.Background(), "get_blocklist", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var page sonarr.BlocklistPage
	if err := json.Unmarshal([]byte(result), &page); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if page.TotalRecords != 1 {
		t.Errorf("expected 1 record, got %d", page.TotalRecords)
	}
}

func TestDispatchGetRootFolders(t *testing.T) {
	mock := &mockSonarr{
		rootFoldersResult: []sonarr.RootFolder{
			{Path: "/tv", FreeSpace: 500000000000, TotalSpace: 1000000000000},
		},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(struct{}{})
	result, isErr := a.dispatchTool(context.Background(), "get_root_folders", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var folders []sonarr.RootFolder
	if err := json.Unmarshal([]byte(result), &folders); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(folders) != 1 || folders[0].Path != "/tv" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestDispatchGetDownloadClients(t *testing.T) {
	mock := &mockSonarr{
		downloadClientsResult: []sonarr.DownloadClient{
			{Name: "SABnzbd", Enable: true, Protocol: "usenet", Priority: 1},
		},
	}
	a := newTestAgent()
	a.sonarr = mock

	input, _ := json.Marshal(struct{}{})
	result, isErr := a.dispatchTool(context.Background(), "get_download_clients", input)
	if isErr {
		t.Fatalf("unexpected error: %s", result)
	}

	var clients []sonarr.DownloadClient
	if err := json.Unmarshal([]byte(result), &clients); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(clients) != 1 || clients[0].Name != "SABnzbd" {
		t.Errorf("unexpected result: %s", result)
	}
}
