package enrichment

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"metadata-service/internal/enrichment/handlers"
	"metadata-service/internal/model"
	"metadata-service/internal/store"
)

type workerTestHandler struct {
	jobType string
	err     error
}

func (h *workerTestHandler) Type() string { return h.jobType }
func (h *workerTestHandler) Handle(_ context.Context, _ model.EnrichmentJob) error {
	return h.err
}

type workerTestStore struct {
	mu sync.Mutex

	job         *model.EnrichmentJob
	providedJob bool

	runStartErr error

	runID         int64
	runFinished   int
	finishOutcome string
	finishError   string

	markSucceededCount int
	markFailedCount    int
	lastFailedBackoff  time.Duration
	lastFailedType     string
	lastFailedErr      string

	doneOnce sync.Once
	done     chan struct{}
}

func newWorkerTestStore(job model.EnrichmentJob) *workerTestStore {
	return &workerTestStore{
		job:  &job,
		done: make(chan struct{}),
	}
}

func (s *workerTestStore) signalDone() {
	s.doneOnce.Do(func() { close(s.done) })
}

func (s *workerTestStore) EnqueueJob(_ context.Context, _ model.EnrichmentJob) (int64, error) {
	return 0, nil
}

func (s *workerTestStore) GetJobByID(_ context.Context, id int64) (*model.EnrichmentJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job == nil || s.job.ID != id {
		return nil, errors.New("not found")
	}
	cp := *s.job
	return &cp, nil
}

func (s *workerTestStore) TryLockNextJob(_ context.Context, workerID string) (*model.EnrichmentJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.providedJob || s.job == nil || s.job.Status != model.EnrichmentStatusQueued {
		return nil, store.ErrNoAvailableEnrichmentJobs
	}
	s.providedJob = true
	now := time.Now().UTC()
	s.job.Status = model.EnrichmentStatusRunning
	s.job.LockedAt = &now
	s.job.LockedBy = &workerID
	cp := *s.job
	return &cp, nil
}

func (s *workerTestStore) MarkSucceeded(_ context.Context, jobID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job != nil && s.job.ID == jobID {
		s.markSucceededCount++
		s.job.Status = model.EnrichmentStatusSucceeded
		s.job.LockedAt = nil
		s.job.LockedBy = nil
	}
	s.signalDone()
	return nil
}

func (s *workerTestStore) MarkFailed(_ context.Context, jobID int64, jobType string, errMsg string, backoff time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.job != nil && s.job.ID == jobID {
		s.markFailedCount++
		s.lastFailedType = jobType
		s.lastFailedErr = errMsg
		s.lastFailedBackoff = backoff
		s.job.AttemptCount++
		s.job.LockedAt = nil
		s.job.LockedBy = nil
		s.job.LastError = &errMsg
		if s.job.AttemptCount >= s.job.MaxAttempts {
			s.job.Status = model.EnrichmentStatusDead
		} else {
			s.job.Status = model.EnrichmentStatusQueued
		}
	}
	s.signalDone()
	return nil
}

func (s *workerTestStore) MarkDead(_ context.Context, _ int64, _ string) error { return nil }

func (s *workerTestStore) RecordRunStart(_ context.Context, _ int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runStartErr != nil {
		return 0, s.runStartErr
	}
	s.runID++
	return s.runID, nil
}

func (s *workerTestStore) RecordRunFinish(_ context.Context, _ int64, outcome string, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runFinished++
	s.finishOutcome = outcome
	s.finishError = errMsg
	return nil
}

func (s *workerTestStore) ListJobs(_ context.Context, _ model.EnrichmentJobFilters) ([]model.EnrichmentJob, error) {
	return nil, nil
}

func (s *workerTestStore) CountJobsByStatus(_ context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func (s *workerTestStore) NextRunnableAt(_ context.Context) (*time.Time, error) {
	return nil, nil
}

func waitForSignal(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for worker to process job")
	}
}

