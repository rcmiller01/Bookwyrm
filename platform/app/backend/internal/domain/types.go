package domain

type Work struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type SearchResponse struct {
	Works []Work `json:"works"`
}

type WorkResponse struct {
	Work map[string]any `json:"work"`
}

type GraphResponse struct {
	WorkID string `json:"work_id"`
}

type RecommendationsResponse struct {
	SeedWorkID      string         `json:"seed_work_id"`
	Recommendations []map[string]any `json:"recommendations"`
}

type QualityReportResponse struct {
	Report map[string]any `json:"report"`
}

type QualityRepairRequest struct {
	Limit                    int   `json:"limit,omitempty"`
	DryRun                   bool  `json:"dry_run"`
	RemoveInvalidIdentifiers *bool `json:"remove_invalid_identifiers,omitempty"`
}

type QualityRepairResponse struct {
	Result map[string]any `json:"result"`
}

type WorkIntelligence struct {
	Work            map[string]any `json:"work"`
	Graph           map[string]any `json:"graph"`
	Recommendations []map[string]any `json:"recommendations"`
}

type WatchTargetType string

const (
	WatchTargetWork   WatchTargetType = "work"
	WatchTargetAuthor WatchTargetType = "author"
	WatchTargetSeries WatchTargetType = "series"
)

type WatchlistItem struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	TargetType WatchTargetType `json:"target_type"`
	TargetID   string          `json:"target_id"`
	Label      string          `json:"label"`
}
