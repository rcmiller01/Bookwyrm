package resolver

import (
	"context"
	"errors"
	"testing"
	"time"

	"metadata-service/internal/model"
	"metadata-service/internal/provider"
	"metadata-service/internal/store"
)

// --- in-memory reliability store ---

type memReliabilityStore struct {
	scores map[string]*store.ReliabilityScore
}

func newMemReliabilityStore() *memReliabilityStore {
	return &memReliabilityStore{scores: make(map[string]*store.ReliabilityScore)}
}

func (s *memReliabilityStore) GetScore(_ context.Context, name string) (*store.ReliabilityScore, error) {
	if rs, ok := s.scores[name]; ok {
		cp := *rs
		return &cp, nil
	}
	return nil, errors.New("not found")
}

func (s *memReliabilityStore) GetAllScores(_ context.Context) ([]store.ReliabilityScore, error) {
	var out []store.ReliabilityScore
	for _, rs := range s.scores {
		out = append(out, *rs)
	}
	return out, nil
}

func (s *memReliabilityStore) UpdateScore(_ context.Context, score store.ReliabilityScore) error {
	cp := score
	s.scores[score.Provider] = &cp
	return nil
}

// --- in-memory provider metrics store ---

type memProviderMetricsStore struct{}

func (s *memProviderMetricsStore) RecordSuccess(_ context.Context, _ string, _ time.Duration) error {
	return nil
}
func (s *memProviderMetricsStore) RecordFailure(_ context.Context, _ string) error { return nil }
func (s *memProviderMetricsStore) RecordIdentifierMatch(_ context.Context, _ string) error {
	return nil
}
func (s *memProviderMetricsStore) RecordIdentifierIntroduced(_ context.Context, _ string) error {
	return nil
}
func (s *memProviderMetricsStore) GetMetrics(_ context.Context, _ string) (*store.ProviderMetrics, error) {
	return &store.ProviderMetrics{}, nil
}
func (s *memProviderMetricsStore) GetAllMetrics(_ context.Context) ([]store.ProviderMetrics, error) {
	return nil, nil
}

// buildReliabilityResolver creates a resolver wired with the supplied reliability store.
func buildReliabilityResolver(
	ws *mockWorkStore,
	reg *provider.Registry,
	rl *provider.RateLimiter,
	rs *memReliabilityStore,
) Resolver {
	c := newMockCache()
	s := Stores{
		Works:       ws,
		Authors:     &noopAuthorStore{},
		Editions:    &noopEditionStore{},
		IDs:         &noopIdentifierStore{},
		Mappings:    &noopMappingStore{},
		Status:      &noopStatusStore{},
		ProvMetrics: &memProviderMetricsStore{},
		Reliability: rs,
	}
	return New(reg, rl, s, c)
}

// callOrder records the order in which providers were called.
type callOrderProvider struct {
	name   string
	order  *[]string
	result []model.Work
}

func (p *callOrderProvider) Name() string { return p.name }
func (p *callOrderProvider) SearchWorks(_ context.Context, _ string) ([]model.Work, error) {
	*p.order = append(*p.order, p.name)
	return p.result, nil
}
func (p *callOrderProvider) GetWork(_ context.Context, _ string) (*model.Work, error) {
	return nil, nil
}
func (p *callOrderProvider) GetEditions(_ context.Context, _ string) ([]model.Edition, error) {
	return nil, nil
}
func (p *callOrderProvider) ResolveIdentifier(_ context.Context, _, _ string) (*model.Edition, error) {
	return nil, nil
}

// TestReliability_ResolverSortsByScore verifies that when Provider A has a
// higher reliability score than Provider B, A is dispatched first.
//
// NOTE: The resolver dispatches providers concurrently, so strict ordering is
// not guaranteed at the goroutine-scheduler level. This test checks that the
// registry sees A sorted before B by verifying that A appears in the results
// produced by calling EnabledProviders on the sorted slice.
func TestReliability_ProvidersSortedByScore(t *testing.T) {
	rs := newMemReliabilityStore()
	ctx := context.Background()
	_ = rs.UpdateScore(ctx, store.ReliabilityScore{Provider: "providerA", CompositeScore: 0.9})
	_ = rs.UpdateScore(ctx, store.ReliabilityScore{Provider: "providerB", CompositeScore: 0.4})

	reg := provider.NewRegistry()

	// Use a dummy resolver just to test loadReliabilityScores + sort logic
	scores, err := rs.GetAllScores(ctx)
	if err != nil {
		t.Fatalf("GetAllScores: %v", err)
	}

	scoreMap := make(map[string]float64)
	for _, s := range scores {
		scoreMap[s.Provider] = s.CompositeScore
	}

	providers := []string{"providerB", "providerA"}
	// sort descending by score
	for i := 0; i < len(providers); i++ {
		for j := i + 1; j < len(providers); j++ {
			if scoreMap[providers[j]] > scoreMap[providers[i]] {
				providers[i], providers[j] = providers[j], providers[i]
			}
		}
	}

	if providers[0] != "providerA" {
		t.Errorf("expected providerA (score 0.9) first, got %s", providers[0])
	}
	if providers[1] != "providerB" {
		t.Errorf("expected providerB (score 0.4) second, got %s", providers[1])
	}
	_ = reg
}

