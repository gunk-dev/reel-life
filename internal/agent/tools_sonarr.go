package agent

import "github.com/anthropics/anthropic-sdk-go"

type searchSeriesInput struct {
	Term string `json:"term" jsonschema_description:"The search term to look up TV series"`
}

type addSeriesInput struct {
	Title            string `json:"title" jsonschema_description:"Title of the series to add"`
	TVDBID           int    `json:"tvdb_id" jsonschema_description:"TVDB ID of the series"`
	QualityProfileID int    `json:"quality_profile_id" jsonschema_description:"Quality profile ID to use"`
	RootFolderPath   string `json:"root_folder_path" jsonschema_description:"Root folder path for the series"`
}

type removeFailedInput struct {
	ID        int  `json:"id" jsonschema_description:"Queue item ID to remove"`
	Blocklist bool `json:"blocklist" jsonschema_description:"Whether to add the release to the blocklist"`
}

type getHistoryInput struct {
	PageSize int `json:"page_size,omitempty" jsonschema_description:"Number of history records to return (default 20)"`
}

type getSeriesDetailInput struct {
	SeriesID int `json:"series_id" jsonschema_description:"Sonarr series ID"`
}

type getEpisodesInput struct {
	SeriesID int `json:"series_id" jsonschema_description:"Sonarr series ID"`
}

type getLogsInput struct {
	PageSize int    `json:"page_size,omitempty" jsonschema_description:"Number of log records to return"`
	Level    string `json:"level,omitempty" jsonschema_description:"Filter by log level: info, warn, or error"`
}

type manualSearchInput struct {
	EpisodeID int `json:"episode_id" jsonschema_description:"Episode ID to search releases for"`
}

type getBlocklistInput struct {
	PageSize int `json:"page_size,omitempty" jsonschema_description:"Number of blocklist records to return"`
}

type updateSeriesMonitoringInput struct {
	SeriesID     int  `json:"series_id" jsonschema_description:"Sonarr series ID"`
	SeasonNumber int  `json:"season_number" jsonschema_description:"Season number to update monitoring for"`
	Monitored    bool `json:"monitored" jsonschema_description:"Whether to enable (true) or disable (false) monitoring"`
}

type triggerSeriesSearchInput struct {
	SeriesID     int  `json:"series_id" jsonschema_description:"Sonarr series ID to search for"`
	SeasonNumber *int `json:"season_number,omitempty" jsonschema_description:"Season number to search. If omitted, searches the entire series."`
}

type deleteSeriesInput struct {
	SeriesID    int  `json:"series_id" jsonschema_description:"Sonarr series ID to delete"`
	DeleteFiles bool `json:"delete_files,omitempty" jsonschema_description:"Whether to delete the series files from disk (default false)"`
}

type removeBlocklistItemInput struct {
	ID int `json:"id" jsonschema_description:"Blocklist item ID to remove"`
}

type grabReleaseInput struct {
	GUID      string `json:"guid" jsonschema_description:"Release GUID from manual_search results"`
	IndexerID int    `json:"indexer_id" jsonschema_description:"Indexer ID from manual_search results"`
}

type updateEpisodeMonitoringInput struct {
	EpisodeID int  `json:"episode_id" jsonschema_description:"Sonarr episode ID to update monitoring for"`
	Monitored bool `json:"monitored" jsonschema_description:"Whether to enable (true) or disable (false) monitoring"`
}

type updateSeriesProfileInput struct {
	SeriesID         int `json:"series_id" jsonschema_description:"Sonarr series ID"`
	QualityProfileID int `json:"quality_profile_id" jsonschema_description:"Quality profile ID to assign. Use get_quality_profiles to find available IDs."`
}

type monitorSeasonEpisodesInput struct {
	SeriesID     int  `json:"series_id" jsonschema_description:"Sonarr series ID"`
	SeasonNumber int  `json:"season_number" jsonschema_description:"Season number whose episodes to update monitoring for"`
	Monitored    bool `json:"monitored" jsonschema_description:"Whether to enable (true) or disable (false) monitoring for all episodes in the season"`
}

type updateSeriesLanguageProfileInput struct {
	SeriesID          int `json:"series_id" jsonschema_description:"Sonarr series ID"`
	LanguageProfileID int `json:"language_profile_id" jsonschema_description:"Language profile ID to assign"`
}

type investigateEpisodeInput struct {
	SeriesID  int `json:"series_id" jsonschema_description:"Verified Sonarr series ID from tool results"`
	EpisodeID int `json:"episode_id" jsonschema_description:"Verified Sonarr episode ID from get_episodes, not the episode number"`
}

