package model

import "time"

const (
	EnrichmentStatusQueued    = "queued"
	EnrichmentStatusRunning   = "running"
	EnrichmentStatusSucceeded = "succeeded"
	EnrichmentStatusFailed    = "failed"
	EnrichmentStatusDead      = "dead"
	EnrichmentStatusCancelled = "cancelled"
)

const (
	EnrichmentOutcomeSucceeded = "succeeded"
	EnrichmentOutcomeFailed    = "failed"
)

const (
	EnrichmentJobTypeWorkEditions = "work_editions"
	EnrichmentJobTypeAuthorExpand = "author_expand"
)

// EnrichmentJob represents a queued or executed enrichment task.
type EnrichmentJob struct {
	ID           int64      `json:"id"`
	JobType      string     `json:"job_type"`
	EntityType   string     `json:"entity_type"`
	EntityID     string     `json:"entity_id"`
	Status       string     `json:"status"`
	Priority     int        `json:"priority"`
	AttemptCount int        `json:"attempt_count"`
	MaxAttempts  int        `json:"max_attempts"`
	NotBefore    *time.Time `json:"not_before,omitempty"`
	LockedAt     *time.Time `json:"locked_at,omitempty"`
	LockedBy     *string    `json:"locked_by,omitempty"`
	LastError    *string    `json:"last_error,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// EnrichmentJobRun stores one execution attempt for an enrichment job.
type EnrichmentJobRun struct {
	ID         int64      `json:"id"`
	JobID      int64      `json:"job_id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Outcome    string     `json:"outcome"`
	Error      *string    `json:"error,omitempty"`
}

// EnrichmentJobFilters supports lightweight job listing for API and debugging.
type EnrichmentJobFilters struct {
	Status string
	Limit  int
}
