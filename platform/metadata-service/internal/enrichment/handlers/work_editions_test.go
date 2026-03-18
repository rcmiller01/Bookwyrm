package handlers

import (
	"context"
	"sync"
	"testing"
	"time"

	"metadata-service/internal/model"
	"metadata-service/internal/provider"
	"metadata-service/internal/store"
)

type fakeProviderForEditions struct {
	name          string
	searchResults []model.Work
	editions      map[string][]model.Edition
	calls         *[]string
	mu            *sync.Mutex
}

func (p *fakeProviderForEditions) Name() string { return p.name }

func (p *fakeProviderForEditions) SearchWorks(_ context.Context, _ string) ([]model.Work, error) {
	if p.calls != nil && p.mu != nil {
		p.mu.Lock()
		*p.calls = append(*p.calls, p.name)
		p.mu.Unlock()
	}
	return p.searchResults, nil
}

func (p *fakeProviderForEditions) GetWork(_ context.Context, _ string) (*model.Work, error) {
	return nil, nil
}

func (p *fakeProviderForEditions) GetEditions(_ context.Context, providerWorkID string) ([]model.Edition, error) {
	return p.editions[providerWorkID], nil
}

func (p *fakeProviderForEditions) ResolveIdentifier(_ context.Context, _ string, _ string) (*model.Edition, error) {
	return nil, nil
}

type fakeWorkStoreForEditions struct {
	work model.Work
}

func (s *fakeWorkStoreForEditions) GetWorkByID(_ context.Context, _ string) (*model.Work, error) {
	cp := s.work
	return &cp, nil
}
func (s *fakeWorkStoreForEditions) SearchWorks(_ context.Context, _ string) ([]model.Work, error) {
	return nil, nil
}
func (s *fakeWorkStoreForEditions) InsertWork(_ context.Context, _ model.Work) error { return nil }
func (s *fakeWorkStoreForEditions) UpdateWork(_ context.Context, _ model.Work) error { return nil }
func (s *fakeWorkStoreForEditions) GetWorkByFingerprint(_ context.Context, _ string) (*model.Work, error) {
	return nil, nil
}

type fakeEditionStoreForEditions struct {
	inserted []model.Edition
}

func (s *fakeEditionStoreForEditions) InsertEdition(_ context.Context, edition model.Edition) error {
	s.inserted = append(s.inserted, edition)
	return nil
}
func (s *fakeEditionStoreForEditions) GetEditionsByWork(_ context.Context, _ string) ([]model.Edition, error) {
	return nil, nil
}
func (s *fakeEditionStoreForEditions) GetEditionByID(_ context.Context, _ string) (*model.Edition, error) {
	return nil, nil
}

type fakeIdentifierStoreForEditions struct {
	inserted []model.Identifier
}

func (s *fakeIdentifierStoreForEditions) InsertIdentifier(_ context.Context, _ string, id model.Identifier) error {
	s.inserted = append(s.inserted, id)
	return nil
}
func (s *fakeIdentifierStoreForEditions) FindEditionByIdentifier(_ context.Context, _ string, _ string) (*model.Edition, error) {
	return nil, nil
}
func (s *fakeIdentifierStoreForEditions) GetIdentifiersByEdition(_ context.Context, _ string) ([]model.Identifier, error) {
	return nil, nil
}

type fakeProviderMetricsStoreForEditions struct {
	introduced map[string]int
}

func (s *fakeProviderMetricsStoreForEditions) RecordSuccess(_ context.Context, _ string, _ time.Duration) error {
	return nil
}
func (s *fakeProviderMetricsStoreForEditions) RecordFailure(_ context.Context, _ string) error {
	return nil
}
func (s *fakeProviderMetricsStoreForEditions) RecordIdentifierMatch(_ context.Context, _ string) error {
	return nil
}
func (s *fakeProviderMetricsStoreForEditions) RecordIdentifierIntroduced(_ context.Context, providerName string) error {
	if s.introduced == nil {
		s.introduced = map[string]int{}
	}
	s.introduced[providerName]++
	return nil
}
func (s *fakeProviderMetricsStoreForEditions) GetMetrics(_ context.Context, _ string) (*store.ProviderMetrics, error) {
	return nil, nil
}
func (s *fakeProviderMetricsStoreForEditions) GetAllMetrics(_ context.Context) ([]store.ProviderMetrics, error) {
	return nil, nil
}

