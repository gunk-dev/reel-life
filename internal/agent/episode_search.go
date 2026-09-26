package agent

import (
	"context"
	"strings"
	"time"

	"github.com/patflynn/reel-life/internal/sonarr"
)

func (a *Agent) searchEpisode(ctx context.Context, input investigateEpisodeInput) ToolResult {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	refuse := func(message string) ToolResult { return errorResult("invalid_input", message, false) }
	s, err := a.sonarr.GetSeries(ctx, input.SeriesID)
	if err != nil {
		return errorResultFromErr(err)
	}
	if s == nil || s.ID != input.SeriesID || !s.Monitored {
		return refuse("Verify the series identity and monitoring before requesting an episode search.")
	}
	episodes, err := a.sonarr.GetEpisodes(ctx, input.SeriesID)
	if err != nil {
		return errorResultFromErr(err)
	}
	var target sonarr.Episode
	matches := 0
	for _, ep := range episodes {
		if ep.ID == input.EpisodeID && ep.SeriesID == input.SeriesID {
			target = ep
			matches++
		}
	}
	if matches != 1 {
		return refuse("Episode identity could not be uniquely verified in this series.")
	}
	air, err := time.Parse(time.RFC3339, target.AirDateUTC)
	if target.HasFile || !target.Monitored || err != nil || air.After(time.Now()) {
		return refuse("Search not started: the episode has a file, is unmonitored, or has not verifiably aired.")
	}
	queue, err := a.sonarr.Queue(ctx)
	if err != nil {
		return errorResultFromErr(err)
	}
	if queue == nil {
		return refuse("The queue could not be verified.")
	}
	for _, item := range queue.Records {
		if item.SeriesID == input.SeriesID && (item.EpisodeID == input.EpisodeID || item.EpisodeID == 0) {
			return refuse("Search not started: a matching or ambiguous series download is already queued.")
		}
	}
	cmd, err := a.sonarr.Command(ctx, sonarr.CommandRequest{Name: "EpisodeSearch", EpisodeIDs: []int{input.EpisodeID}})
	// A timeout may follow acceptance; never encourage an automatic command retry.
	if err != nil {
		r := errorResultFromErr(err)
		r.Retryable = false
		return r
	}
	if cmd == nil || cmd.ID <= 0 {
		return errorResult("unknown", "Search response lacked a command ID; acceptance is unknown. Check Sonarr before retrying.", false)
	}
	if strings.EqualFold(cmd.Status, "failed") {
		return errorResult("unknown", "Sonarr returned a failed search command. Check its status before retrying.", false)
	}
	return marshalResult(map[string]any{"command": cmd, "episode_id": input.EpisodeID, "verification": "Search request returned a command ID. Download, import, and playback are not verified."})
}
