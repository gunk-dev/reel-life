package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/patflynn/reel-life/internal/sonarr"
)

func (a *Agent) dispatchSonarr(ctx context.Context, name string, rawInput json.RawMessage) (ToolResult, bool) {
	if a.sonarr == nil {
		switch name {
		case "search_series", "add_series", "get_queue", "get_history", "check_health", "remove_failed",
			"get_series_detail", "get_episodes", "get_logs", "manual_search", "get_quality_profiles",
			"get_blocklist", "get_root_folders", "get_download_clients", "update_series_monitoring",
			"trigger_series_search", "delete_series", "remove_blocklist_item", "grab_release",
			"update_episode_monitoring", "monitor_season_episodes", "update_series_profile",
			"get_language_profiles", "update_series_language_profile":
			return notConfiguredError("Sonarr"), true
		}
	}

	var result any
	var err error

	switch name {
	case "search_series":
		var input searchSeriesInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.sonarr.Search(ctx, input.Term)

	case "add_series":
		var input addSeriesInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.sonarr.Add(ctx, sonarr.AddSeriesRequest{
			Title:            input.Title,
			TVDBID:           input.TVDBID,
			QualityProfileID: input.QualityProfileID,
			RootFolderPath:   input.RootFolderPath,
			Monitored:        true,
			SeasonFolder:     true,
		})

	case "get_queue":
		result, err = a.sonarr.Queue(ctx)

	case "get_history":
		var input getHistoryInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		pageSize := input.PageSize
		if pageSize == 0 {
			pageSize = 20
		}
		result, err = a.sonarr.History(ctx, pageSize)

	case "check_health":
		result, err = a.sonarr.Health(ctx)

	case "remove_failed":
		var input removeFailedInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.sonarr.RemoveFailed(ctx, input.ID, input.Blocklist)
		if err == nil {
			result = map[string]string{"status": "removed"}
		}

	case "get_series_detail":
		var input getSeriesDetailInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.sonarr.GetSeries(ctx, input.SeriesID)

	case "get_episodes":
		var input getEpisodesInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.sonarr.GetEpisodes(ctx, input.SeriesID)

	case "get_logs":
		var input getLogsInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.sonarr.GetLogs(ctx, input.PageSize, input.Level)

	case "manual_search":
		var input manualSearchInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.sonarr.ManualSearch(ctx, input.EpisodeID)

	case "get_quality_profiles":
		result, err = a.sonarr.GetQualityProfiles(ctx)

	case "get_blocklist":
		var input getBlocklistInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.sonarr.GetBlocklist(ctx, input.PageSize)

	case "get_root_folders":
		result, err = a.sonarr.GetRootFolders(ctx)

	case "get_download_clients":
		result, err = a.sonarr.GetDownloadClients(ctx)

	case "update_series_monitoring":
		var input updateSeriesMonitoringInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		series, getErr := a.sonarr.GetSeries(ctx, input.SeriesID)
		if getErr != nil {
			err = getErr
			break
		}
		found := false
		for i, s := range series.Seasons {
			if s.SeasonNumber == input.SeasonNumber {
				series.Seasons[i].Monitored = input.Monitored
				found = true
				break
			}
		}
		if !found {
			return errorResult("not_found", fmt.Sprintf("season %d not found in series %q", input.SeasonNumber, series.Title), false), true
		}
		result, err = a.sonarr.UpdateSeries(ctx, series)

	case "trigger_series_search":
		var input triggerSeriesSearchInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		cmd := sonarr.CommandRequest{
			Name:     "SeriesSearch",
			SeriesID: input.SeriesID,
		}
		if input.SeasonNumber != nil {
			cmd.Name = "SeasonSearch"
			cmd.SeasonNumber = input.SeasonNumber
		}
		result, err = a.sonarr.Command(ctx, cmd)

	case "delete_series":
		var input deleteSeriesInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.sonarr.DeleteSeries(ctx, input.SeriesID, input.DeleteFiles)
		if err == nil {
			result = map[string]string{"status": "deleted"}
		}

	case "remove_blocklist_item":
		var input removeBlocklistItemInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.sonarr.DeleteBlocklistItem(ctx, input.ID)
		if err == nil {
			result = map[string]string{"status": "removed"}
		}

	case "grab_release":
		var input grabReleaseInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.sonarr.GrabRelease(ctx, input.GUID, input.IndexerID)

	case "update_episode_monitoring":
		var input updateEpisodeMonitoringInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.sonarr.MonitorEpisodes(ctx, []int{input.EpisodeID}, input.Monitored)
		if err == nil {
			result = map[string]string{"status": "updated"}
		}

	case "monitor_season_episodes":
		var input monitorSeasonEpisodesInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		episodes, getErr := a.sonarr.GetEpisodes(ctx, input.SeriesID, input.SeasonNumber)
		if getErr != nil {
			err = getErr
			break
		}
		var ids []int
		for _, ep := range episodes {
			ids = append(ids, ep.ID)
		}
		if len(ids) == 0 {
			return errorResult("not_found", fmt.Sprintf("no episodes found for season %d", input.SeasonNumber), false), true
		}
		err = a.sonarr.MonitorEpisodes(ctx, ids, input.Monitored)
		if err == nil {
			result = map[string]any{"status": "updated", "episodeCount": len(ids)}
		}

	case "update_series_profile":
		var input updateSeriesProfileInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		series, getErr := a.sonarr.GetSeries(ctx, input.SeriesID)
		if getErr != nil {
			err = getErr
			break
		}
		series.QualityProfileID = input.QualityProfileID
		result, err = a.sonarr.UpdateSeries(ctx, series)

	case "get_language_profiles":
		result, err = a.sonarr.GetLanguageProfiles(ctx)

	case "update_series_language_profile":
		var input updateSeriesLanguageProfileInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		series, getErr := a.sonarr.GetSeries(ctx, input.SeriesID)
		if getErr != nil {
			err = getErr
			break
		}
		series.LanguageProfileID = input.LanguageProfileID
		result, err = a.sonarr.UpdateSeries(ctx, series)

	default:
		return ToolResult{}, false
	}

	if err != nil {
		a.logger.Warn("tool error", "tool", name, "error", err)
		return errorResultFromErr(err), true
	}
	return marshalResult(result), true
}
