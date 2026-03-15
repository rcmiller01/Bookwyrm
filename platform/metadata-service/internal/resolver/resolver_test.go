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

// ============================================================
// Mock: cache
// ============================================================

type mockCache struct {
	data map[string]interface{}
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string]interface{})}
}
func (c *mockCache) Get(key string) (interface{}, bool) {
	v, ok := c.data[key]
	return v, ok
}
func (c *mockCache) Set(key string, value interface{}, _ time.Duration) { c.data[key] = value }
func (c *mockCache) Delete(key string)                                  { delete(c.data, key) }

// ============================================================
// Mock: provider
// ============================================================

type mockProvider struct {
	name           string
	results        []model.Work
	err            error
	calls          int // incremented on each SearchWorks call
	resolveCalls   int
	resolveEdition *model.Edition
	resolveErr     error
	caps           provider.Capabilities
}

func (p *mockProvider) Name() string { return p.name }
func (p *mockProvider) SearchWorks(_ context.Context, _ string) ([]model.Work, error) {
	p.calls++
	return p.results, p.err
}
func (p *mockProvider) GetWork(_ context.Context, _ string) (*model.Work, error) { return nil, nil }
func (p *mockProvider) GetEditions(_ context.Context, _ string) ([]model.Edition, error) {
	return nil, nil
}
func (p *mockProvider) ResolveIdentifier(_ context.Context, _, _ string) (*model.Edition, error) {
	p.resolveCalls++
	return p.resolveEdition, p.resolveErr
}
func (p *mockProvider) Capabilities() provider.Capabilities { return p.caps }

// ============================================================
// Mock stores
// ============================================================

type mockWorkStore struct {
	dbResults []model.Work
	byID      map[string]*model.Work
}

func newMockWorkStore(results []model.Work) *mockWorkStore {
	m := &mockWorkStore{dbResults: results, byID: make(map[string]*model.Work)}
	for i := range results {
		m.byID[results[i].ID] = &results[i]
	}
	return m
}

func (s *mockWorkStore) GetWorkByID(_ context.Context, id string) (*model.Work, error) {
	if w, ok := s.byID[id]; ok {
		return w, nil
	}
	return nil, errors.New("not found")
}
func (s *mockWorkStore) SearchWorks(_ context.Context, _ string) ([]model.Work, error) {
	return s.dbResults, nil
}
func (s *mockWorkStore) InsertWork(_ context.Context, w model.Work) error {
	s.byID[w.ID] = &w
	return nil
}
func (s *mockWorkStore) UpdateWork(_ context.Context, _ model.Work) error { return nil }
func (s *mockWorkStore) GetWorkByFingerprint(_ context.Context, _ string) (*model.Work, error) {
	return nil, errors.New("not found")
}

type noopAuthorStore struct{}

func (s *noopAuthorStore) InsertAuthor(_ context.Context, _ model.Author) error { return nil }
func (s *noopAuthorStore) GetAuthorByName(_ context.Context, _ string) (*model.Author, error) {
	return nil, errors.New("not found")
}
func (s *noopAuthorStore) LinkWorkAuthor(_ context.Context, _, _ string) error { return nil }

type noopEditionStore struct{}

func (s *noopEditionStore) InsertEdition(_ context.Context, _ model.Edition) error { return nil }
func (s *noopEditionStore) GetEditionsByWork(_ context.Context, _ string) ([]model.Edition, error) {
	return nil, nil
}
func (s *noopEditionStore) GetEditionByID(_ context.Context, _ string) (*model.Edition, error) {
	return nil, errors.New("not found")
}

type noopIdentifierStore struct{}

func (s *noopIdentifierStore) InsertIdentifier(_ context.Context, _ string, _ model.Identifier) error {
	return nil
}
func (s *noopIdentifierStore) FindEditionByIdentifier(_ context.Context, _, _ string) (*model.Edition, error) {
	return nil, errors.New("not found")
}
func (s *noopIdentifierStore) GetIdentifiersByEdition(_ context.Context, _ string) ([]model.Identifier, error) {
	return nil, nil
}

type noopMappingStore struct{}

func (s *noopMappingStore) GetCanonicalID(_ context.Context, _, _ string) (string, error) {
	return "", errors.New("not found")
}
func (s *noopMappingStore) InsertMapping(_ context.Context, _, _, _, _ string) error { return nil }

type noopStatusStore struct{}

func (s *noopStatusStore) GetAll(_ context.Context) ([]store.ProviderStatus, error) { return nil, nil }
func (s *noopStatusStore) GetByName(_ context.Context, _ string) (*store.ProviderStatus, error) {
	return nil, errors.New("not found")
}
func (s *noopStatusStore) UpdateStatus(_ context.Context, _ string, _ string, _ int, _ int64) error {
	return nil
}
func (s *noopStatusStore) RecordSuccess(_ context.Context, _ string, _ int64) error { return nil }
func (s *noopStatusStore) RecordFailure(_ context.Context, _ string) error          { return nil }

