package handlers

import (
	"context"
	"fmt"
	"sync"

	"metadata-service/internal/model"
)

// JobHandler processes a specific enrichment job type.
type JobHandler interface {
	Type() string
	Handle(ctx context.Context, job model.EnrichmentJob) error
}

// Registry stores handlers by job type.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]JobHandler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]JobHandler)}
}

func (r *Registry) Register(handler JobHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[handler.Type()] = handler
}

func (r *Registry) Handle(ctx context.Context, job model.EnrichmentJob) error {
	r.mu.RLock()
	handler, ok := r.handlers[job.JobType]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no enrichment handler registered for job type %q", job.JobType)
	}
	return handler.Handle(ctx, job)
}
