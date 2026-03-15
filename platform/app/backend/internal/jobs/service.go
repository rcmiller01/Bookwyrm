package jobs

import (
	"context"
	"time"

	"app-backend/internal/domain"
	"app-backend/internal/store"
)

type Service struct {
	store  store.JobStore
	engine *Engine
}

func NewService(jobStore store.JobStore, opts Options, handlers ...Handler) *Service {
	return &Service{
		store:  jobStore,
		engine: NewEngine(jobStore, opts, handlers...),
	}
}

func (s *Service) Start(ctx context.Context) {
	s.engine.Start(ctx)
}

func (s *Service) Enqueue(jobType domain.JobType, payload map[string]any, runAt time.Time, maxAttempts int) domain.Job {
	return s.store.Enqueue(jobType, payload, runAt, maxAttempts)
}

func (s *Service) List(filter domain.JobFilter) []domain.Job {
	return s.store.List(filter)
}

func (s *Service) Get(id string) (domain.Job, error) {
	return s.store.GetByID(id)
}

func (s *Service) Retry(id string) (domain.Job, error) {
	return s.store.Retry(id)
}

func (s *Service) Cancel(id string) (domain.Job, error) {
	return s.store.Cancel(id)
}
