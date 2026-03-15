package importer

import "time"

const ImportJobLeaseTTL = 2 * time.Minute

type JobStatus string

type DecisionAction string

const (
	DecisionKeepBoth        DecisionAction = "keep_both"
	DecisionReplaceExisting DecisionAction = "replace_existing"
	DecisionSkip            DecisionAction = "skip"
)

func IsValidDecisionAction(action DecisionAction) bool {
	switch action {
	case DecisionKeepBoth, DecisionReplaceExisting, DecisionSkip:
		return true
	default:
		return false
	}
}

const (
	JobStatusQueued      JobStatus = "queued"
	JobStatusRunning     JobStatus = "running"
	JobStatusNeedsReview JobStatus = "needs_review"
	JobStatusImported    JobStatus = "imported"
	JobStatusFailed      JobStatus = "failed"
	JobStatusSkipped     JobStatus = "skipped"
)

type Job struct {
	ID             int64          `json:"id"`
	DownloadJobID  int64          `json:"download_job_id"`
	WorkID         string         `json:"work_id,omitempty"`
	EditionID      string         `json:"edition_id,omitempty"`
	SourcePath     string         `json:"source_path"`
	TargetRoot     string         `json:"target_root"`
	TargetPath     string         `json:"target_path,omitempty"`
	Status         JobStatus      `json:"status"`
	AttemptCount   int            `json:"attempt_count"`
	MaxAttempts    int            `json:"max_attempts"`
	RenameTemplate string         `json:"rename_template,omitempty"`
	NamingResult   map[string]any `json:"naming_result_json,omitempty"`
	Decision       map[string]any `json:"decision_json,omitempty"`
	LastError      string         `json:"last_error,omitempty"`
	LockedAt       *time.Time     `json:"locked_at,omitempty"`
	LockedBy       string         `json:"locked_by,omitempty"`
	LeaseExpiresAt *time.Time     `json:"lease_expires_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type Event struct {
	ID          int64          `json:"id"`
	ImportJobID int64          `json:"import_job_id"`
	TS          time.Time      `json:"ts"`
	EventType   string         `json:"event_type"`
	Message     string         `json:"message,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
}

type LibraryItem struct {
	ID        int64     `json:"id"`
	WorkID    string    `json:"work_id"`
	EditionID string    `json:"edition_id,omitempty"`
	Path      string    `json:"path"`
	Format    string    `json:"format"`
	SizeBytes int64     `json:"size_bytes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type JobFilter struct {
	Status JobStatus
	Limit  int
}

type Config struct {
	LibraryRoot             string
	AllowCrossDeviceMove    bool
	MaxScanFiles            int
	TemplateEbook           string
	TemplateAudiobookSingle string
	TemplateAudiobookFolder string
	MaxPathLen              int
	ReplaceColon            bool
	KeepIncoming            bool
	KeepIncomingDays        int
	KeepTrashDays           int
}
