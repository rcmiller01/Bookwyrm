package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"app-backend/internal/domain"
	"app-backend/internal/store"
)

type failingHandler struct {
	jobType domain.JobType
	failFor int
	seen    int
}

func (h *failingHandler) Type() domain.JobType { return h.jobType }
func (h *failingHandler) Handle(_ context.Context, _ domain.Job) (map[string]any, error) {
	h.seen++
	if h.seen <= h.failFor {
		return nil, errors.New("temporary failure")
	}
	return map[string]any{"ok": true}, nil
}

func TestEngine_RetryThenSuccess(t *testing.T) {
	jobStore := store.NewInMemoryJobStore()
	handler := &failingHandler{jobType: domain.JobTypeSearchMissing, failFor: 1}
	engine := NewEngine(jobStore, Options{WorkerCount: 1, PollInterval: 10 * time.Millisecond}, handler)

	job := jobStore.Enqueue(domain.JobTypeSearchMissing, map[string]any{}, time.Now().UTC(), 3)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	go engine.Start(ctx)

	deadline := time.Now().Add(350 * time.Millisecond)
	retried := false
	for time.Now().Before(deadline) {
		current, err := jobStore.GetByID(job.ID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if current.State == domain.JobStateSucceeded {
			return
		}
		if current.State == domain.JobStateRetryableFail && !retried {
			if _, err := jobStore.Retry(job.ID); err != nil {
				t.Fatalf("retry: %v", err)
			}
			retried = true
		}
		time.Sleep(15 * time.Millisecond)
	}
	final, _ := jobStore.GetByID(job.ID)
	t.Fatalf("expected job succeeded, got state=%s attempt=%d error=%s", final.State, final.Attempt, final.LastError)
}
