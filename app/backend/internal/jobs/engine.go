package jobs

import (
	"context"
	"errors"
	"sync"
	"time"

	"app-backend/internal/domain"
	"app-backend/internal/store"
)

type Handler interface {
	Type() domain.JobType
	Handle(ctx context.Context, job domain.Job) (map[string]any, error)
}

type Engine struct {
	store        store.JobStore
	handlers     map[domain.JobType]Handler
	workerCount  int
	pollInterval time.Duration
}

type Options struct {
	WorkerCount  int
	PollInterval time.Duration
}

func NewEngine(jobStore store.JobStore, opts Options, handlers ...Handler) *Engine {
	workerCount := opts.WorkerCount
	if workerCount <= 0 {
		workerCount = 2
	}
	poll := opts.PollInterval
	if poll <= 0 {
		poll = 500 * time.Millisecond
	}
	reg := map[domain.JobType]Handler{}
	for _, h := range handlers {
		reg[h.Type()] = h
	}
	return &Engine{
		store:        jobStore,
		handlers:     reg,
		workerCount:  workerCount,
		pollInterval: poll,
	}
}

func (e *Engine) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < e.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.workerLoop(ctx)
		}()
	}
	wg.Wait()
}

func (e *Engine) workerLoop(ctx context.Context) {
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.processOne(ctx)
		}
	}
}

func (e *Engine) processOne(ctx context.Context) {
	job, ok := e.store.ClaimRunnable(time.Now().UTC())
	if !ok {
		return
	}
	handler, exists := e.handlers[job.Type]
	if !exists {
		_, _ = e.store.MarkDeadLetter(job.ID, "no handler registered for job type")
		return
	}
	output, err := handler.Handle(ctx, job)
	if err != nil {
		if job.Attempt >= job.MaxAttempts {
			_, _ = e.store.MarkDeadLetter(job.ID, err.Error())
			return
		}
		_, _ = e.store.MarkRetryableFailure(job.ID, err.Error(), time.Now().UTC().Add(backoffForAttempt(job.Attempt)))
		return
	}
	_ = e.store.MarkSucceeded(job.ID, output)
}

func backoffForAttempt(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	d := time.Duration(1<<min(attempt, 7)) * time.Second
	max := 5 * time.Minute
	if d > max {
		return max
	}
	return d
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

var ErrInvalidPayload = errors.New("invalid job payload")
