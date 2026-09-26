// Package investigation combines read-only Sonarr evidence into bounded diagnoses.
package investigation

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/patflynn/reel-life/internal/sonarr"
)

type Client interface {
	GetSeries(context.Context, int) (*sonarr.Series, error)
	GetEpisodes(context.Context, int, ...int) ([]sonarr.Episode, error)
	Queue(context.Context) (*sonarr.QueuePage, error)
	History(context.Context, int) (*sonarr.HistoryPage, error)
	ManualSearch(context.Context, int) ([]sonarr.Release, error)
}

type Finding struct {
	Code        string `json:"code"`
	Explanation string `json:"explanation"`
	NextAction  string `json:"next_action"`
}
type Result struct {
	SeriesID         int                    `json:"series_id"`
	Episode          sonarr.Episode         `json:"episode"`
	CheckedAt        time.Time              `json:"checked_at"`
	Sources          map[string]string      `json:"sources"`
	Findings         []Finding              `json:"findings"`
	Queue            []sonarr.QueueItem     `json:"queue"`
	RecentHistory    []sonarr.HistoryRecord `json:"recent_history"`
	Releases         []sonarr.Release       `json:"release_sample"`
	ReleasesFound    int                    `json:"releases_found"`
	ReleasesAccepted int                    `json:"releases_accepted"`
	Limitations      []string               `json:"limitations"`
}

// Episode validates identity before correlating evidence. History is a bounded
// recent global sample, never proof that an episode has no older history.
func Episode(ctx context.Context, c Client, seriesID, episodeID int, now time.Time) (Result, error) {
	r := Result{SeriesID: seriesID, CheckedAt: now.UTC(), Sources: map[string]string{}, Limitations: []string{
		"Evidence is collected sequentially, not as an atomic snapshot.",
		"History covers at most the 100 most recent global records; missing matches do not mean no past activity.",
		"A Sonarr file flag or search result does not prove playback or a successful download.",
	}}
	if seriesID <= 0 || episodeID <= 0 {
		return r, fmt.Errorf("positive series_id and episode_id are required")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	series, err := c.GetSeries(ctx, seriesID)
	if err != nil {
		return r, fmt.Errorf("read series: %w", err)
	}
	if series == nil || series.ID != seriesID {
		return r, fmt.Errorf("series identity could not be verified")
	}
	episodes, err := c.GetEpisodes(ctx, seriesID)
	if err != nil {
		return r, fmt.Errorf("read episodes: %w", err)
	}
	found := 0
	for _, ep := range episodes {
		if ep.ID == episodeID && ep.SeriesID == seriesID {
			r.Episode = ep
			found++
		}
	}
	if found != 1 {
		return r, fmt.Errorf("episode identity could not be uniquely verified in this series")
	}
	r.Sources["episode"] = "succeeded"
	add := func(code, why, next string) { r.Findings = append(r.Findings, Finding{code, why, next}) }
	if r.Episode.HasFile {
		add("file_present", "Sonarr reports an episode file.", "If it is missing in your player, investigate library refresh or playback separately.")
	}
	if !series.Monitored || !r.Episode.Monitored {
		add("not_monitored", "The series or episode is not monitored.", "Enable the appropriate monitoring only if requested; do not change it as part of diagnosis.")
	}
	air, airErr := time.Parse(time.RFC3339, r.Episode.AirDateUTC)
	if airErr != nil {
		add("air_date_unknown", "A valid air date is unavailable.", "Check episode metadata before deciding whether a release should exist.")
	} else if air.After(now) {
		add("not_aired", "The episode's air date is in the future.", "Wait until after airing; availability can lag the air date.")
	}
	q, err := c.Queue(ctx)
	r.Sources["queue"] = "failed"
	ambiguous := false
	if err == nil && q != nil {
		r.Sources["queue"] = "succeeded"
		for _, item := range q.Records {
			if item.SeriesID == seriesID && item.EpisodeID == episodeID {
				r.Queue = append(r.Queue, item)
			}
			if item.SeriesID == seriesID && item.EpisodeID == 0 {
				ambiguous = true
			}
		}
		if ambiguous {
			add("queue_identity_unknown", "Some queue entries for this series have no episode identity.", "Inspect those downloads before starting another search; they may cover this episode.")
		}
		if len(r.Queue) > 0 {
			add("in_queue", "The episode has matching queue entries.", "Check their download/import status; do not start a duplicate search.")
			for _, item := range r.Queue {
				status := strings.ToLower(item.Status + " " + item.TrackedDownloadStatus + " " + item.TrackedDownloadState)
				if strings.Contains(status, "failed") || strings.Contains(status, "warning") || strings.Contains(status, "pending") {
					add("queue_needs_attention", "A matching queue entry reports a failed, warning, or pending state.", "Explain the returned queue error/status messages and inspect the download client or import details before removing or retrying anything.")
					break
				}
			}
		}
	} else {
		add("queue_unavailable", "The complete queue could not be read.", "Restore queue access before concluding that no download exists or retrying.")
	}
	h, err := c.History(ctx, 100)
	r.Sources["history"] = "failed"
	if err == nil && h != nil {
		r.Sources["history"] = "succeeded"
		for _, e := range h.Records {
			if e.SeriesID == seriesID && e.EpisodeID == episodeID {
				r.RecentHistory = append(r.RecentHistory, e)
			}
		}
		sort.SliceStable(r.RecentHistory, func(i, j int) bool { return r.RecentHistory[i].Date > r.RecentHistory[j].Date })
		if len(r.RecentHistory) > 0 {
			add("recent_activity", "Matching recent history is available; inspect event types and dates for failed downloads, imports, or later file changes.", "Use the current file and queue observations together with history; an older import does not prove the file still exists.")
		}
		if len(r.RecentHistory) > 10 {
			r.RecentHistory = r.RecentHistory[:10]
		}
	} else {
		r.Limitations = append(r.Limitations, "Recent history could not be read; past download/import activity is unknown.")
	}
	r.Sources["releases"] = "skipped"
	if !r.Episode.HasFile && airErr == nil && !air.After(now) && r.Sources["queue"] == "succeeded" && len(r.Queue) == 0 && !ambiguous {
		releases, err := c.ManualSearch(ctx, episodeID)
		r.Sources["releases"] = "failed"
		if err != nil {
			add("search_unavailable", "The release search failed.", "Check indexer health and retry the investigation later; this is not proof of no releases.")
		} else {
			r.Sources["releases"] = "succeeded"
			r.ReleasesFound = len(releases)
			for _, release := range releases {
				if !release.Rejected && len(release.Rejections) == 0 {
					r.ReleasesAccepted++
				}
			}
			r.Releases = releases
			if len(r.Releases) > 10 {
				r.Releases = r.Releases[:10]
			}
			switch {
			case len(releases) == 0:
				add("no_releases_returned", "This search returned no releases.", "Check indexer health or wait and search again; no results do not prove no release exists.")
			case r.ReleasesAccepted == 0:
				add("releases_rejected", "Sonarr rejected all returned releases.", "Explain the rejection reasons; do not bypass quality settings or blocklists automatically.")
			default:
				add("releases_available", "Sonarr returned releases without rejection reasons, but the episode has no file or matching queue entry.", "Offer a search for this episode only; run it only when the user explicitly requests it.")
			}
		}
	}
	if len(r.Queue) > 10 {
		r.Queue = r.Queue[:10]
		r.Limitations = append(r.Limitations, "Only the first 10 matching queue entries are shown.")
	}
	return r, nil
}
