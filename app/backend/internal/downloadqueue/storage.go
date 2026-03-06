package downloadqueue

import (
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("not found")
)

type Storage interface {
	UpsertClient(rec DownloadClientRecord) DownloadClientRecord
	ListClients() []DownloadClientRecord

	CreateJob(job Job) (Job, error)
	GetJob(id int64) (Job, error)
	ListJobs(filter JobFilter) []Job
	ClaimNextQueued(workerID string, now time.Time) (Job, bool, error)
	RecoverExpiredLeases(now time.Time, limit int) (int, error)
	ListActiveJobs(limit int) []Job
	ListCompletedNotImported(limit int) []Job
	MarkSubmitted(id int64, downloadID string) error
	UpdateProgress(id int64, status JobStatus, outputPath string, lastErr string) error
	MarkImported(id int64, imported bool) error
	Reschedule(id int64, errMsg string, notBefore time.Time, terminal bool) error
	CancelJob(id int64) error
	RetryJob(id int64) error

	AddEvent(event Event) (Event, error)
	ListEvents(jobID int64) []Event

	RecordClientResult(clientID string, success bool, latency time.Duration, terminalComplete bool) error
	RecomputeClientReliability() error
}
