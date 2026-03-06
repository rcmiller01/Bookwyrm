package downloadqueue

import "time"

const DownloadJobLeaseTTL = 2 * time.Minute

type JobStatus string

const (
	JobStatusQueued      JobStatus = "queued"
	JobStatusSubmitted   JobStatus = "submitted"
	JobStatusDownloading JobStatus = "downloading"
	JobStatusRepairing   JobStatus = "repairing"
	JobStatusUnpacking   JobStatus = "unpacking"
	JobStatusCompleted   JobStatus = "completed"
	JobStatusFailed      JobStatus = "failed"
	JobStatusCanceled    JobStatus = "canceled"
)

type DownloadClientRecord struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	ClientType       string         `json:"client_type"`
	Enabled          bool           `json:"enabled"`
	Tier             string         `json:"tier"`
	ReliabilityScore float64        `json:"reliability_score"`
	Priority         int            `json:"priority"`
	Config           map[string]any `json:"config_json,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type Job struct {
	ID             int64          `json:"id"`
	GrabID         int64          `json:"grab_id"`
	CandidateID    int64          `json:"candidate_id"`
	WorkID         string         `json:"work_id"`
	EditionID      string         `json:"edition_id,omitempty"`
	Protocol       string         `json:"protocol"`
	ClientName     string         `json:"client_name"`
	Status         JobStatus      `json:"status"`
	DownloadID     string         `json:"download_id,omitempty"`
	OutputPath     string         `json:"output_path,omitempty"`
	Imported       bool           `json:"imported"`
	RequestPayload map[string]any `json:"request_payload,omitempty"`
	LastError      string         `json:"last_error,omitempty"`
	AttemptCount   int            `json:"attempt_count"`
	MaxAttempts    int            `json:"max_attempts"`
	NotBefore      time.Time      `json:"not_before"`
	LockedAt       *time.Time     `json:"locked_at,omitempty"`
	LockedBy       string         `json:"locked_by,omitempty"`
	LeaseExpiresAt *time.Time     `json:"lease_expires_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type Event struct {
	ID        int64          `json:"id"`
	JobID     int64          `json:"job_id"`
	EventType string         `json:"event_type"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data_json,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type JobFilter struct {
	Status   JobStatus
	Imported *bool
	Limit    int
}
