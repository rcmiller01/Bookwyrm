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
	metrics map[string]clientMetrics
}

type clientMetrics struct {
	SuccessCount    int64
	FailureCount    int64
	TotalLatencyMS  int64
	PollCount       int64
	CompletionCount int64
}

func NewStore() *Store {
	return &Store{
		nextClientID: 1,
		nextJobID:    1,
		nextEventID:  1,
		clients:      map[string]DownloadClientRecord{},
		jobs:         map[int64]Job{},
		events:       map[int64][]Event{},
		metrics:      map[string]clientMetrics{},
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
	if rec.Tier == "" {
		rec.Tier = "unclassified"
	}
	if rec.ReliabilityScore == 0 {
		rec.ReliabilityScore = 0.70
	}
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
		if rankTier(out[i].Tier) != rankTier(out[j].Tier) {
			return rankTier(out[i].Tier) < rankTier(out[j].Tier)
		}
		if out[i].ReliabilityScore != out[j].ReliabilityScore {
			return out[i].ReliabilityScore > out[j].ReliabilityScore
		}
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
	job.Imported = false
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
		if filter.Imported != nil && job.Imported != *filter.Imported {
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
	leaseExpiresAt := lockTime.Add(DownloadJobLeaseTTL)
	job.LeaseExpiresAt = &leaseExpiresAt
	job.UpdatedAt = lockTime
	s.jobs[job.ID] = job
	return job, true, nil
}

func (s *Store) RecoverExpiredLeases(now time.Time, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	recovered := 0
	for id := range s.jobs {
		if recovered >= limit {
			break
		}
		job := s.jobs[id]
		if job.Status != JobStatusSubmitted || job.LeaseExpiresAt == nil || job.LeaseExpiresAt.After(now) {
			continue
		}
		nextAttempt := job.AttemptCount + 1
		job.AttemptCount = nextAttempt
		job.LastError = "lease expired; recovered"
		job.LockedAt = nil
		job.LockedBy = ""
		job.LeaseExpiresAt = nil
		job.UpdatedAt = now.UTC()
		if nextAttempt >= job.MaxAttempts {
			job.Status = JobStatusFailed
			job.NotBefore = now.UTC()
		} else {
			job.Status = JobStatusQueued
			job.NotBefore = now.UTC().Add(recoveryBackoffForAttempt(nextAttempt))
		}
		s.jobs[id] = job
		event := Event{
			ID:        s.nextEventID,
			JobID:     job.ID,
			EventType: "lease_recovered",
			Message:   "submitted lease expired; job recovered",
			Data: map[string]any{
				"next_status":   string(job.Status),
				"attempt_count": job.AttemptCount,
			},
			CreatedAt: now.UTC(),
		}
		s.nextEventID++
		s.events[job.ID] = append(s.events[job.ID], event)
		recovered++
	}
	return recovered, nil
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

func (s *Store) ListCompletedNotImported(limit int) []Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]Job, 0, limit)
	for _, job := range s.jobs {
		if job.Status == JobStatusCompleted && !job.Imported {
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
	job.LeaseExpiresAt = nil
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
	job.LeaseExpiresAt = nil
	job.UpdatedAt = time.Now().UTC()
	s.jobs[id] = job
	return nil
}

func (s *Store) MarkImported(id int64, imported bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return ErrNotFound
	}
	job.Imported = imported
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
	job.LeaseExpiresAt = nil
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
	job.LeaseExpiresAt = nil
	job.UpdatedAt = time.Now().UTC()
	s.jobs[id] = job
	return nil
}

func recoveryBackoffForAttempt(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Duration(1<<minInt(attempt, 7)) * time.Second
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
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

func (s *Store) ListEvents(jobID int64) []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := s.events[jobID]
	if len(events) == 0 {
		return []Event{}
	}
	out := make([]Event, len(events))
	copy(out, events)
	return out
}

func (s *Store) RecordClientResult(clientID string, success bool, latency time.Duration, terminalComplete bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.clients[clientID]; !ok {
		return ErrNotFound
	}
	m := s.metrics[clientID]
	m.PollCount++
	m.TotalLatencyMS += latency.Milliseconds()
	if success {
		m.SuccessCount++
	} else {
		m.FailureCount++
	}
	if terminalComplete {
		m.CompletionCount++
	}
	s.metrics[clientID] = m
	return nil
}

func (s *Store) RecomputeClientReliability() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for clientID, m := range s.metrics {
		rec, ok := s.clients[clientID]
		if !ok {
			continue
		}
		availability := 0.70
		total := m.SuccessCount + m.FailureCount
		if total > 0 {
			availability = float64(m.SuccessCount) / float64(total)
		}
		latencyScore := 0.70
		if m.PollCount > 0 {
			avg := float64(m.TotalLatencyMS) / float64(m.PollCount)
			latencyScore = 1.0 - (avg / 5000.0)
			if latencyScore < 0 {
				latencyScore = 0
			}
		}
		completion := 0.70
		if m.PollCount > 0 {
			completion = float64(m.CompletionCount) / float64(m.PollCount)
		}
		score := (availability * 0.50) + (latencyScore * 0.30) + (completion * 0.20)
		rec.ReliabilityScore = score
		rec.Tier = reliabilityTier(score)
		rec.UpdatedAt = time.Now().UTC()
		s.clients[clientID] = rec
	}
	return nil
}

func rankTier(tier string) int {
	switch tier {
	case "primary":
		return 0
	case "secondary":
		return 1
	case "fallback":
		return 2
	case "quarantine":
		return 3
	default:
		return 4
	}
}

func reliabilityTier(score float64) string {
	switch {
	case score >= 0.85:
		return "primary"
	case score >= 0.70:
		return "secondary"
	case score >= 0.50:
		return "fallback"
	default:
		return "quarantine"
	}
}
