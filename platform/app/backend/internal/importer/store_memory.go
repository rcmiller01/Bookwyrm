package importer

import (
	"sort"
	"sync"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/metrics"
)

type MemoryStore struct {
	mu sync.RWMutex

	nextJobID     int64
	nextEventID   int64
	nextLibraryID int64

	jobsByID        map[int64]Job
	downloadJobToID map[int64]int64
	eventsByJobID   map[int64][]Event
	libraryByPath   map[string]LibraryItem
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextJobID:       1,
		nextEventID:     1,
		nextLibraryID:   1,
		jobsByID:        map[int64]Job{},
		downloadJobToID: map[int64]int64{},
		eventsByJobID:   map[int64][]Event{},
		libraryByPath:   map[string]LibraryItem{},
	}
}

func (s *MemoryStore) CreateOrGetFromDownload(download downloadqueue.Job, targetRoot string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.downloadJobToID[download.ID]; ok {
		return s.jobsByID[id], nil
	}
	now := time.Now().UTC()
	job := Job{
		ID:             s.nextJobID,
		DownloadJobID:  download.ID,
		WorkID:         download.WorkID,
		EditionID:      download.EditionID,
		SourcePath:     download.OutputPath,
		TargetRoot:     targetRoot,
		Status:         JobStatusQueued,
		MaxAttempts:    3,
		RenameTemplate: "",
		NamingResult:   map[string]any{},
		Decision:       map[string]any{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.nextJobID++
	s.jobsByID[job.ID] = job
	s.downloadJobToID[download.ID] = job.ID
	metrics.ImportJobsCreatedTotal.Inc()
	return job, nil
}

func (s *MemoryStore) ClaimNextQueued(workerID string, now time.Time) (Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var selected *Job
	for id := range s.jobsByID {
		job := s.jobsByID[id]
		if job.Status != JobStatusQueued {
			continue
		}
		if selected == nil || job.CreatedAt.Before(selected.CreatedAt) {
			tmp := job
			selected = &tmp
		}
	}
	if selected == nil {
		return Job{}, false, nil
	}
	job := *selected
	job.Status = JobStatusRunning
	job.AttemptCount++
	lockTime := now.UTC()
	job.LockedAt = &lockTime
	job.LockedBy = workerID
	leaseExpiresAt := lockTime.Add(ImportJobLeaseTTL)
	job.LeaseExpiresAt = &leaseExpiresAt
	job.UpdatedAt = now.UTC()
	if job.Decision == nil {
		job.Decision = map[string]any{}
	}
	job.Decision["locked_by"] = workerID
	s.jobsByID[job.ID] = job
	return job, true, nil
}

func (s *MemoryStore) RecoverExpiredLeases(now time.Time, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	recovered := 0
	for id, job := range s.jobsByID {
		if recovered >= limit {
			break
		}
		if job.Status != JobStatusRunning || job.LeaseExpiresAt == nil || job.LeaseExpiresAt.After(now.UTC()) {
			continue
		}
		job.AttemptCount++
		if job.AttemptCount >= job.MaxAttempts {
			job.Status = JobStatusFailed
			job.LastError = "lease expired; recovered as failed"
			metrics.ImportJobsFailedTotal.Inc()
			metrics.ObserveImportTerminalDuration(job.CreatedAt)
		} else {
			job.Status = JobStatusQueued
			job.LastError = "lease expired; recovered for retry"
		}
		job.LockedAt = nil
		job.LockedBy = ""
		job.LeaseExpiresAt = nil
		job.UpdatedAt = now.UTC()
		s.jobsByID[id] = job
		event := Event{
			ID:          s.nextEventID,
			ImportJobID: job.ID,
			TS:          now.UTC(),
			EventType:   "lease_recovered",
			Message:     "running lease expired; job recovered",
			Payload: map[string]any{
				"next_status":   string(job.Status),
				"attempt_count": job.AttemptCount,
			},
		}
		s.nextEventID++
		s.eventsByJobID[job.ID] = append(s.eventsByJobID[job.ID], event)
		recovered++
	}
	return recovered, nil
}

func (s *MemoryStore) GetJob(id int64) (Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobsByID[id]
	if !ok {
		return Job{}, ErrNotFound
	}
	return job, nil
}

func (s *MemoryStore) ExistsDownloadJob(downloadJobID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.downloadJobToID[downloadJobID]
	return ok
}

func (s *MemoryStore) ListJobs(filter JobFilter) []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]Job, 0, limit)
	for _, job := range s.jobsByID {
		if filter.Status != "" && job.Status != filter.Status {
			continue
		}
		out = append(out, job)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *MemoryStore) MarkImported(id int64, targetPath string, naming map[string]any, decision map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobsByID[id]
	if !ok {
		return ErrNotFound
	}
	job.Status = JobStatusImported
	job.TargetPath = targetPath
	job.NamingResult = cloneMap(naming)
	job.Decision = cloneMap(decision)
	job.LastError = ""
	job.LockedAt = nil
	job.LockedBy = ""
	job.LeaseExpiresAt = nil
	job.UpdatedAt = time.Now().UTC()
	s.jobsByID[id] = job
	metrics.ImportJobsImportedTotal.Inc()
	metrics.ObserveImportTerminalDuration(job.CreatedAt)
	return nil
}

