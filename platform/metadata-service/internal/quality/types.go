package quality

import (
	"context"
	"time"
)

type SeriesAnomaly struct {
	SeriesID      string `json:"series_id"`
	SeriesName    string `json:"series_name"`
	EntryCount    int    `json:"entry_count"`
	MissingIndex  int    `json:"missing_index"`
	DuplicateSlot bool   `json:"duplicate_slot"`
	Reason        string `json:"reason"`
}

type PublicationYearConflict struct {
	WorkID string `json:"work_id"`
	Title  string `json:"title"`
	Years  []int  `json:"years"`
}

type DuplicateEdition struct {
	WorkID       string   `json:"work_id"`
	WorkTitle    string   `json:"work_title"`
	CanonicalKey string   `json:"canonical_key"`
	EditionIDs   []string `json:"edition_ids"`
}

type InvalidIdentifier struct {
	EditionID string `json:"edition_id"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	Reason    string `json:"reason"`
}

type IdentifierCandidate struct {
	EditionID string
	Type      string
	Value     string
}

type AuditReport struct {
	GeneratedAt              time.Time                 `json:"generated_at"`
	SeriesAnomalies          []SeriesAnomaly           `json:"series_anomalies"`
	PublicationYearConflicts []PublicationYearConflict `json:"publication_year_conflicts"`
	DuplicateEditions        []DuplicateEdition        `json:"duplicate_editions"`
	InvalidIdentifiers       []InvalidIdentifier       `json:"invalid_identifiers"`
}

type RepairRequest struct {
	Limit                    int
	DryRun                   bool
	RemoveInvalidIdentifiers bool
}

type RepairResult struct {
	DryRun                     bool `json:"dry_run"`
	SeriesReordered            int  `json:"series_reordered"`
	PublicationYearsNormalized int  `json:"publication_years_normalized"`
	InvalidIdentifiersRemoved  int  `json:"invalid_identifiers_removed"`
	ExaminedSeriesAnomalies    int  `json:"examined_series_anomalies"`
	ExaminedYearConflicts      int  `json:"examined_year_conflicts"`
	ExaminedInvalidIdentifiers int  `json:"examined_invalid_identifiers"`
}

type Engine interface {
	Audit(ctx context.Context, limit int) (*AuditReport, error)
	Repair(ctx context.Context, req RepairRequest) (*RepairResult, error)
}
