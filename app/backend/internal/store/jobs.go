package store

import (
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"app-backend/internal/domain"
)

var (
	ErrJobNotFound    = errors.New("job not found")
	ErrJobNotRunnable = errors.New("job not runnable")
)

type JobStore interface {
	Enqueue(jobType domain.JobType, payload map[string]any, runAt time.Time, maxAttempts int) domain.Job
	List(filter domain.JobFilter) []domain.Job
	GetByID(id string) (domain.Job, error)
	ClaimRunnable(now time.Time) (domain.Job, bool)
	MarkSucceeded(id string, output map[string]any) error
	MarkRetryableFailure(id string, errMsg string, nextRunAt time.Time) (domain.Job, error)
	MarkDeadLetter(id string, errMsg string) (domain.Job, error)
	Retry(id string) (domain.Job, error)
	Cancel(id string) (domain.Job, error)
}

type inMemoryJobStore struct {
	mu      sync.RWMutex
	nextID  int64
	ordered []string
	jobs    map[string]domain.Job
}

func NewInMemoryJobStore() JobStore {
	return &inMemoryJobStore{
		nextID:  1,
		jobs:    map[string]domain.Job{},
		ordered: []string{},
	}
}

func (s *inMemoryJobStore) Enqueue(jobType domain.JobType, payload map[string]any, runAt time.Time, maxAttempts int) domain.Job {
	s.mu.Lock()
	defer s.mu.Unlock()

	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if runAt.IsZero() {
		runAt = time.Now().UTC()
	}
	now := time.Now().UTC()
	id := "job-" + strconv.FormatInt(s.nextID, 10)
	s.nextID++

	job := domain.Job{
		ID:          id,
		Type:        jobType,
		State:       domain.JobStateQueued,
		Payload:     clonePayload(payload),
		Attempt:     0,
		MaxAttempts: maxAttempts,
		RunAt:       runAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.jobs[id] = job
	s.ordered = append(s.ordered, id)
	return job
}

func (s *inMemoryJobStore) List(filter domain.JobFilter) []domain.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]domain.Job, 0, limit)
	for _, id := range s.ordered {
		job := s.jobs[id]
		if filter.Type != "" && job.Type != filter.Type {
			continue
		}
		if filter.State != "" && job.State != filter.State {
			continue
		}
		out = append(out, job)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *inMemoryJobStore) GetByID(id string) (domain.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrJobNotFound
	}
	return job, nil
}

func (s *inMemoryJobStore) ClaimRunnable(now time.Time) (domain.Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	candidates := make([]domain.Job, 0)
	for _, id := range s.ordered {
		job := s.jobs[id]
		if (job.State == domain.JobStateQueued || job.State == domain.JobStateRetryableFail) && !job.RunAt.After(now) {
			candidates = append(candidates, job)
		}
	}
	if len(candidates) == 0 {
		return domain.Job{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].RunAt.Equal(candidates[j].RunAt) {
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		}
		return candidates[i].RunAt.Before(candidates[j].RunAt)
	})
	job := candidates[0]
	locked := now.UTC()
	job.State = domain.JobStateRunning
	job.LockedAt = &locked
	job.Attempt++
	job.UpdatedAt = locked
	s.jobs[job.ID] = job
	return job, true
}

func (s *inMemoryJobStore) MarkSucceeded(id string, output map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return ErrJobNotFound
	}
	now := time.Now().UTC()
	job.State = domain.JobStateSucceeded
	job.LastError = ""
	job.Output = clonePayload(output)
	job.LockedAt = nil
	job.UpdatedAt = now
	s.jobs[id] = job
	return nil
}

func (s *inMemoryJobStore) MarkRetryableFailure(id string, errMsg string, nextRunAt time.Time) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrJobNotFound
	}
	now := time.Now().UTC()
	job.State = domain.JobStateRetryableFail
	job.LastError = errMsg
	job.LockedAt = nil
	job.RunAt = nextRunAt.UTC()
	job.UpdatedAt = now
	s.jobs[id] = job
	return job, nil
}

func (s *inMemoryJobStore) MarkDeadLetter(id string, errMsg string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrJobNotFound
	}
	now := time.Now().UTC()
	job.State = domain.JobStateDeadLetter
	job.LastError = errMsg
	job.LockedAt = nil
	job.UpdatedAt = now
	s.jobs[id] = job
	return job, nil
}

func (s *inMemoryJobStore) Retry(id string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrJobNotFound
	}
	switch job.State {
	case domain.JobStateRetryableFail, domain.JobStateDeadLetter, domain.JobStateCanceled:
	default:
		return domain.Job{}, ErrJobNotRunnable
	}
	now := time.Now().UTC()
	job.State = domain.JobStateQueued
	job.LastError = ""
	job.LockedAt = nil
	job.RunAt = now
	job.UpdatedAt = now
	s.jobs[id] = job
	return job, nil
}

func (s *inMemoryJobStore) Cancel(id string) (domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return domain.Job{}, ErrJobNotFound
	}
	switch job.State {
	case domain.JobStateSucceeded, domain.JobStateDeadLetter:
		return domain.Job{}, ErrJobNotRunnable
	}
	now := time.Now().UTC()
	job.State = domain.JobStateCanceled
	job.LockedAt = nil
	job.UpdatedAt = now
	s.jobs[id] = job
	return job, nil
}

func clonePayload(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}
