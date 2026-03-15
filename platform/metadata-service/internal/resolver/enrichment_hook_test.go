package resolver

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

type fakeEnrichmentHookStore struct {
	mu           sync.Mutex
	enqueueDelay time.Duration
	attempts     int
	uniqueJobs   map[string]model.EnrichmentJob
	enqueued     []model.EnrichmentJob
	notify       chan struct{}
}

func newFakeEnrichmentHookStore() *fakeEnrichmentHookStore {
	return &fakeEnrichmentHookStore{
		uniqueJobs: map[string]model.EnrichmentJob{},
		notify:     make(chan struct{}, 32),
	}
}

func (s *fakeEnrichmentHookStore) EnqueueJob(_ context.Context, job model.EnrichmentJob) (int64, error) {
	if s.enqueueDelay > 0 {
		time.Sleep(s.enqueueDelay)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts++
	key := job.JobType + ":" + job.EntityType + ":" + job.EntityID
	if _, ok := s.uniqueJobs[key]; !ok {
		s.uniqueJobs[key] = job
		s.enqueued = append(s.enqueued, job)
	}
	select {
	case s.notify <- struct{}{}:
	default:
	}
	return int64(len(s.enqueued)), nil
}

func (s *fakeEnrichmentHookStore) waitForAttempts(t *testing.T, n int, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		s.mu.Lock()
		got := s.attempts
		s.mu.Unlock()
		if got >= n {
			return
		}
		select {
		case <-s.notify:
		case <-deadline:
			t.Fatalf("timed out waiting for %d enqueue attempts", n)
		}
	}
}

func (s *fakeEnrichmentHookStore) GetJobByID(_ context.Context, _ int64) (*model.EnrichmentJob, error) {
	return nil, errors.New("not used")
}
func (s *fakeEnrichmentHookStore) TryLockNextJob(_ context.Context, _ string) (*model.EnrichmentJob, error) {
	return nil, store.ErrNoAvailableEnrichmentJobs
}
func (s *fakeEnrichmentHookStore) MarkSucceeded(_ context.Context, _ int64) error { return nil }
func (s *fakeEnrichmentHookStore) MarkFailed(_ context.Context, _ int64, _ string, _ string, _ time.Duration) error {
	return nil
}
func (s *fakeEnrichmentHookStore) MarkDead(_ context.Context, _ int64, _ string) error { return nil }
func (s *fakeEnrichmentHookStore) RecordRunStart(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
func (s *fakeEnrichmentHookStore) RecordRunFinish(_ context.Context, _ int64, _ string, _ string) error {
	return nil
}
func (s *fakeEnrichmentHookStore) ListJobs(_ context.Context, _ model.EnrichmentJobFilters) ([]model.EnrichmentJob, error) {
	return nil, nil
}
func (s *fakeEnrichmentHookStore) CountJobsByStatus(_ context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}
func (s *fakeEnrichmentHookStore) NextRunnableAt(_ context.Context) (*time.Time, error) {
	return nil, nil
}

func buildResolverWithEnrichment(ws *mockWorkStore, reg *provider.Registry, rl *provider.RateLimiter, c *mockCache, es store.EnrichmentJobStore) Resolver {
	s := Stores{
		Works:      ws,
		Authors:    &noopAuthorStore{},
		Editions:   &noopEditionStore{},
		IDs:        &noopIdentifierStore{},
		Mappings:   &noopMappingStore{},
		Status:     &noopStatusStore{},
		Enrichment: es,
	}
	return New(reg, rl, s, c)
}

type mockIdentifierProvider struct {
	name    string
	edition *model.Edition
}

func (p *mockIdentifierProvider) Name() string { return p.name }
func (p *mockIdentifierProvider) SearchWorks(_ context.Context, _ string) ([]model.Work, error) {
	return nil, nil
}
func (p *mockIdentifierProvider) GetWork(_ context.Context, _ string) (*model.Work, error) {
	return nil, nil
}
func (p *mockIdentifierProvider) GetEditions(_ context.Context, _ string) ([]model.Edition, error) {
	return nil, nil
}
func (p *mockIdentifierProvider) ResolveIdentifier(_ context.Context, _, _ string) (*model.Edition, error) {
	return p.edition, nil
}

func TestScheduleSearchEnrichment_EnqueuesTopConfidentWorks(t *testing.T) {
	es := newFakeEnrichmentHookStore()
	r := &defaultResolver{enrichment: es}

	works := []model.Work{
		{ID: "w1", Confidence: 0.90},
		{ID: "w2", Confidence: 0.84},
		{ID: "", Confidence: 0.95},
		{ID: "w3", Confidence: 0.95},
		{ID: "w4", Confidence: 0.88},
		{ID: "w5", Confidence: 0.99},
	}

	r.scheduleSearchEnrichment(context.Background(), works)
	es.waitForAttempts(t, 3, 2*time.Second)

	es.mu.Lock()
	defer es.mu.Unlock()
	if len(es.enqueued) != 3 {
		t.Fatalf("expected 3 enqueued jobs, got %d", len(es.enqueued))
	}
	for _, job := range es.enqueued {
		if job.JobType != model.EnrichmentJobTypeWorkEditions || job.EntityType != "work" || job.Priority != 50 {
			t.Fatalf("unexpected enqueued job payload: %+v", job)
		}
	}
}

func TestScheduleSearchEnrichment_DedupeFriendly(t *testing.T) {
	es := newFakeEnrichmentHookStore()
	r := &defaultResolver{enrichment: es}

	r.scheduleSearchEnrichment(context.Background(), []model.Work{
		{ID: "same", Confidence: 0.90},
		{ID: "same", Confidence: 0.92},
	})
	es.waitForAttempts(t, 2, 2*time.Second)

	es.mu.Lock()
	defer es.mu.Unlock()
	if es.attempts != 2 {
		t.Fatalf("expected 2 enqueue attempts, got %d", es.attempts)
	}
	if len(es.enqueued) != 1 {
		t.Fatalf("expected dedupe store to keep 1 unique job, got %d", len(es.enqueued))
	}
}

func TestScheduleSearchEnrichment_IsNonBlocking(t *testing.T) {
	es := newFakeEnrichmentHookStore()
	es.enqueueDelay = 250 * time.Millisecond
	r := &defaultResolver{enrichment: es}

	start := time.Now()
	r.scheduleSearchEnrichment(context.Background(), []model.Work{{ID: "w1", Confidence: 0.95}})
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Fatalf("scheduleSearchEnrichment should return quickly, took %v", elapsed)
	}
}