// ============================================================
// Helpers
// ============================================================

func buildResolver(ws *mockWorkStore, reg *provider.Registry, rl *provider.RateLimiter) Resolver {
	c := newMockCache()
	s := Stores{
		Works:    ws,
		Authors:  &noopAuthorStore{},
		Editions: &noopEditionStore{},
		IDs:      &noopIdentifierStore{},
		Mappings: &noopMappingStore{},
		Status:   &noopStatusStore{},
	}
	return New(reg, rl, s, c)
}

func buildResolverWithCache(ws *mockWorkStore, reg *provider.Registry, rl *provider.RateLimiter, c *mockCache) Resolver {
	s := Stores{
		Works:    ws,
		Authors:  &noopAuthorStore{},
		Editions: &noopEditionStore{},
		IDs:      &noopIdentifierStore{},
		Mappings: &noopMappingStore{},
		Status:   &noopStatusStore{},
	}
	return New(reg, rl, s, c)
}

// ============================================================
// Tests: SearchWorks
// ============================================================

// When all providers return errors and the DB has partial results, the resolver
// should degrade gracefully and return what the DB had rather than failing.
func TestSearchWorks_AllProvidersFail_FallsBackToDBResults(t *testing.T) {
	dbWork := model.Work{ID: "db-1", Title: "DB Book", NormalizedTitle: "db book"}
	ws := newMockWorkStore([]model.Work{dbWork}) // 1 result → below the 3-result early-exit threshold

	failProvider := &mockProvider{name: "fails", err: errors.New("upstream down")}
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(failProvider, 1, true)

	res := buildResolver(ws, reg, provider.NewRateLimiter())
	works, err := res.SearchWorks(context.Background(), "db book")
	if err != nil {
		t.Fatalf("expected no error on fallback, got: %v", err)
	}
	if len(works) != 1 || works[0].ID != "db-1" {
		t.Errorf("expected fallback DB result {id:db-1}, got %+v", works)
	}
}

// When DB is empty and all providers fail, the resolver should return empty
// results without an error — partial failure is not a hard error.
func TestSearchWorks_AllProvidersFail_EmptyDB_ReturnsEmpty(t *testing.T) {
	ws := newMockWorkStore(nil)
	failProvider := &mockProvider{name: "fail", err: errors.New("down")}
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(failProvider, 1, true)

	res := buildResolver(ws, reg, provider.NewRateLimiter())
	works, err := res.SearchWorks(context.Background(), "anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(works) > 0 {
		t.Errorf("expected empty results, got %d", len(works))
	}
}

// When a provider succeeds the results should flow through merge and be returned.
func TestSearchWorks_ProviderSucceeds_ReturnsResults(t *testing.T) {
	ws := newMockWorkStore(nil)
	provWork := model.Work{ID: "pv-1", Title: "Provider Book", Fingerprint: "fp-pv"}
	okProvider := &mockProvider{name: "ok", results: []model.Work{provWork}}

	reg := provider.NewRegistry()
	reg.RegisterWithConfig(okProvider, 1, true)

	res := buildResolver(ws, reg, provider.NewRateLimiter())
	works, err := res.SearchWorks(context.Background(), "provider book")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(works) == 0 {
		t.Fatal("expected at least one result from provider, got none")
	}
	if works[0].Title != "Provider Book" {
		t.Errorf("expected title 'Provider Book', got %q", works[0].Title)
	}
}

// When the DB already has ≥3 results the resolver should return them immediately
// without ever calling providers.
func TestSearchWorks_DBSufficient_SkipsProviders(t *testing.T) {
	dbWorks := []model.Work{
		{ID: "d1", Title: "Alpha", NormalizedTitle: "alpha"},
		{ID: "d2", Title: "Beta", NormalizedTitle: "beta"},
		{ID: "d3", Title: "Gamma", NormalizedTitle: "gamma"},
	}
	ws := newMockWorkStore(dbWorks)
	shouldNotCall := &mockProvider{name: "skip-me"}
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(shouldNotCall, 1, true)

	res := buildResolver(ws, reg, provider.NewRateLimiter())
	works, err := res.SearchWorks(context.Background(), "anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(works) != 3 {
		t.Errorf("expected 3 DB results, got %d", len(works))
	}
	if shouldNotCall.calls != 0 {
		t.Errorf("provider should not be called when DB has ≥3 results, got %d calls", shouldNotCall.calls)
	}
}

// A provider whose rate limiter bucket is empty should be silently skipped.
func TestSearchWorks_RateLimitedProvider_Skipped(t *testing.T) {
	ws := newMockWorkStore(nil)
	throttled := &mockProvider{name: "throttled", results: []model.Work{
		{ID: "t1", Title: "Throttled Book"},
	}}
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(throttled, 1, true)

	rl := provider.NewRateLimiter()
	rl.Configure("throttled", 0) // zero tokens → immediate exhaustion

	res := buildResolver(ws, reg, rl)
	_, _ = res.SearchWorks(context.Background(), "anything")
	if throttled.calls != 0 {
		t.Errorf("rate-limited provider should not be called, got %d calls", throttled.calls)
	}
}

