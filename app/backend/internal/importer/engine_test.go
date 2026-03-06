package importer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/integration/metadata"
)

func TestEngineImportsCompletedDownloadIntoFinalLayout(t *testing.T) {
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
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
		MaxPathLen:              240,
		ReplaceColon:            true,
		KeepIncoming:            true,
	}, importStore, downloadStore, nil)

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
	finalFile := filepath.Join(libraryRoot, "Unknown Author", "Dune", "Dune.epub")
	if _, err := os.Stat(finalFile); err != nil {
		t.Fatalf("expected moved file at %s: %v", finalFile, err)
	}
	updatedDownload, err := downloadStore.GetJob(dj.ID)
	if err != nil {
		t.Fatalf("get download job: %v", err)
	}
	if !updatedDownload.Imported {
		t.Fatalf("expected download job imported=true")
	}
}

func TestEngineCollisionDifferentFileNeedsReview(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	sourceRoot := filepath.Join(t.TempDir(), "completed", "job2")
	mustWriteFileEngine(t, filepath.Join(sourceRoot, "Dune.epub"), "new-content-longer")
	existing := filepath.Join(libraryRoot, "Unknown Author", "Dune", "Dune.epub")
	mustWriteFileEngine(t, existing, "old")

	downloadStore := downloadqueue.NewStore()
	dj, _ := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      2,
		CandidateID: 2,
		WorkID:      "work-2",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	_ = downloadStore.UpdateProgress(dj.ID, downloadqueue.JobStatusCompleted, sourceRoot, "")

	importStore := NewMemoryStore()
	engine := NewEngine(Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
	}, importStore, downloadStore, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.Start(ctx)

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		items := importStore.ListJobs(JobFilter{Status: JobStatusNeedsReview, Limit: 10})
		if len(items) > 0 {
			return
		}
		time.Sleep(120 * time.Millisecond)
	}
	t.Fatalf("expected needs_review job after collision")
}

func TestEngineKeepIncomingFalseCleansStagingAfterSuccess(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	sourceRoot := filepath.Join(t.TempDir(), "completed", "job4")
	mustWriteFileEngine(t, filepath.Join(sourceRoot, "Dune.epub"), "content")

	downloadStore := downloadqueue.NewStore()
	dj, _ := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      4,
		CandidateID: 4,
		WorkID:      "work-4",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	_ = downloadStore.UpdateProgress(dj.ID, downloadqueue.JobStatusCompleted, sourceRoot, "")

	importStore := NewMemoryStore()
	engine := NewEngine(Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
		KeepIncoming:            false,
	}, importStore, downloadStore, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.Start(ctx)

	imported := waitForImportStatus(t, importStore, JobStatusImported, 8*time.Second)
	if imported.ID == 0 {
		t.Fatalf("expected imported job")
	}
	incomingDir := filepath.Join(libraryRoot, "_incoming", itoa64(dj.ID))
	if _, err := os.Stat(incomingDir); !os.IsNotExist(err) {
		t.Fatalf("expected incoming dir to be deleted, stat err=%v", err)
	}
}

func TestEngineKeepIncomingFalsePreservesStagingOnNeedsReview(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	sourceRoot := filepath.Join(t.TempDir(), "completed", "job5")
	mustWriteFileEngine(t, filepath.Join(sourceRoot, "Dune.epub"), "new-content-longer")
	existing := filepath.Join(libraryRoot, "Unknown Author", "Dune", "Dune.epub")
	mustWriteFileEngine(t, existing, "old")

	downloadStore := downloadqueue.NewStore()
	dj, _ := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      5,
		CandidateID: 5,
		WorkID:      "work-5",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	_ = downloadStore.UpdateProgress(dj.ID, downloadqueue.JobStatusCompleted, sourceRoot, "")

	importStore := NewMemoryStore()
	engine := NewEngine(Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
		KeepIncoming:            false,
	}, importStore, downloadStore, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.Start(ctx)

	review := waitForImportStatus(t, importStore, JobStatusNeedsReview, 8*time.Second)
	if review.ID == 0 {
		t.Fatalf("expected needs_review job")
	}
	incomingDir := filepath.Join(libraryRoot, "_incoming", itoa64(dj.ID))
	if _, err := os.Stat(incomingDir); err != nil {
		t.Fatalf("expected incoming dir to remain for debugging, err=%v", err)
	}
}

