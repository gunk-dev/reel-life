package investigation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/patflynn/reel-life/internal/sonarr"
)

type fakeClient struct {
	series                              *sonarr.Series
	episodes                            []sonarr.Episode
	queue                               *sonarr.QueuePage
	history                             *sonarr.HistoryPage
	releases                            []sonarr.Release
	queueErr, errorSearch, errorHistory error
	searches                            int
}

func (c *fakeClient) GetSeries(context.Context, int) (*sonarr.Series, error) { return c.series, nil }
func (c *fakeClient) GetEpisodes(context.Context, int, ...int) ([]sonarr.Episode, error) {
	return c.episodes, nil
}
func (c *fakeClient) Queue(context.Context) (*sonarr.QueuePage, error) { return c.queue, c.queueErr }
func (c *fakeClient) History(context.Context, int) (*sonarr.HistoryPage, error) {
	return c.history, c.errorHistory
}
func (c *fakeClient) ManualSearch(context.Context, int) ([]sonarr.Release, error) {
	c.searches++
	return c.releases, c.errorSearch
}
func fixture() *fakeClient {
	return &fakeClient{series: &sonarr.Series{ID: 1, Monitored: true}, episodes: []sonarr.Episode{{ID: 2, SeriesID: 1, Monitored: true, AirDateUTC: "2020-01-01T00:00:00Z"}}, queue: &sonarr.QueuePage{}, history: &sonarr.HistoryPage{}, releases: []sonarr.Release{{GUID: "candidate"}}}
}

func TestEpisodeInvestigation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		setup    func(*fakeClient)
		want     string
		searches int
	}{
		{"available", func(c *fakeClient) {}, "releases_available", 1},
		{"file", func(c *fakeClient) { c.episodes[0].HasFile = true }, "file_present", 0},
		{"future", func(c *fakeClient) { c.episodes[0].AirDateUTC = "2099-01-01T00:00:00Z" }, "not_aired", 0},
		{"unknown_air", func(c *fakeClient) { c.episodes[0].AirDateUTC = "" }, "air_date_unknown", 0},
		{"unmonitored", func(c *fakeClient) { c.series.Monitored = false }, "not_monitored", 1},
		{"queued", func(c *fakeClient) { c.queue.Records = []sonarr.QueueItem{{SeriesID: 1, EpisodeID: 2}} }, "in_queue", 0},
		{"import_warning", func(c *fakeClient) {
			c.queue.Records = []sonarr.QueueItem{{SeriesID: 1, EpisodeID: 2, TrackedDownloadStatus: "warning"}}
		}, "queue_needs_attention", 0},
		{"ambiguous_pack", func(c *fakeClient) { c.queue.Records = []sonarr.QueueItem{{SeriesID: 1}} }, "queue_identity_unknown", 0},
		{"queue_failure", func(c *fakeClient) { c.queueErr = errors.New("unavailable") }, "queue_unavailable", 0},
		{"empty_search", func(c *fakeClient) { c.releases = nil }, "no_releases_returned", 1},
		{"failed_search", func(c *fakeClient) { c.errorSearch = errors.New("unavailable") }, "search_unavailable", 1},
		{"rejected", func(c *fakeClient) { c.releases = []sonarr.Release{{Rejected: true, Rejections: []string{"quality"}}} }, "releases_rejected", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fixture()
			tc.setup(c)
			r, err := Episode(context.Background(), c, 1, 2, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, f := range r.Findings {
				if f.Code == tc.want {
					found = true
				}
			}
			if !found || c.searches != tc.searches {
				t.Fatalf("findings=%+v searches=%d", r.Findings, c.searches)
			}
		})
	}
}
func TestIdentityHistoryAndPartialEvidence(t *testing.T) {
	c := fixture()
	c.episodes[0].SeriesID = 9
	if _, err := Episode(context.Background(), c, 1, 2, time.Now()); err == nil || c.searches != 0 {
		t.Fatal("accepted wrong episode")
	}
	c = fixture()
	c.history.Records = []sonarr.HistoryRecord{{SeriesID: 1, EpisodeID: 8}, {SeriesID: 1, EpisodeID: 2, EventType: "downloadFailed"}}
	r, err := Episode(context.Background(), c, 1, 2, time.Now())
	if err != nil || len(r.RecentHistory) != 1 || len(r.Limitations) == 0 {
		t.Fatalf("history evidence: %+v %v", r, err)
	}
	c.errorHistory = errors.New("unavailable")
	r, err = Episode(context.Background(), c, 1, 2, time.Now())
	if err != nil || r.Sources["history"] != "failed" || r.Sources["releases"] != "succeeded" {
		t.Fatal("partial evidence hidden")
	}
}