func TestSearchWorks_CacheHit_DoesNotScheduleEnrichment(t *testing.T) {
	ws := newMockWorkStore(nil)
	providerShouldNotCall := &mockProvider{name: "skip"}
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(providerShouldNotCall, 1, true)

	c := newMockCache()
	cq := ClassifyQuery("cached query")
	c.Set("search:"+cq.Normalized, []model.Work{{ID: "cached-1", Title: "Cached"}}, time.Hour)

	es := newFakeEnrichmentHookStore()
	res := buildResolverWithEnrichment(ws, reg, provider.NewRateLimiter(), c, es)

	_, err := res.SearchWorks(context.Background(), "cached query")
	if err != nil {
		t.Fatalf("search works cache hit: %v", err)
	}

	es.mu.Lock()
	attempts := es.attempts
	es.mu.Unlock()
	if attempts != 0 {
		t.Fatalf("expected no enrichment enqueue attempts on cache hit, got %d", attempts)
	}
}

func TestResolveIdentifier_Success_EnqueuesWorkEditions(t *testing.T) {
	ws := newMockWorkStore(nil)
	idProvider := &mockIdentifierProvider{name: "idp", edition: &model.Edition{ID: "ed1", WorkID: "w-resolved"}}
	reg := provider.NewRegistry()
	reg.RegisterWithConfig(idProvider, 1, true)

	es := newFakeEnrichmentHookStore()
	res := buildResolverWithEnrichment(ws, reg, provider.NewRateLimiter(), newMockCache(), es)

	edition, err := res.ResolveIdentifier(context.Background(), "ISBN_13", "9780000000000")
	if err != nil {
		t.Fatalf("resolve identifier: %v", err)
	}
	if edition == nil || edition.WorkID != "w-resolved" {
		t.Fatalf("expected resolved edition with work id, got %+v", edition)
	}

	es.waitForAttempts(t, 2, 2*time.Second)
	es.mu.Lock()
	defer es.mu.Unlock()
	if len(es.enqueued) != 2 {
		t.Fatalf("expected two enqueues after identifier resolve, got %d", len(es.enqueued))
	}
	seen := map[string]bool{}
	for _, job := range es.enqueued {
		if job.EntityID != "w-resolved" || job.EntityType != "work" {
			t.Fatalf("unexpected scheduled enrichment job payload: %+v", job)
		}
		seen[job.JobType] = true
	}
	if !seen[model.EnrichmentJobTypeWorkEditions] || !seen[model.EnrichmentJobTypeGraphUpdate] {
		t.Fatalf("expected both work_editions and graph_update_work jobs, got %+v", es.enqueued)
	}
}