func TestEngineAmbiguousMatchNeedsReviewThenApproveImports(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	sourceRoot := filepath.Join(t.TempDir(), "completed", "job3")
	mustWriteFileEngine(t, filepath.Join(sourceRoot, "Some.Random.Book.epub"), "content")

	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/search" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"works":[{"id":"w-a","title":"Another Book"},{"id":"w-b","title":"Different Title"}]}`))
	}))
	defer meta.Close()

	downloadStore := downloadqueue.NewStore()
	dj, _ := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      3,
		CandidateID: 3,
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	_ = downloadStore.UpdateProgress(dj.ID, downloadqueue.JobStatusCompleted, sourceRoot, "")

	importStore := NewMemoryStore()
	engine := NewEngine(Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
	}, importStore, downloadStore, metadata.NewClient(metadata.Config{BaseURL: meta.URL, Timeout: time.Second}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.Start(ctx)

	job := waitForImportStatus(t, importStore, JobStatusNeedsReview, 8*time.Second)
	if job.ID == 0 {
		t.Fatalf("expected needs_review job")
	}
	if err := importStore.Approve(job.ID, "work-approved", "", ""); err != nil {
		t.Fatalf("approve job: %v", err)
	}

	imported := waitForImportStatus(t, importStore, JobStatusImported, 8*time.Second)
	if imported.ID != job.ID {
		t.Fatalf("expected same job to import; got needs_review=%d imported=%d", job.ID, imported.ID)
	}
	finalFile := filepath.Join(libraryRoot, "Unknown Author", "Some.Random.Book", "Some.Random.Book.epub")
	if _, err := os.Stat(finalFile); err != nil {
		t.Fatalf("expected moved file at %s: %v", finalFile, err)
	}
}

func TestEngineAudiobookFolderCollision_IdempotentThenNeedsReview(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	importStore := NewMemoryStore()
	downloadStore := downloadqueue.NewStore()

	// First import establishes target audiobook folder with two tracks.
	srcA := filepath.Join(t.TempDir(), "completed", "audiobookA")
	mustWriteFileEngine(t, filepath.Join(srcA, "Track 01.mp3"), "aaa")
	mustWriteFileEngine(t, filepath.Join(srcA, "Track 02.mp3"), "bbb")
	jobA, _ := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      201,
		CandidateID: 201,
		WorkID:      "work-audio",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	_ = downloadStore.UpdateProgress(jobA.ID, downloadqueue.JobStatusCompleted, srcA, "")

	engine := NewEngine(Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
	}, importStore, downloadStore, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.Start(ctx)
	_ = waitForImportStatus(t, importStore, JobStatusImported, 8*time.Second)

	// Second import, same filenames and sizes: should idempotently import.
	srcB := filepath.Join(t.TempDir(), "completed", "audiobookB")
	mustWriteFileEngine(t, filepath.Join(srcB, "Track 01.mp3"), "aaa")
	mustWriteFileEngine(t, filepath.Join(srcB, "Track 02.mp3"), "bbb")
	jobB, _ := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      202,
		CandidateID: 202,
		WorkID:      "work-audio",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	_ = downloadStore.UpdateProgress(jobB.ID, downloadqueue.JobStatusCompleted, srcB, "")
	importedAgain := waitForImportStatus(t, importStore, JobStatusImported, 8*time.Second)
	if importedAgain.ID == 0 {
		t.Fatalf("expected idempotent audiobook re-import to import successfully")
	}

	// Third import, same path but one track differs: should require review.
	srcC := filepath.Join(t.TempDir(), "completed", "audiobookC")
	mustWriteFileEngine(t, filepath.Join(srcC, "Track 01.mp3"), "aaa")
	mustWriteFileEngine(t, filepath.Join(srcC, "Track 02.mp3"), "different-size-content")
	jobC, _ := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      203,
		CandidateID: 203,
		WorkID:      "work-audio",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	_ = downloadStore.UpdateProgress(jobC.ID, downloadqueue.JobStatusCompleted, srcC, "")
	review := waitForImportStatus(t, importStore, JobStatusNeedsReview, 8*time.Second)
	if review.ID == 0 {
		t.Fatalf("expected needs_review on conflicting audiobook folder collision")
	}
}

func TestMoveOrCopy_FallbackOnCrossDeviceRename(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src.epub")
	dst := filepath.Join(root, "dst.epub")
	mustWriteFileEngine(t, src, "copy-via-exdev")

	engine := NewEngine(Config{
		LibraryRoot:          root,
		AllowCrossDeviceMove: true,
	}, NewMemoryStore(), downloadqueue.NewStore(), nil)
	engine.renameFn = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: syscall.EXDEV}
	}

	if err := engine.moveOrCopy(src, dst); err != nil {
		t.Fatalf("expected cross-device fallback to succeed, got %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("expected destination file to exist: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("expected source file removed after verified copy, err=%v", err)
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

func waitForImportStatus(t *testing.T, store Store, status JobStatus, timeout time.Duration) Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		items := store.ListJobs(JobFilter{Status: status, Limit: 10})
		if len(items) > 0 {
			return items[0]
		}
		time.Sleep(120 * time.Millisecond)
	}
	return Job{}
}

func itoa64(v int64) string {
	return fmt.Sprintf("%d", v)
}
