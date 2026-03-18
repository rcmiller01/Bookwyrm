package downloadqueue

import (
	"testing"
	"time"
)

func TestStoreClaimSetsAndClearsLease(t *testing.T) {
	store := NewStore()
	job, err := store.CreateJob(Job{
		GrabID:      1,
		CandidateID: 1,
		WorkID:      "work-1",
		Protocol:    "torrent",
		ClientName:  "qbittorrent",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	locked, ok, err := store.ClaimNextQueued("worker-a", time.Now().UTC())
	if err != nil {
		t.Fatalf("claim job: %v", err)
	}
	if !ok {
		t.Fatalf("expected queued job to be claimed")
	}
	if locked.ID != job.ID {
		t.Fatalf("expected claimed id %d, got %d", job.ID, locked.ID)
	}
	if locked.LeaseExpiresAt == nil || locked.LockedAt == nil {
		t.Fatalf("expected lease and lock timestamps to be set")
	}
	if !locked.LeaseExpiresAt.After(*locked.LockedAt) {
		t.Fatalf("expected lease expiry after lock time")
	}

	if err := store.MarkSubmitted(job.ID, "downstream-1"); err != nil {
		t.Fatalf("mark submitted: %v", err)
	}
	updated, err := store.GetJob(job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.LeaseExpiresAt != nil {
		t.Fatalf("expected lease to clear after submit transition")
	}
}

func TestRecoverExpiredLeasesRequeuesSubmittedJobs(t *testing.T) {
	store := NewStore()
	job, err := store.CreateJob(Job{
		GrabID:      2,
		CandidateID: 2,
		WorkID:      "work-2",
		Protocol:    "usenet",
		ClientName:  "sabnzbd",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	locked, ok, err := store.ClaimNextQueued("worker-a", time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("claim job: ok=%v err=%v", ok, err)
	}
	store.mu.Lock()
	current := store.jobs[locked.ID]
	expired := time.Now().UTC().Add(-1 * time.Second)
	current.LeaseExpiresAt = &expired
	store.jobs[locked.ID] = current
	store.mu.Unlock()

	recovered, err := store.RecoverExpiredLeases(time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("recover leases: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 recovered job, got %d", recovered)
	}
	updated, err := store.GetJob(job.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if updated.Status != JobStatusQueued {
		t.Fatalf("expected queued after recovery, got %s", updated.Status)
	}
	if updated.LeaseExpiresAt != nil {
		t.Fatalf("expected lease to be cleared after recovery")
	}
}
