package indexer

import (
	"testing"
	"time"
)

func TestStoreCreateOrGetSearchRequestDedupesByRequestKey(t *testing.T) {
	store := NewStore()
	query := QuerySpec{EntityType: "work", EntityID: "w1", Title: "Dune"}

	first := store.CreateOrGetSearchRequest("key-1", query, 3)
	second := store.CreateOrGetSearchRequest("key-1", query, 3)

	if first.ID != second.ID {
		t.Fatalf("expected same request id for duplicate request key")
	}
}

func TestStoreTryLockAndReschedule(t *testing.T) {
	store := NewStore()
	query := QuerySpec{EntityType: "work", EntityID: "w1", Title: "Dune"}
	req := store.CreateOrGetSearchRequest("key-1", query, 3)

	locked, ok, err := store.TryLockNextSearchRequest("worker-a", time.Now().UTC())
	if err != nil {
		t.Fatalf("try lock failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected lock success")
	}
	if locked.ID != req.ID {
		t.Fatalf("expected locked request id %d, got %d", req.ID, locked.ID)
	}
	if locked.Status != "running" {
		t.Fatalf("expected status running, got %s", locked.Status)
	}
	if locked.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1, got %d", locked.AttemptCount)
	}

	nextRun := time.Now().UTC().Add(2 * time.Minute)
	if err := store.RescheduleSearchRequest(req.ID, "temporary error", nextRun, false); err != nil {
		t.Fatalf("reschedule failed: %v", err)
	}

	rescheduled, err := store.GetSearchRequest(req.ID)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	if rescheduled.Status != "queued" {
		t.Fatalf("expected status queued after non-terminal reschedule, got %s", rescheduled.Status)
	}
	if !rescheduled.NotBefore.Equal(nextRun) {
		t.Fatalf("expected not_before to match reschedule time")
	}
	if rescheduled.LockedAt != nil {
		t.Fatalf("expected lock cleared after reschedule")
	}
}

func TestStoreRescheduleTerminalAfterMaxAttempts(t *testing.T) {
	store := NewStore()
	query := QuerySpec{EntityType: "work", EntityID: "w1", Title: "Dune"}
	req := store.CreateOrGetSearchRequest("key-1", query, 1)

	_, ok, err := store.TryLockNextSearchRequest("worker-a", time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("expected lock success, err=%v", err)
	}

	if err := store.RescheduleSearchRequest(req.ID, "fatal", time.Now().UTC().Add(time.Minute), false); err != nil {
		t.Fatalf("reschedule failed: %v", err)
	}
	rec, err := store.GetSearchRequest(req.ID)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	if rec.Status != "failed" {
		t.Fatalf("expected failed status when max attempts reached, got %s", rec.Status)
	}
}

func TestStoreRecomputeReliabilityUpdatesBackendTier(t *testing.T) {
	store := NewStore()
	store.UpsertBackend(BackendRecord{
		ID:               "prowlarr:primary",
		Name:             "prowlarr-primary",
		BackendType:      BackendTypeProwlarr,
		Enabled:          true,
		Tier:             TierUnclassified,
		ReliabilityScore: 0.70,
		Priority:         100,
	})

	if err := store.RecordBackendSearchResult("prowlarr:primary", true, 200*time.Millisecond, true); err != nil {
		t.Fatalf("record result failed: %v", err)
	}
	if err := store.RecordBackendSearchResult("prowlarr:primary", true, 250*time.Millisecond, true); err != nil {
		t.Fatalf("record result failed: %v", err)
	}
	if err := store.RecordBackendSearchResult("prowlarr:primary", false, 300*time.Millisecond, false); err != nil {
		t.Fatalf("record result failed: %v", err)
	}

	if err := store.RecomputeReliability(); err != nil {
		t.Fatalf("recompute reliability failed: %v", err)
	}

	backends := store.ListBackends()
	if len(backends) != 1 {
		t.Fatalf("expected one backend, got %d", len(backends))
	}
	if backends[0].ReliabilityScore <= 0 {
		t.Fatalf("expected reliability score > 0")
	}
	if backends[0].Tier == TierQuarantine {
		t.Fatalf("did not expect quarantine tier for mostly successful backend")
	}
}
