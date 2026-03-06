package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
)

func TestEngineImportsCompletedDownloadIntoIncoming(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	sourceRoot := filepath.Join(t.TempDir(), "completed", "job1")
	mustWriteFileEngine(t, filepath.Join(sourceRoot, "Dune.epub"), "content")

	downloadStore := downloadqueue.NewStore()
	dj, err := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      1,
		CandidateID: 1,
		WorkID:      "work-1",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create download job: %v", err)
	}
	if err := downloadStore.UpdateProgress(dj.ID, downloadqueue.JobStatusCompleted, sourceRoot, ""); err != nil {
		t.Fatalf("mark completed: %v", err)
	}

	importStore := NewMemoryStore()
	engine := NewEngine(Config{
		LibraryRoot:          libraryRoot,
		AllowCrossDeviceMove: true,
		MaxScanFiles:         100,
	}, importStore, downloadStore)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.Start(ctx)

	deadline := time.Now().Add(6 * time.Second)
	var imported Job
	for time.Now().Before(deadline) {
		items := importStore.ListJobs(JobFilter{Status: JobStatusImported, Limit: 10})
		if len(items) > 0 {
			imported = items[0]
			break
		}
		time.Sleep(120 * time.Millisecond)
	}
	if imported.ID == 0 {
		t.Fatalf("expected imported job")
	}
	incomingFile := filepath.Join(libraryRoot, "_incoming", "1", "Dune.epub")
	if _, err := os.Stat(incomingFile); err != nil {
		t.Fatalf("expected moved file at %s: %v", incomingFile, err)
	}
	updatedDownload, err := downloadStore.GetJob(dj.ID)
	if err != nil {
		t.Fatalf("get download job: %v", err)
	}
	if !updatedDownload.Imported {
		t.Fatalf("expected download job imported=true")
	}
}

func mustWriteFileEngine(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
}
