package handlers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"metadata-service/internal/model"
	"metadata-service/internal/provider"
	"metadata-service/internal/store"
)

type fakeProviderForAuthor struct {
	name          string
	searchResults []model.Work
	calls         *[]string
	mu            *sync.Mutex
}

func (p *fakeProviderForAuthor) Name() string { return p.name }
func (p *fakeProviderForAuthor) SearchWorks(_ context.Context, _ string) ([]model.Work, error) {
	if p.calls != nil && p.mu != nil {
		p.mu.Lock()
		*p.calls = append(*p.calls, p.name)
		p.mu.Unlock()
	}
	return p.searchResults, nil
}
func (p *fakeProviderForAuthor) GetWork(_ context.Context, _ string) (*model.Work, error) {
	return nil, nil
}
func (p *fakeProviderForAuthor) GetEditions(_ context.Context, _ string) ([]model.Edition, error) {
	return nil, nil
}
func (p *fakeProviderForAuthor) ResolveIdentifier(_ context.Context, _ string, _ string) (*model.Edition, error) {
	return nil, nil
}

type fakeWorkStoreForAuthor struct {
	inserted []model.Work
}

func (s *fakeWorkStoreForAuthor) GetWorkByID(_ context.Context, _ string) (*model.Work, error) {
	return nil, errors.New("not used")
}
func (s *fakeWorkStoreForAuthor) SearchWorks(_ context.Context, _ string) ([]model.Work, error) {
	return nil, nil
}
func (s *fakeWorkStoreForAuthor) InsertWork(_ context.Context, work model.Work) error {
	s.inserted = append(s.inserted, work)
	return nil
}
func (s *fakeWorkStoreForAuthor) UpdateWork(_ context.Context, _ model.Work) error { return nil }
func (s *fakeWorkStoreForAuthor) GetWorkByFingerprint(_ context.Context, _ string) (*model.Work, error) {
	return nil, nil
}

type fakeAuthorStoreForAuthor struct {
	insertedAuthors []model.Author
	links           [][2]string
}

func (s *fakeAuthorStoreForAuthor) InsertAuthor(_ context.Context, author model.Author) error {
	s.insertedAuthors = append(s.insertedAuthors, author)
	return nil
}
func (s *fakeAuthorStoreForAuthor) GetAuthorByName(_ context.Context, _ string) (*model.Author, error) {
	return nil, nil
}
func (s *fakeAuthorStoreForAuthor) LinkWorkAuthor(_ context.Context, workID string, authorID string) error {
	s.links = append(s.links, [2]string{workID, authorID})
	return nil
}

type fakeMappingStoreForAuthor struct {
	inserted int
}

func (s *fakeMappingStoreForAuthor) GetCanonicalID(_ context.Context, _ string, _ string) (string, error) {
	return "", errors.New("not found")
}
func (s *fakeMappingStoreForAuthor) InsertMapping(_ context.Context, _ string, _ string, _ string, _ string) error {
	s.inserted++
	return nil
}

type fakeEnrichmentJobStoreForAuthor struct {
	enqueued []model.EnrichmentJob
}

