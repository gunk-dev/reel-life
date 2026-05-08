package sonarr

type Season struct {
	SeasonNumber int  `json:"seasonNumber"`
	Monitored    bool `json:"monitored"`
}

type Series struct {
	ID               int      `json:"id"`
	Title            string   `json:"title"`
	Year             int      `json:"year"`
	TVDBID           int      `json:"tvdbId"`
	Status           string   `json:"status"`
	Overview         string   `json:"overview"`
	Monitored        bool     `json:"monitored"`
	SeasonCount      int      `json:"seasonCount"`
	Seasons          []Season `json:"seasons,omitempty"`
	EpisodeCount     int      `json:"episodeCount,omitempty"`
	EpisodeFileCount int      `json:"episodeFileCount,omitempty"`
	SizeOnDisk       int64    `json:"sizeOnDisk,omitempty"`
	RootFolderPath   string   `json:"rootFolderPath"`
	Path             string   `json:"path,omitempty"`
	QualityProfileID  int      `json:"qualityProfileId"`
	LanguageProfileID int      `json:"languageProfileId"`
}

type AddSeriesRequest struct {
	Title            string `json:"title"`
	TVDBID           int    `json:"tvdbId"`
	QualityProfileID int    `json:"qualityProfileId"`
	RootFolderPath   string `json:"rootFolderPath"`
	Monitored        bool   `json:"monitored"`
	SeasonFolder     bool   `json:"seasonFolder"`
}

type QueuePage struct {
	Page         int         `json:"page"`
	PageSize     int         `json:"pageSize"`
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

type QueueItem struct {
	ID                    int     `json:"id"`
	SeriesID              int     `json:"seriesId"`
	Title                 string  `json:"title"`
	Status                string  `json:"status"`
	TrackedDownloadStatus string  `json:"trackedDownloadStatus"`
	TrackedDownloadState  string  `json:"trackedDownloadState"`
	Sizeleft              float64 `json:"sizeleft"`
	Size                  float64 `json:"size"`
}

type HistoryPage struct {
	Page         int             `json:"page"`
	PageSize     int             `json:"pageSize"`
	TotalRecords int             `json:"totalRecords"`
	Records      []HistoryRecord `json:"records"`
}

type HistoryRecord struct {
	ID          int    `json:"id"`
	SeriesID    int    `json:"seriesId"`
	EpisodeID   int    `json:"episodeId"`
	SourceTitle string `json:"sourceTitle"`
	EventType   string `json:"eventType"`
	Date        string `json:"date"`
}

type HealthCheck struct {
	Source  string `json:"source"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

type Episode struct {
	ID            int    `json:"id"`
	SeriesID      int    `json:"seriesId"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Title         string `json:"title"`
	HasFile       bool   `json:"hasFile"`
	Monitored     bool   `json:"monitored"`
	AirDateUTC    string `json:"airDateUtc,omitempty"`
}

type LogRecord struct {
	Time      string `json:"time"`
	Level     string `json:"level"`
	Logger    string `json:"logger"`
	Message   string `json:"message"`
	Exception string `json:"exception,omitempty"`
}

type LogPage struct {
	Records []LogRecord `json:"records"`
}

type Release struct {
	GUID       string   `json:"guid"`
	IndexerID  int      `json:"indexerId"`
	Title      string   `json:"title"`
	Indexer    string   `json:"indexer"`
	Quality    Quality  `json:"quality"`
	Size       int64    `json:"size"`
	Age        int      `json:"age"`
	Rejected   bool     `json:"rejected"`
	Rejections []string `json:"rejections,omitempty"`
}

type Quality struct {
	Definition QualityDefinition `json:"quality"`
	Revision   Revision          `json:"revision"`
}

type QualityDefinition struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source"`
	Resolution int    `json:"resolution"`
	Modifier   string `json:"modifier"`
}

type Revision struct {
	Version  int  `json:"version"`
	Real     int  `json:"real"`
	IsRepack bool `json:"isRepack"`
}

type QualityProfile struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Cutoff int    `json:"cutoff"`
}

type BlocklistItem struct {
	ID          int    `json:"id"`
	SeriesID    int    `json:"seriesId"`
	SourceTitle string `json:"sourceTitle"`
	Date        string `json:"date"`
}

type BlocklistPage struct {
	TotalRecords int             `json:"totalRecords"`
	Records      []BlocklistItem `json:"records"`
}

type RootFolder struct {
	Path       string `json:"path"`
	FreeSpace  int64  `json:"freeSpace"`
	TotalSpace int64  `json:"totalSpace"`
}

type DownloadClient struct {
	Name     string `json:"name"`
	Enable   bool   `json:"enable"`
	Protocol string `json:"protocol"`
	Priority int    `json:"priority"`
}

type CommandRequest struct {
	Name         string `json:"name"`
	SeriesID     int    `json:"seriesId,omitempty"`
	SeasonNumber *int   `json:"seasonNumber,omitempty"`
}

type CommandResource struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Queued    string `json:"queued,omitempty"`
	Started   string `json:"started,omitempty"`
	Ended     string `json:"ended,omitempty"`
	Priority  string `json:"priority,omitempty"`
	Trigger   string `json:"trigger,omitempty"`
	SendUpdatesToClient bool `json:"sendUpdatesToClient"`
}

type GrabReleaseRequest struct {
	GUID      string `json:"guid"`
	IndexerID int    `json:"indexerId"`
}

type MonitorEpisodesRequest struct {
	EpisodeIDs []int `json:"episodeIds"`
	Monitored  bool  `json:"monitored"`
}

type LanguageProfile struct {
	ID        int            `json:"id"`
	Name      string         `json:"name"`
	Languages []LanguageItem `json:"languages"`
}

type LanguageItem struct {
	Language Language `json:"language"`
	Allowed  bool     `json:"allowed"`
}

type Language struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
