package domain

import "time"

type JobType string

const (
	JobTypeSearchMissing   JobType = "search_missing"
	JobTypeEnqueueDownload JobType = "enqueue_download"
	JobTypePollDownload    JobType = "poll_download_status"
	JobTypeImportCompleted JobType = "import_completed"
	JobTypeRenameFinalize  JobType = "rename_finalize"
)

type JobState string

const (
	JobStateQueued        JobState = "queued"
	JobStateRunning       JobState = "running"
	JobStateSucceeded     JobState = "succeeded"
	JobStateRetryableFail JobState = "retryable_failed"
	JobStateDeadLetter    JobState = "dead_letter"
	JobStateCanceled      JobState = "canceled"
)

type Job struct {
	ID          string         `json:"id"`
	Type        JobType        `json:"type"`
	State       JobState       `json:"state"`
	Payload     map[string]any `json:"payload,omitempty"`
	Output      map[string]any `json:"output,omitempty"`
	Attempt     int            `json:"attempt"`
	MaxAttempts int            `json:"max_attempts"`
	LastError   string         `json:"last_error,omitempty"`
	RunAt       time.Time      `json:"run_at"`
	LockedAt    *time.Time     `json:"locked_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type JobFilter struct {
	Type   JobType
	State  JobState
	Limit  int
	UserID string
}
