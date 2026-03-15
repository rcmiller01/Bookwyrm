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
	if locked.LeaseExpiresAt == nil {
		t.Fatalf("expected lease_expires_at to be set")
	}
	if !locked.LeaseExpiresAt.After(*locked.LockedAt) {
		t.Fatalf("expected lease_expires_at after locked_at")
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
	if rescheduled.LeaseExpiresAt != nil {
		t.Fatalf("expected lease_expires_at cleared after reschedule")
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

func TestStoreRecoverExpiredSearchRequestsRequeues(t *testing.T) {
	store := NewStore()
	query := QuerySpec{EntityType: "work", EntityID: "w1", Title: "Dune"}
	req := store.CreateOrGetSearchRequest("key-recover-1", query, 3)

	lockTime := time.Now().UTC()
	_, ok, err := store.TryLockNextSearchRequest("worker-a", lockTime)
	if err != nil || !ok {
		t.Fatalf("expected lock success, err=%v", err)
	}

	recoveredAt := lockTime.Add(SearchRequestLeaseTTL).Add(time.Second)
	recovered, err := store.RecoverExpiredSearchRequests(recoveredAt, 10)
	if err != nil {
		t.Fatalf("recover expired failed: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected one recovered request, got %d", recovered)
	}

	rec, err := store.GetSearchRequest(req.ID)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	if rec.Status != "queued" {
		t.Fatalf("expected queued after recovery, got %s", rec.Status)
	}
	if rec.AttemptCount != 2 {
		t.Fatalf("expected attempt_count=2 after recovery increment, got %d", rec.AttemptCount)
	}
	if rec.LockedAt != nil || rec.LeaseExpiresAt != nil || rec.LockedBy != "" {
		t.Fatalf("expected lock fields cleared after recovery")
	}
}

func TestStoreRecoverExpiredSearchRequestsTerminal(t *testing.T) {
	store := NewStore()
	query := QuerySpec{EntityType: "work", EntityID: "w1", Title: "Dune"}
	req := store.CreateOrGetSearchRequest("key-recover-2", query, 1)

	lockTime := time.Now().UTC()
	_, ok, err := store.TryLockNextSearchRequest("worker-a", lockTime)
	if err != nil || !ok {
		t.Fatalf("expected lock success, err=%v", err)
	}

	recoveredAt := lockTime.Add(SearchRequestLeaseTTL).Add(time.Second)
	recovered, err := store.RecoverExpiredSearchRequests(recoveredAt, 10)
	if err != nil {
		t.Fatalf("recover expired failed: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected one recovered request, got %d", recovered)
	}

	rec, err := store.GetSearchRequest(req.ID)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	if rec.Status != "failed" {
		t.Fatalf("expected failed after recovery at max attempts, got %s", rec.Status)
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

func TestStoreWantedWorkAndAuthorDueLifecycle(t *testing.T) {
	store := NewStore()
	now := time.Now().UTC()

	_, err := store.SetWantedWork(WantedWorkRecord{
		WorkID:         "work-1",
		Enabled:        true,
		Priority:       10,
		CadenceMinutes: 30,
		Formats:        []string{"epub"},
		Languages:      []string{"en"},
	})
	if err != nil {
		t.Fatalf("set wanted work failed: %v", err)
	}
	_, err = store.SetWantedAuthor(WantedAuthorRecord{
		AuthorID:       "author-1",
		Enabled:        true,
		Priority:       20,
		CadenceMinutes: 30,
	})
	if err != nil {
		t.Fatalf("set wanted author failed: %v", err)
	}

	if due := store.ListDueWantedWorks(now); len(due) != 1 {
		t.Fatalf("expected 1 due wanted work, got %d", len(due))
	}
	if due := store.ListDueWantedAuthors(now); len(due) != 1 {
		t.Fatalf("expected 1 due wanted author, got %d", len(due))
	}

	if err := store.MarkWantedWorkEnqueued("work-1", now); err != nil {
		t.Fatalf("mark wanted work enqueued failed: %v", err)
	}
	if err := store.MarkWantedAuthorEnqueued("author-1", now); err != nil {
		t.Fatalf("mark wanted author enqueued failed: %v", err)
	}

	if due := store.ListDueWantedWorks(now.Add(10 * time.Minute)); len(due) != 0 {
		t.Fatalf("expected no due wanted work before cadence, got %d", len(due))
	}
	if due := store.ListDueWantedAuthors(now.Add(10 * time.Minute)); len(due) != 0 {
		t.Fatalf("expected no due wanted author before cadence, got %d", len(due))
	}

	if due := store.ListDueWantedWorks(now.Add(31 * time.Minute)); len(due) != 1 {
		t.Fatalf("expected due wanted work after cadence, got %d", len(due))
	}
	if due := store.ListDueWantedAuthors(now.Add(31 * time.Minute)); len(due) != 1 {
		t.Fatalf("expected due wanted author after cadence, got %d", len(due))
	}
}

func TestStorePruneStaleCandidatesKeepsMostRecent(t *testing.T) {
	store := NewStore()
	query := QuerySpec{EntityType: "work", EntityID: "w-prune", Title: "Dune"}
	req := store.CreateOrGetSearchRequest("key-prune", query, 3)

	_, err := store.ReplaceCandidates(req.ID, []Candidate{
		{CandidateID: "c1", Title: "A"},
		{CandidateID: "c2", Title: "B"},
		{CandidateID: "c3", Title: "C"},
		{CandidateID: "c4", Title: "D"},
	})
	if err != nil {
		t.Fatalf("replace candidates failed: %v", err)
	}

	pruned, err := store.PruneStaleCandidates(2)
	if err != nil {
		t.Fatalf("prune stale candidates failed: %v", err)
	}
	if pruned != 2 {
		t.Fatalf("expected 2 pruned candidates, got %d", pruned)
	}

	recs, err := store.ListCandidates(req.ID, 10)
	if err != nil {
		t.Fatalf("list candidates failed: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 retained candidates, got %d", len(recs))
	}
}
