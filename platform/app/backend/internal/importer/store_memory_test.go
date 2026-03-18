package importer

import (
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
)

func TestMemoryStoreClaimAndRescheduleClearsLease(t *testing.T) {
	store := NewMemoryStore()

	created, err := store.CreateOrGetFromDownload(downloadqueue.Job{
		ID:         10,
		WorkID:     "work-1",
		EditionID:  "ed-1",
		OutputPath: "/tmp/incoming/file.epub",
	}, "/library")
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}

	claimed, ok, err := store.ClaimNextQueued("worker-a", time.Now().UTC())
	if err != nil {
		t.Fatalf("claim import job: %v", err)
	}
	if !ok {
		t.Fatalf("expected queued import job to be claimed")
	}
	if claimed.ID != created.ID {
		t.Fatalf("expected claimed id %d, got %d", created.ID, claimed.ID)
	}
	if claimed.LockedAt == nil || claimed.LeaseExpiresAt == nil {
		t.Fatalf("expected lock and lease timestamps to be set")
	}
	if !claimed.LeaseExpiresAt.After(*claimed.LockedAt) {
		t.Fatalf("expected lease expiry after lock timestamp")
	}

	if err := store.MarkFailed(created.ID, "temporary", false); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	updated, err := store.GetJob(created.ID)
	if err != nil {
		t.Fatalf("get import job: %v", err)
	}
	if updated.Status != JobStatusQueued {
		t.Fatalf("expected queued after non-terminal failure, got %s", updated.Status)
	}
	if updated.LeaseExpiresAt != nil || updated.LockedAt != nil || updated.LockedBy != "" {
		t.Fatalf("expected lease and lock metadata cleared after reschedule")
	}
}

func TestMemoryStoreRecoverExpiredLeasesRequeuesRunningJobs(t *testing.T) {
	store := NewMemoryStore()
	created, err := store.CreateOrGetFromDownload(downloadqueue.Job{
		ID:         11,
		WorkID:     "work-2",
		EditionID:  "ed-2",
		OutputPath: "/tmp/incoming/file2.epub",
	}, "/library")
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}
	_, ok, err := store.ClaimNextQueued("worker-a", time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("claim import job: ok=%v err=%v", ok, err)
	}

	store.mu.Lock()
	job := store.jobsByID[created.ID]
	expired := time.Now().UTC().Add(-1 * time.Second)
	job.LeaseExpiresAt = &expired
	store.jobsByID[created.ID] = job
	store.mu.Unlock()

	recovered, err := store.RecoverExpiredLeases(time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("recover leases: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 recovered job, got %d", recovered)
	}
	updated, err := store.GetJob(created.ID)
	if err != nil {
		t.Fatalf("get import job: %v", err)
	}
	if updated.Status != JobStatusQueued {
		t.Fatalf("expected queued after recovery, got %s", updated.Status)
	}
	if updated.LeaseExpiresAt != nil || updated.LockedAt != nil || updated.LockedBy != "" {
		t.Fatalf("expected lease and lock metadata cleared after recovery")
	}
}

func TestMemoryStoreExistsDownloadJob(t *testing.T) {
	store := NewMemoryStore()
	if store.ExistsDownloadJob(999) {
		t.Fatalf("expected nonexistent download job to return false")
	}
	created, err := store.CreateOrGetFromDownload(downloadqueue.Job{
		ID:         15,
		WorkID:     "work-exists",
		EditionID:  "ed-exists",
		OutputPath: "/tmp/incoming/exists.epub",
	}, "/library")
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("expected created import job id")
	}
	if !store.ExistsDownloadJob(15) {
		t.Fatalf("expected existing download job mapping to return true")
	}
}