func TestWorkEditionsHandler_RespectsProviderOrderAndMaxLimit(t *testing.T) {
	var calls []string
	var callsMu sync.Mutex

	primary := &fakeProviderForEditions{
		name:          "primary",
		searchResults: []model.Work{{ID: "p1", Title: "The Hobbit"}},
		editions: map[string][]model.Edition{
			"p1": {
				{ID: "ed-primary-1", Title: "The Hobbit A", Identifiers: []model.Identifier{{Type: "ISBN_13", Value: "111"}}},
				{ID: "ed-primary-2", Title: "The Hobbit B", Identifiers: []model.Identifier{{Type: "ISBN_13", Value: "222"}}},
			},
		},
		calls: &calls,
		mu:    &callsMu,
	}
	secondary := &fakeProviderForEditions{
		name:          "secondary",
		searchResults: []model.Work{{ID: "s1", Title: "The Hobbit"}},
		editions: map[string][]model.Edition{
			"s1": {{ID: "ed-secondary-1", Title: "The Hobbit C"}},
		},
		calls: &calls,
		mu:    &callsMu,
	}

	registry := provider.NewRegistry()
	registry.RegisterWithConfig(secondary, 20, true)
	registry.RegisterWithConfig(primary, 10, true)

	workStore := &fakeWorkStoreForEditions{work: model.Work{ID: "w1", Title: "The Hobbit"}}
	editionStore := &fakeEditionStoreForEditions{}
	identifierStore := &fakeIdentifierStoreForEditions{}
	metricsStore := &fakeProviderMetricsStoreForEditions{}

	h := NewWorkEditionsHandler(registry, nil, workStore, editionStore, identifierStore, metricsStore, 2)

	err := h.Handle(context.Background(), model.EnrichmentJob{
		JobType:    model.EnrichmentJobTypeWorkEditions,
		EntityType: "work",
		EntityID:   "w1",
	})
	if err != nil {
		t.Fatalf("handle work_editions: %v", err)
	}

	if len(calls) != 1 || calls[0] != "primary" {
		t.Fatalf("expected only primary provider to be used due to max limit, got calls=%v", calls)
	}
	if len(editionStore.inserted) != 2 {
		t.Fatalf("expected exactly 2 inserted editions, got %d", len(editionStore.inserted))
	}
	if len(identifierStore.inserted) != 2 {
		t.Fatalf("expected 2 inserted identifiers, got %d", len(identifierStore.inserted))
	}
	if metricsStore.introduced["primary"] != 2 {
		t.Fatalf("expected 2 identifier-introduced metrics for primary, got %d", metricsStore.introduced["primary"])
	}
}

func TestWorkEditionsHandler_RespectsQuarantineDisabledPolicy(t *testing.T) {
	var calls []string
	var callsMu sync.Mutex

	primary := &fakeProviderForEditions{
		name:          "primary",
		searchResults: []model.Work{{ID: "p1", Title: "Dune"}},
		editions: map[string][]model.Edition{
			"p1": {{ID: "ed-primary", Title: "Dune Primary"}},
		},
		calls: &calls,
		mu:    &callsMu,
	}
	quarantine := &fakeProviderForEditions{
		name:          "quarantine",
		searchResults: []model.Work{{ID: "q1", Title: "Dune"}},
		editions: map[string][]model.Edition{
			"q1": {{ID: "ed-quarantine", Title: "Dune Quarantine"}},
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

	workStore := &fakeWorkStoreForEditions{work: model.Work{ID: "w2", Title: "Dune"}}
	editionStore := &fakeEditionStoreForEditions{}
	identifierStore := &fakeIdentifierStoreForEditions{}
	metricsStore := &fakeProviderMetricsStoreForEditions{}

	h := NewWorkEditionsHandler(registry, nil, workStore, editionStore, identifierStore, metricsStore, 10)

	err := h.Handle(context.Background(), model.EnrichmentJob{
		JobType:    model.EnrichmentJobTypeWorkEditions,
		EntityType: "work",
		EntityID:   "w2",
	})
	if err != nil {
		t.Fatalf("handle work_editions: %v", err)
	}

	if len(calls) != 1 || calls[0] != "primary" {
		t.Fatalf("expected only non-quarantine provider to be used when quarantine disabled, calls=%v", calls)
	}
	if len(editionStore.inserted) != 1 || editionStore.inserted[0].ID != "ed-primary" {
		t.Fatalf("expected only primary edition inserted, got %+v", editionStore.inserted)
	}
}
