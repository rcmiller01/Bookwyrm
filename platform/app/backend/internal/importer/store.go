package importer

import (
	"errors"
	"time"

	"app-backend/internal/downloadqueue"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	CreateOrGetFromDownload(download downloadqueue.Job, targetRoot string) (Job, error)
	ClaimNextQueued(workerID string, now time.Time) (Job, bool, error)
	RecoverExpiredLeases(now time.Time, limit int) (int, error)
	GetJob(id int64) (Job, error)
	ExistsDownloadJob(downloadJobID int64) bool
	ListJobs(filter JobFilter) []Job
	MarkImported(id int64, targetPath string, naming map[string]any, decision map[string]any) error
	MarkNeedsReview(id int64, reason string, naming map[string]any, decision map[string]any) error
	MarkFailed(id int64, errMsg string, terminal bool) error
	Retry(id int64) error
	Approve(id int64, workID string, editionID string, templateOverride string) error
	Skip(id int64, reason string) error
	AddEvent(importJobID int64, eventType string, message string, payload map[string]any) error
	ListEvents(importJobID int64) []Event

	UpsertLibraryItem(item LibraryItem) (LibraryItem, error)
	ListLibraryItems(workID string, limit int) []LibraryItem
	CountJobsByStatus() map[JobStatus]int
	NextRunnableAt() *time.Time
}
