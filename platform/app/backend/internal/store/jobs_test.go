package store

import (
	"testing"
	"time"

	"app-backend/internal/domain"
)

func TestInMemoryJobStore_ClaimAndRetry(t *testing.T) {
	s := NewInMemoryJobStore()
	job := s.Enqueue(domain.JobTypeSearchMissing, map[string]any{"x": 1}, time.Now().UTC(), 3)

	claimed, ok := s.ClaimRunnable(time.Now().UTC().Add(1 * time.Second))
	if !ok {
		t.Fatalf("expected claimed job")
	}
	if claimed.ID != job.ID || claimed.State != domain.JobStateRunning {
		t.Fatalf("unexpected claimed job state: %+v", claimed)
	}

	if _, err := s.MarkRetryableFailure(job.ID, "boom", time.Now().UTC()); err != nil {
		t.Fatalf("mark retryable failed: %v", err)
	}
	retried, err := s.Retry(job.ID)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if retried.State != domain.JobStateQueued {
		t.Fatalf("expected queued after retry, got %s", retried.State)
	}
}