// A cache hit should be returned immediately; providers must not be queried.
func TestSearchWorks_CacheHit_SkipsProviders(t *testing.T) {
	ws := newMockWorkStore(nil)
	shouldNotCall := &mockProvider{name: "skip-cached"}
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(shouldNotCall, 1, true)

	c := newMockCache()
	cachedWorks := []model.Work{{ID: "c-1", Title: "Cached Book"}}
	cq := ClassifyQuery("cached book")
	c.Set("search:"+cq.Normalized, cachedWorks, time.Hour)

	res := buildResolverWithCache(ws, reg, provider.NewRateLimiter(), c)
	works, err := res.SearchWorks(context.Background(), "cached book")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(works) != 1 || works[0].ID != "c-1" {
		t.Errorf("expected cached result, got %+v", works)
	}
	if shouldNotCall.calls != 0 {
		t.Errorf("provider should not be called on cache hit, got %d calls", shouldNotCall.calls)
	}
}

// Results from providers with distinct fingerprints should both appear after merge.
func TestSearchWorks_MultipleProviders_ResultsMerged(t *testing.T) {
	ws := newMockWorkStore(nil)
	work1 := model.Work{ID: "w1", Title: "First Book", Fingerprint: "fp-first"}
	work2 := model.Work{ID: "w2", Title: "Second Book", Fingerprint: "fp-second"}

	p1 := &mockProvider{name: "p1", results: []model.Work{work1}}
	p2 := &mockProvider{name: "p2", results: []model.Work{work2}}
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(p1, 1, true)
	reg.RegisterWithConfig(p2, 2, true)

	res := buildResolver(ws, reg, provider.NewRateLimiter())
	works, err := res.SearchWorks(context.Background(), "book")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(works) < 2 {
		t.Errorf("expected merged results from both providers, got %d", len(works))
	}
}

// A disabled provider should never be called by the resolver.
func TestSearchWorks_DisabledProvider_NeverCalled(t *testing.T) {
	ws := newMockWorkStore(nil)
	disabled := &mockProvider{name: "disabled", results: []model.Work{
		{ID: "x1", Title: "Disabled Result"},
	}}
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(disabled, 1, false) // explicitly disabled

	res := buildResolver(ws, reg, provider.NewRateLimiter())
	_, _ = res.SearchWorks(context.Background(), "anything")
	if disabled.calls != 0 {
		t.Errorf("disabled provider should never be called, got %d calls", disabled.calls)
	}
}

// Successive calls for the same query should cache results and not re-query providers.
func TestSearchWorks_ResultsCachedAfterProviderQuery(t *testing.T) {
	ws := newMockWorkStore(nil)
	prov := &mockProvider{name: "counted", results: []model.Work{
		{ID: "r1", Title: "Countable Book", Fingerprint: "fp-count"},
	}}
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(prov, 1, true)

	c := newMockCache()
	res := buildResolverWithCache(ws, reg, provider.NewRateLimiter(), c)

	// First call — hits provider
	_, _ = res.SearchWorks(context.Background(), "countable book")
	firstCallCount := prov.calls

	// Second call — should hit cache
	_, _ = res.SearchWorks(context.Background(), "countable book")
	if prov.calls != firstCallCount {
		t.Errorf("second identical query should be served from cache; provider calls went from %d to %d",
			firstCallCount, prov.calls)
	}
}

func TestResolveIdentifier_SkipsUnsupportedProviders(t *testing.T) {
	ws := newMockWorkStore(nil)
	unsupported := &mockProvider{
		name: "no-doi",
		caps: provider.Capabilities{
			SupportsSearch: true,
			SupportsISBN:   true,
			SupportsDOI:    false,
		},
	}
	supported := &mockProvider{
		name: "yes-doi",
		caps: provider.Capabilities{
			SupportsSearch: true,
			SupportsISBN:   false,
			SupportsDOI:    true,
		},
		resolveEdition: &model.Edition{
			ID: "ed-1",
			Identifiers: []model.Identifier{
				{Type: "DOI", Value: "10.1234/example"},
			},
		},
	}
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(unsupported, 10, true)
	reg.RegisterWithConfig(supported, 20, true)

	res := buildResolver(ws, reg, provider.NewRateLimiter())
	edition, err := res.ResolveIdentifier(context.Background(), "DOI", "10.1234/example")
	if err != nil {
		t.Fatalf("resolve identifier failed: %v", err)
	}
	if edition == nil || edition.ID != "ed-1" {
		t.Fatalf("expected resolved edition from DOI-capable provider")
	}
	if unsupported.resolveCalls != 0 {
		t.Fatalf("expected DOI-unsupported provider to be skipped, got %d calls", unsupported.resolveCalls)
	}
	if supported.resolveCalls != 1 {
		t.Fatalf("expected DOI-capable provider to be called once, got %d", supported.resolveCalls)
	}
}
