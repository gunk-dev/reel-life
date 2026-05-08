package radarr

type Movie struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	Year             int    `json:"year"`
	TMDBID           int    `json:"tmdbId"`
	Status           string `json:"status"`
	Overview         string `json:"overview"`
	Monitored        bool   `json:"monitored"`
	HasFile          bool   `json:"hasFile"`
	RootFolderPath   string `json:"rootFolderPath"`
	QualityProfileID  int    `json:"qualityProfileId"`
	LanguageProfileID int    `json:"languageProfileId"`
}

type AddMovieRequest struct {
	Title               string `json:"title"`
	TMDBID              int    `json:"tmdbId"`
	QualityProfileID    int    `json:"qualityProfileId"`
	RootFolderPath      string `json:"rootFolderPath"`
	Monitored           bool   `json:"monitored"`
	MinimumAvailability string `json:"minimumAvailability"`
}

type QueuePage struct {
	Page         int         `json:"page"`
	PageSize     int         `json:"pageSize"`
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

type QueueItem struct {
	ID                    int     `json:"id"`
	MovieID               int     `json:"movieId"`
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
	MovieID     int    `json:"movieId"`
	SourceTitle string `json:"sourceTitle"`
	EventType   string `json:"eventType"`
	Date        string `json:"date"`
}

type HealthCheck struct {
	Source  string `json:"source"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

type QualityProfile struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Cutoff int    `json:"cutoff"`
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

type Release struct {
	GUID       string   `json:"guid"`
	Title      string   `json:"title"`
	Indexer    string   `json:"indexer"`
	IndexerID  int      `json:"indexerId"`
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

type BlocklistItem struct {
	ID          int    `json:"id"`
	MovieID     int    `json:"movieId"`
	SourceTitle string `json:"sourceTitle"`
	Date        string `json:"date"`
}

type BlocklistPage struct {
	TotalRecords int             `json:"totalRecords"`
	Records      []BlocklistItem `json:"records"`
}

const (
	CommandMoviesSearch = "MoviesSearch"
)

type CommandRequest struct {
	Name     string `json:"name"`
	MovieIDs []int  `json:"movieIds,omitempty"`
}

type GrabReleaseRequest struct {
	GUID      string `json:"guid"`
	IndexerID int    `json:"indexerId"`
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

type CustomFormat struct {
	ID                              int                `json:"id"`
	Name                            string             `json:"name"`
	IncludeCustomFormatWhenRenaming bool               `json:"includeCustomFormatWhenRenaming"`
	Specifications                  []CustomFormatSpec `json:"specifications"`
}

type CustomFormatSpec struct {
	Name           string `json:"name"`
	Implementation string `json:"implementation"`
	Negate         bool   `json:"negate"`
	Required       bool   `json:"required"`
}
