package agent

import (
	"context"
	"encoding/json"

	"github.com/patflynn/reel-life/internal/radarr"
)

func (a *Agent) dispatchRadarr(ctx context.Context, name string, rawInput json.RawMessage) (ToolResult, bool) {
	if a.radarr == nil {
		switch name {
		case "search_movies", "add_movie", "get_movie_queue", "get_movie_history", "check_movie_health", "remove_failed_movie",
			"get_movie_detail", "get_movie_quality_profiles", "get_movie_root_folders", "get_movie_download_clients",
			"get_movie_blocklist", "manual_movie_search", "update_movie_monitoring", "delete_movie",
			"trigger_movie_search", "grab_movie_release", "remove_movie_blocklist_item", "update_movie_profile",
			"get_movie_language_profiles", "get_movie_custom_formats", "update_movie_language_profile":
			return notConfiguredError("Radarr"), true
		}
	}

	var result any
	var err error

	switch name {
	case "search_movies":
		var input searchMoviesInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.radarr.Search(ctx, input.Term)
	case "add_movie":
		var input addMovieInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		minAvail := input.MinimumAvailability
		if minAvail == "" {
			minAvail = "released"
		}
		result, err = a.radarr.Add(ctx, radarr.AddMovieRequest{
			Title:               input.Title,
			TMDBID:              input.TMDBID,
			QualityProfileID:    input.QualityProfileID,
			RootFolderPath:      input.RootFolderPath,
			Monitored:           true,
			MinimumAvailability: minAvail,
		})
	case "get_movie_queue":
		result, err = a.radarr.Queue(ctx)
	case "get_movie_history":
		var input getMovieHistoryInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		pageSize := input.PageSize
		if pageSize == 0 {
			pageSize = 20
		}
		result, err = a.radarr.History(ctx, pageSize)
	case "check_movie_health":
		result, err = a.radarr.Health(ctx)
	case "remove_failed_movie":
		var input removeFailedMovieInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.radarr.RemoveFailed(ctx, input.ID, input.Blocklist)
		if err == nil {
			result = map[string]string{"status": "removed"}
		}
	case "get_movie_detail":
		var input getMovieDetailInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.radarr.GetMovie(ctx, input.MovieID)
	case "get_movie_quality_profiles":
		result, err = a.radarr.GetQualityProfiles(ctx)
	case "get_movie_root_folders":
		result, err = a.radarr.GetRootFolders(ctx)
	case "get_movie_download_clients":
		result, err = a.radarr.GetDownloadClients(ctx)
	case "get_movie_blocklist":
		var input getMovieBlocklistInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		pageSize := input.PageSize
		if pageSize == 0 {
			pageSize = 20
		}
		result, err = a.radarr.GetBlocklist(ctx, pageSize)
	case "manual_movie_search":
		var input manualMovieSearchInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.radarr.ManualSearch(ctx, input.MovieID)
	case "update_movie_monitoring":
		var input updateMovieMonitoringInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		movie, getErr := a.radarr.GetMovie(ctx, input.MovieID)
		if getErr != nil {
			err = getErr
			break
		}
		movie.Monitored = input.Monitored
		result, err = a.radarr.UpdateMovie(ctx, movie)
	case "delete_movie":
		var input deleteMovieInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.radarr.DeleteMovie(ctx, input.MovieID, input.DeleteFiles)
		if err == nil {
			result = map[string]string{"status": "deleted"}
		}
	case "trigger_movie_search":
		var input triggerMovieSearchInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.radarr.Command(ctx, radarr.CommandRequest{
			Name:     radarr.CommandMoviesSearch,
			MovieIDs: []int{input.MovieID},
		})
		if err == nil {
			result = map[string]string{"status": "search triggered"}
		}
	case "grab_movie_release":
		var input grabMovieReleaseInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.radarr.GrabRelease(ctx, input.GUID, input.IndexerID)
		if err == nil {
			result = map[string]string{"status": "grabbed"}
		}
	case "remove_movie_blocklist_item":
		var input removeMovieBlocklistItemInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.radarr.DeleteBlocklistItem(ctx, input.ID)
		if err == nil {
			result = map[string]string{"status": "removed"}
		}
	case "update_movie_profile":
		var input updateMovieProfileInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		movie, getErr := a.radarr.GetMovie(ctx, input.MovieID)
		if getErr != nil {
			err = getErr
			break
		}
		movie.QualityProfileID = input.QualityProfileID
		result, err = a.radarr.UpdateMovie(ctx, movie)
	case "get_movie_language_profiles":
		result, err = a.radarr.GetLanguageProfiles(ctx)
	case "get_movie_custom_formats":
		result, err = a.radarr.GetCustomFormats(ctx)
	case "update_movie_language_profile":
		var input updateMovieLanguageProfileInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		movie, getErr := a.radarr.GetMovie(ctx, input.MovieID)
		if getErr != nil {
			err = getErr
			break
		}
		movie.LanguageProfileID = input.LanguageProfileID
		result, err = a.radarr.UpdateMovie(ctx, movie)
	default:
		return ToolResult{}, false
	}

	if err != nil {
		a.logger.Warn("tool error", "tool", name, "error", err)
		return errorResultFromErr(err), true
	}
	return marshalResult(result), true
}
