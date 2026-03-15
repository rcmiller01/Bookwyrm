package indexer

import (
	"context"
	"sync"
	"testing"
	"time"
)

type recordingBackend struct {
	id       string
	name     string
	pipeline string
	order    *[]string
	mu       *sync.Mutex
}

func (b *recordingBackend) ID() string       { return b.id }
func (b *recordingBackend) Name() string     { return b.name }
func (b *recordingBackend) Pipeline() string { return b.pipeline }
func (b *recordingBackend) Capabilities() BackendCapabilities {
	return BackendCapabilities{Supports: []string{"search"}}
}
func (b *recordingBackend) Search(_ context.Context, q QuerySpec) ([]Candidate, error) {
	b.mu.Lock()
	*b.order = append(*b.order, b.id)
	b.mu.Unlock()
	return []Candidate{{
		CandidateID:     b.id + ":" + q.EntityID,
		Title:           q.Title + " " + b.id,
		NormalizedTitle: normalizeText(q.Title + " " + b.id),
		Protocol:        "usenet",
		SourceBackendID: b.id,
		SourcePipeline:  b.pipeline,
		GrabPayload:     map[string]any{"protocol": "usenet", "guid": b.id},
	}}, nil
}
func (b *recordingBackend) HealthCheck(_ context.Context) error { return nil }

func TestOrchestratorOrderingByTierThenReliabilityThenPriority(t *testing.T) {
	store := NewStore()
	orch := NewOrchestrator(store, "last_resort")

	callOrder := []string{}
	mu := &sync.Mutex{}
	register := func(id string, tier DispatchTier, reliability float64, priority int) {
		orch.RegisterBackend(
			&recordingBackend{id: id, name: id, pipeline: "prowlarr", order: &callOrder, mu: mu},
			BackendRecord{
				ID:               id,
				Name:             id,
				BackendType:      BackendTypeProwlarr,
				Enabled:          true,
				Tier:             tier,
				ReliabilityScore: reliability,
				Priority:         priority,
			},
		)
	}

	register("secondary-best", TierSecondary, 0.95, 100)
	register("primary-low", TierPrimary, 0.60, 100)
	register("primary-high", TierPrimary, 0.90, 100)
	register("primary-high-pri", TierPrimary, 0.90, 50)

	req := orch.Enqueue(QuerySpec{EntityType: "work", EntityID: "w1", Title: "Dune"})
	if err := orch.ProcessRequest(context.Background(), req.ID); err != nil {
		t.Fatalf("process request failed: %v", err)
	}

	want := []string{"primary-high-pri", "primary-high", "primary-low", "secondary-best"}
	if len(callOrder) != len(want) {
		t.Fatalf("expected %d backend calls, got %d", len(want), len(callOrder))
	}
	for i := range want {
		if callOrder[i] != want[i] {
			t.Fatalf("unexpected backend order at %d: got %s want %s", i, callOrder[i], want[i])
		}
	}
}

func TestOrchestratorQuarantineDisabledSkipsQuarantinedBackends(t *testing.T) {
	store := NewStore()
	orch := NewOrchestrator(store, "disabled")

	callOrder := []string{}
	mu := &sync.Mutex{}
	orch.RegisterBackend(
		&recordingBackend{id: "primary", name: "primary", pipeline: "prowlarr", order: &callOrder, mu: mu},
		BackendRecord{
			ID:               "primary",
			Name:             "primary",
			BackendType:      BackendTypeProwlarr,
			Enabled:          true,
			Tier:             TierPrimary,
			ReliabilityScore: 0.80,
			Priority:         100,
		},
	)
	orch.RegisterBackend(
		&recordingBackend{id: "quarantined", name: "quarantined", pipeline: "prowlarr", order: &callOrder, mu: mu},
		BackendRecord{
			ID:               "quarantined",
			Name:             "quarantined",
			BackendType:      BackendTypeProwlarr,
			Enabled:          true,
			Tier:             TierQuarantine,
			ReliabilityScore: 0.99,
			Priority:         1,
		},
	)

	req := orch.Enqueue(QuerySpec{EntityType: "work", EntityID: "w1", Title: "Dune"})
	if err := orch.ProcessRequest(context.Background(), req.ID); err != nil {
		t.Fatalf("process request failed: %v", err)
	}

	if len(callOrder) != 1 || callOrder[0] != "primary" {
		t.Fatalf("expected only primary backend to run, got %v", callOrder)
	}
}

