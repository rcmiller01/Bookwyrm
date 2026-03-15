package provider_test

import (
	"context"
	"testing"
	"time"

	"metadata-service/internal/provider"
	"metadata-service/internal/store"
)

// --- in-memory ProviderMetricsStore for testing ---

type memMetricsStore struct {
	data map[string]*store.ProviderMetrics
}

func newMemMetricsStore() *memMetricsStore {
	return &memMetricsStore{data: make(map[string]*store.ProviderMetrics)}
}

func (s *memMetricsStore) ensure(name string) *store.ProviderMetrics {
	if _, ok := s.data[name]; !ok {
		s.data[name] = &store.ProviderMetrics{Provider: name}
	}
	return s.data[name]
}

func (s *memMetricsStore) RecordSuccess(_ context.Context, name string, latency time.Duration) error {
	m := s.ensure(name)
	m.SuccessCount++
	m.RequestCount++
	m.TotalLatencyMs += latency.Milliseconds()
	now := time.Now()
	m.LastSuccess = &now
	return nil
}

func (s *memMetricsStore) RecordFailure(_ context.Context, name string) error {
	m := s.ensure(name)
	m.FailureCount++
	m.RequestCount++
	now := time.Now()
	m.LastFailure = &now
	return nil
}

func (s *memMetricsStore) RecordIdentifierMatch(_ context.Context, name string) error {
	s.ensure(name).IdentifierMatches++
	return nil
}

func (s *memMetricsStore) RecordIdentifierIntroduced(_ context.Context, name string) error {
	s.ensure(name).IdentifierIntroduced++
	return nil
}

func (s *memMetricsStore) GetMetrics(_ context.Context, name string) (*store.ProviderMetrics, error) {
	if m, ok := s.data[name]; ok {
		cp := *m
		return &cp, nil
	}
	return &store.ProviderMetrics{Provider: name}, nil
}

func (s *memMetricsStore) GetAllMetrics(_ context.Context) ([]store.ProviderMetrics, error) {
	var out []store.ProviderMetrics
	for _, m := range s.data {
		out = append(out, *m)
	}
	return out, nil
}

// --- tests ---

func TestMetricsStore_RecordSuccess(t *testing.T) {
	ms := newMemMetricsStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := ms.RecordSuccess(ctx, "openlibrary", 500*time.Millisecond); err != nil {
			t.Fatalf("RecordSuccess: %v", err)
		}
	}

	m, err := ms.GetMetrics(ctx, "openlibrary")
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if m.SuccessCount != 5 {
		t.Errorf("success_count = %d, want 5", m.SuccessCount)
	}
	if m.RequestCount != 5 {
		t.Errorf("request_count = %d, want 5", m.RequestCount)
	}
	if m.TotalLatencyMs != 2500 {
		t.Errorf("total_latency_ms = %d, want 2500", m.TotalLatencyMs)
	}
}

func TestMetricsStore_RecordFailure(t *testing.T) {
	ms := newMemMetricsStore()
	ctx := context.Background()

	_ = ms.RecordSuccess(ctx, "googlebooks", 300*time.Millisecond)
	_ = ms.RecordFailure(ctx, "googlebooks")
	_ = ms.RecordFailure(ctx, "googlebooks")

	m, _ := ms.GetMetrics(ctx, "googlebooks")
	if m.SuccessCount != 1 {
		t.Errorf("success_count = %d, want 1", m.SuccessCount)
	}
	if m.FailureCount != 2 {
		t.Errorf("failure_count = %d, want 2", m.FailureCount)
	}
	if m.RequestCount != 3 {
		t.Errorf("request_count = %d, want 3", m.RequestCount)
	}
}

func TestMetricsStore_IdentifierCounts(t *testing.T) {
	ms := newMemMetricsStore()
	ctx := context.Background()

	_ = ms.RecordIdentifierIntroduced(ctx, "openlibrary")
	_ = ms.RecordIdentifierIntroduced(ctx, "openlibrary")
	_ = ms.RecordIdentifierMatch(ctx, "openlibrary")

	m, _ := ms.GetMetrics(ctx, "openlibrary")
	if m.IdentifierIntroduced != 2 {
		t.Errorf("identifier_introduced = %d, want 2", m.IdentifierIntroduced)
	}
	if m.IdentifierMatches != 1 {
		t.Errorf("identifier_matches = %d, want 1", m.IdentifierMatches)
	}
}

func TestReliabilityWorker_UpdatesScores(t *testing.T) {
	ms := newMemMetricsStore()
	ctx := context.Background()

	// seed metrics for two providers
	for i := 0; i < 10; i++ {
		_ = ms.RecordSuccess(ctx, "openlibrary", 400*time.Millisecond)
	}
	for i := 0; i < 5; i++ {
		_ = ms.RecordSuccess(ctx, "googlebooks", 1800*time.Millisecond)
		_ = ms.RecordFailure(ctx, "googlebooks")
	}

	rs := newMemReliabilityStore()
	registry := provider.NewRegistry()

	worker := provider.NewReliabilityWorker(ms, rs, registry, time.Hour)
	// manually call the internal run once
	worker.Start(mustCtxWithCancel(t))

	olScore, _ := rs.GetScore(ctx, "openlibrary")
	gbScore, _ := rs.GetScore(ctx, "googlebooks")

	if olScore == nil || gbScore == nil {
		t.Fatal("expected scores to be stored after worker run")
	}
	if olScore.CompositeScore <= gbScore.CompositeScore {
		t.Errorf("openlibrary (%f) should have higher score than googlebooks (%f)",
			olScore.CompositeScore, gbScore.CompositeScore)
	}
}

// --- in-memory ReliabilityStore for testing ---

type memReliabilityStore struct {
	data map[string]*store.ReliabilityScore
}

func newMemReliabilityStore() *memReliabilityStore {
	return &memReliabilityStore{data: make(map[string]*store.ReliabilityScore)}
}

func (s *memReliabilityStore) GetScore(_ context.Context, name string) (*store.ReliabilityScore, error) {
	if rs, ok := s.data[name]; ok {
		cp := *rs
		return &cp, nil
	}
	return nil, nil
}

func (s *memReliabilityStore) GetAllScores(_ context.Context) ([]store.ReliabilityScore, error) {
	var out []store.ReliabilityScore
	for _, rs := range s.data {
		out = append(out, *rs)
	}
	return out, nil
}

func (s *memReliabilityStore) UpdateScore(_ context.Context, score store.ReliabilityScore) error {
	cp := score
	s.data[score.Provider] = &cp
	return nil
}

func mustCtxWithCancel(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return ctx
}