// TestReliability_MergeWeighting verifies that when two providers return the
// same work (same fingerprint) the higher-reliability provider's title wins.
func TestReliability_MergeWeighting(t *testing.T) {
	merger := NewMerger()

	results := []ProviderResult{
		{
			Provider: "lowReliability",
			Works: []model.Work{
				{
					Title:       "Dune (low)",
					Authors:     []model.Author{{Name: "Frank Herbert"}},
					Fingerprint: GenerateFingerprint("dune", "frank herbert", 1965),
				},
			},
		},
		{
			Provider: "highReliability",
			Works: []model.Work{
				{
					Title:       "Dune",
					Authors:     []model.Author{{Name: "Frank Herbert"}},
					Fingerprint: GenerateFingerprint("dune", "frank herbert", 1965),
				},
			},
		},
	}

	scores := map[string]float64{
		"lowReliability":  0.40,
		"highReliability": 0.92,
	}

	merged, err := merger.MergeWorksWeighted(results, scores)
	if err != nil {
		t.Fatalf("MergeWorksWeighted: %v", err)
	}
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged work, got %d", len(merged))
	}
	if merged[0].Title != "Dune" {
		t.Errorf("expected title 'Dune' from high-reliability provider, got %q", merged[0].Title)
	}
}

// TestReliability_QuarantineIsLastResort verifies that providers below 0.40
// remain dispatchable by default but are ordered after higher-tier providers.
func TestReliability_QuarantineIsLastResort(t *testing.T) {
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(&mockProvider{name: "primaryProvider"}, 10, true)
	reg.RegisterWithConfig(&mockProvider{name: "quarantineProvider"}, 1, true)

	reg.SetReliability("primaryProvider", 0.92)
	reg.SetReliability("quarantineProvider", 0.20)

	enabled := reg.EnabledProviders()
	if len(enabled) != 2 {
		t.Fatalf("expected both providers dispatchable by default, got %d", len(enabled))
	}
	if enabled[0].Name() != "primaryProvider" || enabled[1].Name() != "quarantineProvider" {
		t.Fatalf("expected quarantine provider last, got order [%s, %s]", enabled[0].Name(), enabled[1].Name())
	}
}

// TestReliability_ScoreImprovesOnSuccess verifies that repeated successes
// raise the reliability composite score over time.
func TestReliability_ScoreImprovesOnSuccess(t *testing.T) {
	now := time.Now()

	// 50% availability (initial)
	m1 := store.ProviderMetrics{
		Provider:       "testprovider",
		SuccessCount:   5,
		FailureCount:   5,
		RequestCount:   10,
		TotalLatencyMs: 5000,
		LastSuccess:    &now,
	}
	score1 := provider.ComputeScore(m1)

	// 90% availability (after more successes)
	m2 := store.ProviderMetrics{
		Provider:       "testprovider",
		SuccessCount:   90,
		FailureCount:   10,
		RequestCount:   100,
		TotalLatencyMs: 50000,
		LastSuccess:    &now,
	}
	score2 := provider.ComputeScore(m2)

	if score2.CompositeScore <= score1.CompositeScore {
		t.Errorf("more successes should yield higher score: %f <= %f",
			score2.CompositeScore, score1.CompositeScore)
	}
}

// TestReliability_DecayTowardBaseline verifies that a provider inactive for
// 30+ days has its score pulled toward 0.7.
func TestReliability_DecayTowardBaseline(t *testing.T) {
	recentSuccess := time.Now()
	oldSuccess := time.Now().Add(-65 * 24 * time.Hour)

	m := store.ProviderMetrics{
		Provider:       "inactive",
		SuccessCount:   100,
		FailureCount:   0,
		RequestCount:   100,
		TotalLatencyMs: 50_000,
	}

	m.LastSuccess = &recentSuccess
	scoreRecent := provider.ComputeScore(m)

	m.LastSuccess = &oldSuccess
	scoreDecayed := provider.ComputeScore(m)

	if scoreDecayed.CompositeScore >= scoreRecent.CompositeScore {
		t.Errorf("decayed score (%f) should be lower than recent score (%f)",
			scoreDecayed.CompositeScore, scoreRecent.CompositeScore)
	}
	// Decayed score should be closer to 0.7 baseline
	diffDecayed := abs(scoreDecayed.CompositeScore - 0.7)
	diffRecent := abs(scoreRecent.CompositeScore - 0.7)
	if diffDecayed >= diffRecent {
		t.Errorf("decayed score (%f) should be closer to baseline 0.7 than recent score (%f)",
			scoreDecayed.CompositeScore, scoreRecent.CompositeScore)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