func TestWorker_SuccessPathMarksSucceeded(t *testing.T) {
	job := model.EnrichmentJob{
		ID:          101,
		JobType:     model.EnrichmentJobTypeWorkEditions,
		Status:      model.EnrichmentStatusQueued,
		MaxAttempts: 3,
	}
	jobStore := newWorkerTestStore(job)

	registry := handlers.NewRegistry()
	registry.Register(&workerTestHandler{jobType: model.EnrichmentJobTypeWorkEditions})

	worker := NewWorker("worker-success", jobStore, registry)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx)

	waitForSignal(t, jobStore.done)
	cancel()

	jobAfter, err := jobStore.GetJobByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get job by id: %v", err)
	}
	if jobStore.markSucceededCount != 1 {
		t.Fatalf("expected MarkSucceeded called once, got %d", jobStore.markSucceededCount)
	}
	if jobStore.markFailedCount != 0 {
		t.Fatalf("expected MarkFailed not called, got %d", jobStore.markFailedCount)
	}
	if jobStore.finishOutcome != model.EnrichmentOutcomeSucceeded {
		t.Fatalf("expected run outcome %q, got %q", model.EnrichmentOutcomeSucceeded, jobStore.finishOutcome)
	}
	if jobAfter.Status != model.EnrichmentStatusSucceeded {
		t.Fatalf("expected status %q, got %q", model.EnrichmentStatusSucceeded, jobAfter.Status)
	}
}

func TestWorker_FailurePathSchedulesRetry(t *testing.T) {
	job := model.EnrichmentJob{
		ID:          102,
		JobType:     model.EnrichmentJobTypeAuthorExpand,
		Status:      model.EnrichmentStatusQueued,
		MaxAttempts: 3,
	}
	jobStore := newWorkerTestStore(job)

	registry := handlers.NewRegistry()
	registry.Register(&workerTestHandler{jobType: model.EnrichmentJobTypeAuthorExpand, err: errors.New("handler failed")})

	worker := NewWorker("worker-retry", jobStore, registry)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx)

	waitForSignal(t, jobStore.done)
	cancel()

	jobAfter, err := jobStore.GetJobByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get job by id: %v", err)
	}
	if jobStore.markFailedCount != 1 {
		t.Fatalf("expected MarkFailed called once, got %d", jobStore.markFailedCount)
	}
	if jobStore.markSucceededCount != 0 {
		t.Fatalf("expected MarkSucceeded not called, got %d", jobStore.markSucceededCount)
	}
	if jobStore.finishOutcome != model.EnrichmentOutcomeFailed {
		t.Fatalf("expected run outcome %q, got %q", model.EnrichmentOutcomeFailed, jobStore.finishOutcome)
	}
	if jobStore.lastFailedType != model.EnrichmentJobTypeAuthorExpand {
		t.Fatalf("expected failed job type %q, got %q", model.EnrichmentJobTypeAuthorExpand, jobStore.lastFailedType)
	}
	if jobStore.lastFailedBackoff != time.Second {
		t.Fatalf("expected backoff %v, got %v", time.Second, jobStore.lastFailedBackoff)
	}
	if jobAfter.AttemptCount != 1 {
		t.Fatalf("expected attempt_count 1, got %d", jobAfter.AttemptCount)
	}
	if jobAfter.Status != model.EnrichmentStatusQueued {
		t.Fatalf("expected status %q after retry scheduling, got %q", model.EnrichmentStatusQueued, jobAfter.Status)
	}
}

func TestWorker_FailurePathMarksDeadAtMaxAttempts(t *testing.T) {
	job := model.EnrichmentJob{
		ID:           103,
		JobType:      model.EnrichmentJobTypeAuthorExpand,
		Status:       model.EnrichmentStatusQueued,
		AttemptCount: 0,
		MaxAttempts:  1,
	}
	jobStore := newWorkerTestStore(job)

	registry := handlers.NewRegistry()
	registry.Register(&workerTestHandler{jobType: model.EnrichmentJobTypeAuthorExpand, err: errors.New("boom")})

	worker := NewWorker("worker-dead", jobStore, registry)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker.Run(ctx)

	waitForSignal(t, jobStore.done)
	cancel()

	jobAfter, err := jobStore.GetJobByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get job by id: %v", err)
	}
	if jobStore.markFailedCount != 1 {
		t.Fatalf("expected MarkFailed called once, got %d", jobStore.markFailedCount)
	}
	if jobAfter.Status != model.EnrichmentStatusDead {
		t.Fatalf("expected status %q at max attempts, got %q", model.EnrichmentStatusDead, jobAfter.Status)
	}
}