func sonarrToolDefs() []toolDef {
	return []toolDef{
		{Param: anthropic.ToolParam{Name: "investigate_episode", Description: anthropic.String("Investigate why one episode has not arrived. Read-only: combines episode status, complete queue, limited recent history, and release search/rejection reasons. Does not download, change monitoring, or remove anything. Report missing evidence and limitations."), InputSchema: generateSchema[investigateEpisodeInput]()}},
		{Param: anthropic.ToolParam{Name: "trigger_episode_search", Description: anthropic.String("Start an automatic Sonarr search for exactly one episode, only after an explicit user request to retry/search. May download a release. Checks fresh file, air date, monitoring, and queue state first. Command acceptance is not download success. Never use merely because the user asked why an episode is missing."), InputSchema: generateSchema[investigateEpisodeInput]()}, Mutative: true},
		{
			Param: anthropic.ToolParam{
				Name:        "search_series",
				Description: anthropic.String("Search for TV series by name. Returns matching series with details like title, year, overview, and TVDB ID."),
				InputSchema: generateSchema[searchSeriesInput](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "add_series",
				Description: anthropic.String("Add a TV series to Sonarr for monitoring and automatic downloading. Requires the TVDB ID from a search result."),
				InputSchema: generateSchema[addSeriesInput](),
			},
			Mutative: true,
		},
		{
			Param: anthropic.ToolParam{
				Name:        "get_queue",
				Description: anthropic.String("Get the current download queue showing active and pending downloads with their status."),
				InputSchema: generateSchema[struct{}](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "get_history",
				Description: anthropic.String("Get recent download history showing completed, failed, and imported episodes."),
				InputSchema: generateSchema[getHistoryInput](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "check_health",
				Description: anthropic.String("Check Sonarr system health for warnings and errors like connectivity issues, disk space, or indexer problems."),
				InputSchema: generateSchema[struct{}](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "remove_failed",
				Description: anthropic.String("Remove a failed download from the Sonarr queue. Optionally blocklist the release to prevent re-downloading."),
				InputSchema: generateSchema[removeFailedInput](),
			},
			Destructive: true,
		},
		{
			Param: anthropic.ToolParam{
				Name:        "get_series_detail",
				Description: anthropic.String("Get detailed information about a specific series including episode counts, size on disk, and quality profile."),
				InputSchema: generateSchema[getSeriesDetailInput](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "get_episodes",
				Description: anthropic.String("Get all episodes for a series with season/episode numbers, titles, file status, and air dates."),
				InputSchema: generateSchema[getEpisodesInput](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "get_logs",
				Description: anthropic.String("Get recent Sonarr log entries for debugging. Optionally filter by level (info, warn, error)."),
				InputSchema: generateSchema[getLogsInput](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "manual_search",
				Description: anthropic.String("Search for available releases for a specific episode. Shows indexer, quality, size, and rejection reasons."),
				InputSchema: generateSchema[manualSearchInput](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "get_quality_profiles",
				Description: anthropic.String("List all quality profiles configured in Sonarr with their cutoff settings."),
				InputSchema: generateSchema[struct{}](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "get_blocklist",
				Description: anthropic.String("Get blocklisted releases that Sonarr will not re-download."),
				InputSchema: generateSchema[getBlocklistInput](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "get_root_folders",
				Description: anthropic.String("List root folders with free and total disk space."),
				InputSchema: generateSchema[struct{}](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "get_download_clients",
				Description: anthropic.String("List configured download clients with their status, protocol, and priority."),
				InputSchema: generateSchema[struct{}](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "update_series_monitoring",
				Description: anthropic.String("Enable or disable monitoring for a specific season of a series. Use get_series_detail first to find the series ID."),
				InputSchema: generateSchema[updateSeriesMonitoringInput](),
			},
			Mutative: true,
		},
		{
			Param: anthropic.ToolParam{
				Name:        "trigger_series_search",
				Description: anthropic.String("Trigger a search for downloads for a series or a specific season. Sonarr will search indexers and automatically grab matching releases."),
				InputSchema: generateSchema[triggerSeriesSearchInput](),
			},
			Mutative: true,
		},
		{
			Param: anthropic.ToolParam{
				Name:        "delete_series",
				Description: anthropic.String("Delete a series from Sonarr. Optionally delete the series files from disk."),
				InputSchema: generateSchema[deleteSeriesInput](),
			},
			Destructive: true,
		},
		{
			Param: anthropic.ToolParam{
				Name:        "remove_blocklist_item",
				Description: anthropic.String("Remove an item from the blocklist, allowing Sonarr to download that release again."),
				InputSchema: generateSchema[removeBlocklistItemInput](),
			},
			Destructive: true,
		},
		{
			Param: anthropic.ToolParam{
				Name:        "grab_release",
				Description: anthropic.String("Download a specific release found via manual_search. Requires the GUID and indexer ID from the search results."),
				InputSchema: generateSchema[grabReleaseInput](),
			},
			Mutative: true,
		},
		{
			Param: anthropic.ToolParam{
				Name:        "update_episode_monitoring",
				Description: anthropic.String("Enable or disable monitoring for a single episode. Use get_episodes first to find the episode ID."),
				InputSchema: generateSchema[updateEpisodeMonitoringInput](),
			},
			Mutative: true,
		},
		{
			Param: anthropic.ToolParam{
				Name:        "monitor_season_episodes",
				Description: anthropic.String("Enable or disable monitoring for all episodes in a specific season. Use get_series_detail first to find the series ID."),
				InputSchema: generateSchema[monitorSeasonEpisodesInput](),
			},
			Mutative: true,
		},
		{
			Param: anthropic.ToolParam{
				Name:        "update_series_profile",
				Description: anthropic.String("Update the quality profile for a series. Use get_quality_profiles to find available profile IDs and get_series_detail to see the current profile."),
				InputSchema: generateSchema[updateSeriesProfileInput](),
			},
			Mutative: true,
		},
		{
			Param: anthropic.ToolParam{
				Name:        "get_language_profiles",
				Description: anthropic.String("List all language profiles configured in Sonarr."),
				InputSchema: generateSchema[struct{}](),
			},
		},
		{
			Param: anthropic.ToolParam{
				Name:        "update_series_language_profile",
				Description: anthropic.String("Update the language profile assigned to a series. Fetches the series, sets the language profile ID, and saves it back."),
				InputSchema: generateSchema[updateSeriesLanguageProfileInput](),
			},
			Mutative: true,
		},
	}
}
