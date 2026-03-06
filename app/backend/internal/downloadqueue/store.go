package downloadqueue

import (
	"sort"
	"sync"
	"time"
)

type Store struct {
	mu sync.RWMutex

	nextClientID int64
	nextJobID    int64
	nextEventID  int64

	clients map[string]DownloadClientRecord
	jobs    map[int64]Job
	events  map[int64][]Event
}

func NewStore() *Store {
	return &Store{
		nextClientID: 1,
		nextJobID:    1,
		nextEventID:  1,
		clients:      map[string]DownloadClientRecord{},
		jobs:         map[int64]Job{},
		events:       map[int64][]Event{},
	}
}

func (s *Store) UpsertClient(rec DownloadClientRecord) DownloadClientRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	existing, ok := s.clients[rec.ID]
	if ok {
		rec.CreatedAt = existing.CreatedAt
	} else {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	s.clients[rec.ID] = rec
	return rec
}

func (s *Store) ListClients() []DownloadClientRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DownloadClientRecord, 0, len(s.clients))
	for _, rec := range s.clients {
		out = append(out, rec)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].ID < out[j].ID
		}
		return out[i].Priority < out[j].Priority
	})
	return out
}

func (s *Store) CreateJob(job Job) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if job.MaxAttempts <= 0 {
		job.MaxAttempts = 3
	}
	if job.NotBefore.IsZero() {
		job.NotBefore = now
	}
	job.ID = s.nextJobID
	s.nextJobID++
	job.Status = JobStatusQueued
	job.AttemptCount = 0
	job.CreatedAt = now
	job.UpdatedAt = now
	s.jobs[job.ID] = job
	return job, nil
}

func (s *Store) GetJob(id int64) (Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return Job{}, ErrNotFound
	}
	return job, nil
}

func (s *Store) ListJobs(filter JobFilter) []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]Job, 0, limit)
	for _, job := range s.jobs {
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

func (s *Store) ClaimNextQueued(workerID string, now time.Time) (Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var selected *Job
	for id := range s.jobs {
		job := s.jobs[id]
		if job.Status != JobStatusQueued {
			continue
		}
		if job.NotBefore.After(now) {
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
	lockTime := now.UTC()
	job.Status = JobStatusSubmitted
	job.AttemptCount++
	job.LockedAt = &lockTime
	job.LockedBy = workerID
	job.UpdatedAt = lockTime
	s.jobs[job.ID] = job
	return job, true, nil
}

func (s *Store) ListActiveJobs(limit int) []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]Job, 0, limit)
	for _, job := range s.jobs {
		switch job.Status {
		case JobStatusSubmitted, JobStatusDownloading, JobStatusRepairing, JobStatusUnpacking:
			out = append(out, job)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) MarkSubmitted(id int64, downloadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now().UTC()
	job.DownloadID = downloadID
	job.Status = JobStatusDownloading
	job.LockedAt = nil
	job.LockedBy = ""
	job.UpdatedAt = now
	s.jobs[id] = job
	return nil
}

func (s *Store) UpdateProgress(id int64, status JobStatus, outputPath string, lastErr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return ErrNotFound
	}
	job.Status = status
	job.LastError = lastErr
	if outputPath != "" {
		job.OutputPath = outputPath
	}
	job.UpdatedAt = time.Now().UTC()
	s.jobs[id] = job
	return nil
}

func (s *Store) Reschedule(id int64, errMsg string, notBefore time.Time, terminal bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return ErrNotFound
	}
	if terminal || job.AttemptCount >= job.MaxAttempts {
		job.Status = JobStatusFailed
	} else {
		job.Status = JobStatusQueued
	}
	job.LastError = errMsg
	job.NotBefore = notBefore.UTC()
	job.LockedAt = nil
	job.LockedBy = ""
	job.UpdatedAt = time.Now().UTC()
	s.jobs[id] = job
	return nil
}

func (s *Store) CancelJob(id int64) error {
	return s.UpdateProgress(id, JobStatusCanceled, "", "")
}

func (s *Store) RetryJob(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return ErrNotFound
	}
	job.Status = JobStatusQueued
	job.LastError = ""
	job.NotBefore = time.Now().UTC()
	job.LockedAt = nil
	job.LockedBy = ""
	job.UpdatedAt = time.Now().UTC()
	s.jobs[id] = job
	return nil
}

func (s *Store) AddEvent(event Event) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[event.JobID]; !ok {
		return Event{}, ErrNotFound
	}
	event.ID = s.nextEventID
	s.nextEventID++
	event.CreatedAt = time.Now().UTC()
	s.events[event.JobID] = append(s.events[event.JobID], event)
	return event, nil
}