func (s *fakeEnrichmentJobStoreForAuthor) EnqueueJob(_ context.Context, job model.EnrichmentJob) (int64, error) {
	s.enqueued = append(s.enqueued, job)
	return int64(len(s.enqueued)), nil
}
func (s *fakeEnrichmentJobStoreForAuthor) GetJobByID(_ context.Context, _ int64) (*model.EnrichmentJob, error) {
	return nil, errors.New("not used")
}
func (s *fakeEnrichmentJobStoreForAuthor) TryLockNextJob(_ context.Context, _ string) (*model.EnrichmentJob, error) {
	return nil, store.ErrNoAvailableEnrichmentJobs
}
func (s *fakeEnrichmentJobStoreForAuthor) MarkSucceeded(_ context.Context, _ int64) error { return nil }
func (s *fakeEnrichmentJobStoreForAuthor) MarkFailed(_ context.Context, _ int64, _ string, _ string, _ time.Duration) error {
	return nil
}
func (s *fakeEnrichmentJobStoreForAuthor) MarkDead(_ context.Context, _ int64, _ string) error {
	return nil
}
func (s *fakeEnrichmentJobStoreForAuthor) RecordRunStart(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
func (s *fakeEnrichmentJobStoreForAuthor) RecordRunFinish(_ context.Context, _ int64, _ string, _ string) error {
	return nil
}
func (s *fakeEnrichmentJobStoreForAuthor) ListJobs(_ context.Context, _ model.EnrichmentJobFilters) ([]model.EnrichmentJob, error) {
	return nil, nil
}
func (s *fakeEnrichmentJobStoreForAuthor) CountJobsByStatus(_ context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}
func (s *fakeEnrichmentJobStoreForAuthor) NextRunnableAt(_ context.Context) (*time.Time, error) {
	return nil, nil
}

func TestAuthorExpandHandler_RespectsLimitsAndEnqueueCap(t *testing.T) {
	primary := &fakeProviderForAuthor{
		name: "primary",
		searchResults: []model.Work{
			{ID: "w1", Title: "Book 1", Confidence: 0.95, Authors: []model.Author{{ID: "a1", Name: "Ursula K. Le Guin"}}},
			{ID: "w2", Title: "Book 2", Confidence: 0.85, Authors: []model.Author{{ID: "a1", Name: "Ursula K. Le Guin"}}},
			{ID: "w3", Title: "Book 3", Confidence: 0.75, Authors: []model.Author{{ID: "a1", Name: "Ursula K. Le Guin"}}},
		},
	}
	registry := provider.NewRegistry()
	registry.RegisterWithConfig(primary, 10, true)

	works := &fakeWorkStoreForAuthor{}
	authors := &fakeAuthorStoreForAuthor{}
	mappings := &fakeMappingStoreForAuthor{}
	jobs := &fakeEnrichmentJobStoreForAuthor{}

	h := NewAuthorExpandHandler(registry, nil, works, authors, mappings, jobs, 2, 1)
	err := h.Handle(context.Background(), model.EnrichmentJob{
		JobType:    model.EnrichmentJobTypeAuthorExpand,
		EntityType: "author",
		EntityID:   "Ursula K. Le Guin",
	})
	if err != nil {
		t.Fatalf("handle author_expand: %v", err)
	}

	if len(works.inserted) != 2 {
		t.Fatalf("expected max_author_works to cap inserts at 2, got %d", len(works.inserted))
	}
	if len(jobs.enqueued) != 1 {
		t.Fatalf("expected max_jobs_per_expand to cap enqueues at 1, got %d", len(jobs.enqueued))
	}
	if jobs.enqueued[0].JobType != model.EnrichmentJobTypeWorkEditions || jobs.enqueued[0].EntityType != "work" {
		t.Fatalf("expected follow-up work_editions enqueue, got %+v", jobs.enqueued[0])
	}
}

func TestAuthorExpandHandler_RespectsQuarantineDisabledPolicy(t *testing.T) {
	var calls []string
	var callsMu sync.Mutex

	primary := &fakeProviderForAuthor{
		name: "primary",
		searchResults: []model.Work{
			{ID: "w-primary", Title: "Primary Book", Confidence: 0.8, Authors: []model.Author{{Name: "Isaac Asimov"}}},
		},
		calls: &calls,
		mu:    &callsMu,
	}
	quarantine := &fakeProviderForAuthor{
		name: "quarantine",
		searchResults: []model.Work{
			{ID: "w-quarantine", Title: "Quarantine Book", Confidence: 0.7, Authors: []model.Author{{Name: "Isaac Asimov"}}},
		},
		calls: &calls,
		mu:    &callsMu,
	}

	registry := provider.NewRegistry()
	registry.RegisterWithConfig(primary, 10, true)
	registry.RegisterWithConfig(quarantine, 1, true)
	registry.SetReliability("primary", 0.90)
	registry.SetReliability("quarantine", 0.20)
	registry.SetQuarantineDisables(true)

	works := &fakeWorkStoreForAuthor{}
	authors := &fakeAuthorStoreForAuthor{}
	mappings := &fakeMappingStoreForAuthor{}
	jobs := &fakeEnrichmentJobStoreForAuthor{}

	h := NewAuthorExpandHandler(registry, nil, works, authors, mappings, jobs, 10, 10)
	err := h.Handle(context.Background(), model.EnrichmentJob{
		JobType:    model.EnrichmentJobTypeAuthorExpand,
		EntityType: "author",
		EntityID:   "Isaac Asimov",
	})
	if err != nil {
		t.Fatalf("handle author_expand: %v", err)
	}

	if len(calls) != 1 || calls[0] != "primary" {
		t.Fatalf("expected only primary provider queried when quarantine disabled, calls=%v", calls)
	}
	if len(works.inserted) != 1 || works.inserted[0].ID != "w-primary" {
		t.Fatalf("expected only primary work inserted, got %+v", works.inserted)
	}
}