func (s *MemoryStore) MarkNeedsReview(id int64, reason string, naming map[string]any, decision map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobsByID[id]
	if !ok {
		return ErrNotFound
	}
	job.Status = JobStatusNeedsReview
	job.LastError = reason
	job.NamingResult = cloneMap(naming)
	job.Decision = cloneMap(decision)
	job.LockedAt = nil
	job.LockedBy = ""
	job.LeaseExpiresAt = nil
	job.UpdatedAt = time.Now().UTC()
	s.jobsByID[id] = job
	metrics.ImportJobsNeedsReviewTotal.Inc()
	return nil
}

func (s *MemoryStore) MarkFailed(id int64, errMsg string, terminal bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobsByID[id]
	if !ok {
		return ErrNotFound
	}
	if terminal || job.AttemptCount >= job.MaxAttempts {
		job.Status = JobStatusFailed
	} else {
		job.Status = JobStatusQueued
	}
	job.LastError = errMsg
	job.LockedAt = nil
	job.LockedBy = ""
	job.LeaseExpiresAt = nil
	job.UpdatedAt = time.Now().UTC()
	s.jobsByID[id] = job
	if job.Status == JobStatusFailed {
		metrics.ImportJobsFailedTotal.Inc()
		metrics.ObserveImportTerminalDuration(job.CreatedAt)
	}
	return nil
}

func (s *MemoryStore) Retry(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobsByID[id]
	if !ok {
		return ErrNotFound
	}
	job.Status = JobStatusQueued
	job.LastError = ""
	job.LockedAt = nil
	job.LockedBy = ""
	job.LeaseExpiresAt = nil
	job.UpdatedAt = time.Now().UTC()
	s.jobsByID[id] = job
	return nil
}

func (s *MemoryStore) Approve(id int64, workID string, editionID string, templateOverride string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobsByID[id]
	if !ok {
		return ErrNotFound
	}
	job.WorkID = workID
	job.EditionID = editionID
	if templateOverride != "" {
		job.RenameTemplate = templateOverride
	}
	job.Status = JobStatusQueued
	job.LastError = ""
	job.LockedAt = nil
	job.LockedBy = ""
	job.LeaseExpiresAt = nil
	job.UpdatedAt = time.Now().UTC()
	s.jobsByID[id] = job
	return nil
}

func (s *MemoryStore) Skip(id int64, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobsByID[id]
	if !ok {
		return ErrNotFound
	}
	job.Status = JobStatusSkipped
	job.LastError = reason
	job.LockedAt = nil
	job.LockedBy = ""
	job.LeaseExpiresAt = nil
	job.UpdatedAt = time.Now().UTC()
	s.jobsByID[id] = job
	metrics.ImportJobsSkippedTotal.Inc()
	metrics.ObserveImportTerminalDuration(job.CreatedAt)
	return nil
}

func (s *MemoryStore) AddEvent(importJobID int64, eventType string, message string, payload map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobsByID[importJobID]; !ok {
		return ErrNotFound
	}
	event := Event{
		ID:          s.nextEventID,
		ImportJobID: importJobID,
		TS:          time.Now().UTC(),
		EventType:   eventType,
		Message:     message,
		Payload:     cloneMap(payload),
	}
	s.nextEventID++
	s.eventsByJobID[importJobID] = append(s.eventsByJobID[importJobID], event)
	return nil
}

func (s *MemoryStore) ListEvents(importJobID int64) []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.eventsByJobID[importJobID]
	out := make([]Event, len(events))
	copy(out, events)
	return out
}

func (s *MemoryStore) UpsertLibraryItem(item LibraryItem) (LibraryItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.libraryByPath[item.Path]; ok {
		return existing, nil
	}
	item.ID = s.nextLibraryID
	s.nextLibraryID++
	item.CreatedAt = time.Now().UTC()
	s.libraryByPath[item.Path] = item
	return item, nil
}

func (s *MemoryStore) ListLibraryItems(workID string, limit int) []LibraryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]LibraryItem, 0, limit)
	for _, item := range s.libraryByPath {
		if workID != "" && item.WorkID != workID {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *MemoryStore) CountJobsByStatus() map[JobStatus]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[JobStatus]int{}
	for _, job := range s.jobsByID {
		out[job.Status]++
	}
	return out
}

func (s *MemoryStore) NextRunnableAt() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var next *time.Time
	for _, job := range s.jobsByID {
		if job.Status != JobStatusQueued {
			continue
		}
		ts := job.CreatedAt
		if next == nil || ts.Before(*next) {
			candidate := ts
			next = &candidate
		}
	}
	return next
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