func TestOrchestratorEnqueueDueWantedCreatesRequestsAndMarksEnqueued(t *testing.T) {
	store := NewStore()
	orch := NewOrchestrator(store, "last_resort")
	now := time.Now().UTC()

	_, err := store.SetWantedWork(WantedWorkRecord{
		WorkID:         "work-42",
		Enabled:        true,
		Priority:       10,
		CadenceMinutes: 60,
		Formats:        []string{"epub"},
		Languages:      []string{"en"},
	})
	if err != nil {
		t.Fatalf("set wanted work failed: %v", err)
	}
	_, err = store.SetWantedAuthor(WantedAuthorRecord{
		AuthorID:       "author-42",
		Enabled:        true,
		Priority:       20,
		CadenceMinutes: 60,
	})
	if err != nil {
		t.Fatalf("set wanted author failed: %v", err)
	}

	orch.enqueueDueWanted(now)

	workQuery := QuerySpec{EntityType: "work", EntityID: "work-42"}
	workQuery.Preferences.Formats = []string{"epub"}
	workQuery.Preferences.Languages = []string{"en"}
	workReq := store.CreateOrGetSearchRequest(buildRequestKey(workQuery), workQuery, 3)
	if workReq.ID <= 0 {
		t.Fatalf("expected enqueued work search request")
	}
	authorQuery := QuerySpec{EntityType: "author", EntityID: "author-42"}
	authorReq := store.CreateOrGetSearchRequest(buildRequestKey(authorQuery), authorQuery, 3)
	if authorReq.ID <= 0 {
		t.Fatalf("expected enqueued author search request")
	}

	if due := store.ListDueWantedWorks(now); len(due) != 0 {
		t.Fatalf("expected no immediately due wanted works after enqueue, got %d", len(due))
	}
	if due := store.ListDueWantedAuthors(now); len(due) != 0 {
		t.Fatalf("expected no immediately due wanted authors after enqueue, got %d", len(due))
	}
}

func TestOrchestratorPreferredBackendsRunFirst(t *testing.T) {
	store := NewStore()
	orch := NewOrchestrator(store, "last_resort")

	callOrder := []string{}
	mu := &sync.Mutex{}

	orch.RegisterBackend(
		&recordingBackend{id: "non-preferred", name: "non-preferred", pipeline: "prowlarr", order: &callOrder, mu: mu},
		BackendRecord{ID: "non-preferred", Name: "non-preferred", BackendType: BackendTypeProwlarr, Enabled: true, Tier: TierPrimary, ReliabilityScore: 0.99, Priority: 1, Config: map[string]any{"preferred": false}},
	)
	orch.RegisterBackend(
		&recordingBackend{id: "preferred", name: "preferred", pipeline: "prowlarr", order: &callOrder, mu: mu},
		BackendRecord{ID: "preferred", Name: "preferred", BackendType: BackendTypeProwlarr, Enabled: true, Tier: TierSecondary, ReliabilityScore: 0.10, Priority: 500, Config: map[string]any{"preferred": true}},
	)

	req := orch.Enqueue(QuerySpec{EntityType: "work", EntityID: "work-pref", Title: "Dune"})
	if err := orch.ProcessRequest(context.Background(), req.ID); err != nil {
		t.Fatalf("process request failed: %v", err)
	}
	if len(callOrder) < 2 {
		t.Fatalf("expected at least two backend calls, got %v", callOrder)
	}
	if callOrder[0] != "preferred" {
		t.Fatalf("expected preferred backend first, got %v", callOrder)
	}
}
