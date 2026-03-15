package importer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	events := importStore.ListEvents(imported.ID)
	if len(events) == 0 {
		t.Fatalf("expected import events to be recorded")
	}
	payload := events[len(events)-1].Payload
	if payload["import_job_id"] != imported.ID {
		t.Fatalf("expected import_job_id correlation field")
	}
	if payload["download_job_id"] != dj.ID {
		t.Fatalf("expected download_job_id correlation field")
	}
	if payload["work_id"] != "work-1" {
		t.Fatalf("expected work_id correlation field")
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

func TestEngineReconcileIncomingOrphansCreatesNeedsReviewJob(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	downloadStore := downloadqueue.NewStore()
	importStore := NewMemoryStore()

	dj, err := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      301,
		CandidateID: 301,
		WorkID:      "work-orphan",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create download job: %v", err)
	}

	orphanDir := filepath.Join(libraryRoot, "_incoming", itoa64(dj.ID))
	mustWriteFileEngine(t, filepath.Join(orphanDir, "leftover.epub"), "orphan-content")

	engine := NewEngine(Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
	}, importStore, downloadStore, nil)

	reconciled, err := engine.reconcileIncomingOrphans(time.Now().UTC())
	if err != nil {
		t.Fatalf("reconcile incoming orphans failed: %v", err)
	}
	if reconciled != 1 {
		t.Fatalf("expected 1 reconciled orphan, got %d", reconciled)
	}

	items := importStore.ListJobs(JobFilter{Status: JobStatusNeedsReview, Limit: 10})
	if len(items) != 1 {
		t.Fatalf("expected one needs_review import job, got %d", len(items))
	}
	job := items[0]
	if job.DownloadJobID != dj.ID {
		t.Fatalf("expected download_job_id %d, got %d", dj.ID, job.DownloadJobID)
	}
	if filepath.Clean(job.SourcePath) != filepath.Clean(orphanDir) {
		t.Fatalf("expected source path %s, got %s", orphanDir, job.SourcePath)
	}
	if !strings.Contains(strings.ToLower(job.LastError), "orphan incoming") {
		t.Fatalf("expected orphan incoming last_error, got %q", job.LastError)
	}

	second, err := engine.reconcileIncomingOrphans(time.Now().UTC())
	if err != nil {
		t.Fatalf("second reconcile incoming orphans failed: %v", err)
	}
	if second != 0 {
		t.Fatalf("expected idempotent second reconciliation, got %d", second)
	}
}

func TestEngineDecideKeepBothCreatesCopyAndImports(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	original := filepath.Join(libraryRoot, "Unknown Author", "Dune", "Dune.epub")
	mustWriteFileEngine(t, original, "old-content")

	downloadStore := downloadqueue.NewStore()
	dj, _ := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      410,
		CandidateID: 410,
		WorkID:      "work-410",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	importStore := NewMemoryStore()
	job, _ := importStore.CreateOrGetFromDownload(dj, libraryRoot)

	incomingDir := filepath.Join(libraryRoot, "_incoming", itoa64(dj.ID))
	mustWriteFileEngine(t, filepath.Join(incomingDir, "Dune.epub"), "new-content")
	_ = importStore.MarkNeedsReview(job.ID, "collision", map[string]any{}, map[string]any{"collision": map[string]any{"target_path": original}})

	engine := NewEngine(Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
	}, importStore, downloadStore, nil)

	if err := engine.Decide(job.ID, DecisionKeepBoth); err != nil {
		t.Fatalf("decide keep_both failed: %v", err)
	}

	copyPath := filepath.Join(libraryRoot, "Unknown Author", "Dune", "Dune (copy).epub")
	if _, err := os.Stat(copyPath); err != nil {
		t.Fatalf("expected copied file at %s: %v", copyPath, err)
	}
	updated, _ := importStore.GetJob(job.ID)
	if updated.Status != JobStatusImported {
		t.Fatalf("expected imported status, got %s", updated.Status)
	}
}

func TestEngineDecideReplaceExistingMovesOldToTrash(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	original := filepath.Join(libraryRoot, "Unknown Author", "Dune", "Dune.epub")
	mustWriteFileEngine(t, original, "old-content")

	downloadStore := downloadqueue.NewStore()
	dj, _ := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      411,
		CandidateID: 411,
		WorkID:      "work-411",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	importStore := NewMemoryStore()
	job, _ := importStore.CreateOrGetFromDownload(dj, libraryRoot)

	incomingDir := filepath.Join(libraryRoot, "_incoming", itoa64(dj.ID))
	mustWriteFileEngine(t, filepath.Join(incomingDir, "Dune.epub"), "new-content")
	_ = importStore.MarkNeedsReview(job.ID, "collision", map[string]any{}, map[string]any{"collision": map[string]any{"target_path": original}})

	engine := NewEngine(Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
	}, importStore, downloadStore, nil)

	if err := engine.Decide(job.ID, DecisionReplaceExisting); err != nil {
		t.Fatalf("decide replace_existing failed: %v", err)
	}
	content, err := os.ReadFile(original)
	if err != nil {
		t.Fatalf("expected replaced target to exist: %v", err)
	}
	if string(content) != "new-content" {
		t.Fatalf("expected target content to be replaced")
	}
	trashDir := filepath.Join(libraryRoot, "_trash")
	entries, err := os.ReadDir(trashDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected old file moved into trash dir")
	}
}

func TestEngineUniqueTrashPathAvoidsCollisions(t *testing.T) {
	root := t.TempDir()
	trashDir := filepath.Join(root, "_trash")
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		t.Fatalf("mkdir trash dir: %v", err)
	}
	engine := NewEngine(Config{LibraryRoot: root, AllowCrossDeviceMove: true}, NewMemoryStore(), downloadqueue.NewStore(), nil)

	first := engine.uniqueTrashPath(trashDir, "Dune.epub")
	mustWriteFileEngine(t, first, "existing-trash")
	second := engine.uniqueTrashPath(trashDir, "Dune.epub")
	if filepath.Clean(first) == filepath.Clean(second) {
		t.Fatalf("expected unique trash path when first candidate exists")
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

func TestMoveOrCopy_FallbackOnWindowsCrossVolumeRename(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src.epub")
	dst := filepath.Join(root, "dst.epub")
	mustWriteFileEngine(t, src, "copy-via-windows-cross-volume")

	engine := NewEngine(Config{
		LibraryRoot:          root,
		AllowCrossDeviceMove: true,
	}, NewMemoryStore(), downloadqueue.NewStore(), nil)
	engine.renameFn = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: fmt.Errorf("The system cannot move the file to a different disk drive.")}
	}

	if err := engine.moveOrCopy(src, dst); err != nil {
		t.Fatalf("expected windows cross-volume fallback to succeed, got %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("expected destination file to exist: %v", err)
	}
}

func TestMatchJobWithWorkIDHandlesAuthorTitleFilenames(t *testing.T) {
	sourceRoot := filepath.Join(t.TempDir(), "completed", "job-author-title")
	mustWriteFileEngine(t, filepath.Join(sourceRoot, "Rebecca Yarros - Fourth Wing (retail) (epub).epub"), "content")

	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/work/wrk-fourth-wing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"work":{"id":"wrk-fourth-wing","title":"Fourth Wing"}}`))
	}))
	defer meta.Close()

	engine := NewEngine(Config{LibraryRoot: t.TempDir(), AllowCrossDeviceMove: true}, NewMemoryStore(), downloadqueue.NewStore(), metadata.NewClient(metadata.Config{BaseURL: meta.URL, Timeout: time.Second}))
	files, err := ScanMediaFiles(sourceRoot, 10)
	if err != nil {
		t.Fatalf("scan media files: %v", err)
	}
	job, confidence, _ := engine.matchJob(Job{WorkID: "wrk-fourth-wing", SourcePath: sourceRoot}, files)
	if confidence < 0.85 {
		t.Fatalf("expected confident work-id match, got %f", confidence)
	}
	if job.WorkID != "wrk-fourth-wing" {
		t.Fatalf("expected work id to be preserved, got %s", job.WorkID)
	}
}

func TestResolveNamingValuesFallsBackToReleaseAuthorWhenMetadataIsSparse(t *testing.T) {
	sourceRoot := filepath.Join(t.TempDir(), "completed", "job-fourth-wing")
	mustWriteFileEngine(t, filepath.Join(sourceRoot, "Rebecca Yarros - Fourth Wing (retail) (epub).epub"), "content")

	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/work/wrk-fourth-wing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"work":{"id":"wrk-fourth-wing","title":"Fourth Wing","first_pub_year":2024}}`))
	}))
	defer meta.Close()

	engine := NewEngine(Config{LibraryRoot: "H:\\Books", AllowCrossDeviceMove: true}, NewMemoryStore(), downloadqueue.NewStore(), metadata.NewClient(metadata.Config{BaseURL: meta.URL, Timeout: time.Second}))
	files, err := ScanMediaFiles(sourceRoot, 10)
	if err != nil {
		t.Fatalf("scan media files: %v", err)
	}
	values := engine.resolveNamingValues(Job{WorkID: "wrk-fourth-wing", SourcePath: sourceRoot}, files)
	if values.Author != "Rebecca Yarros" {
		t.Fatalf("expected release author fallback, got %q", values.Author)
	}
	if values.Title != "Fourth Wing" {
		t.Fatalf("expected metadata title, got %q", values.Title)
	}
	if values.Year != "2024" {
		t.Fatalf("expected publication year, got %q", values.Year)
	}
}

func TestResolveNamingValuesUsesAudiobookFolderHints(t *testing.T) {
	sourceRoot := filepath.Join(t.TempDir(), "completed", "job-project-hail-mary")
	trackDir := filepath.Join(sourceRoot, "Andy Weir - 2021 - Project Hail Mary")
	mustWriteFileEngine(t, filepath.Join(trackDir, "Project Hail Mary (01).mp3"), "audio")

	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/work/wrk-project-hail-mary" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"work":{"id":"wrk-project-hail-mary","title":"Project Hail Mary","first_pub_year":2021}}`))
	}))
	defer meta.Close()

	engine := NewEngine(Config{LibraryRoot: "H:\\Books", AllowCrossDeviceMove: true}, NewMemoryStore(), downloadqueue.NewStore(), metadata.NewClient(metadata.Config{BaseURL: meta.URL, Timeout: time.Second}))
	files, err := ScanMediaFiles(sourceRoot, 10)
	if err != nil {
		t.Fatalf("scan media files: %v", err)
	}
	values := engine.resolveNamingValues(Job{WorkID: "wrk-project-hail-mary", SourcePath: sourceRoot}, files)
	if values.Author != "Andy Weir" {
		t.Fatalf("expected audiobook folder author fallback, got %q", values.Author)
	}
	if values.Title != "Project Hail Mary" {
		t.Fatalf("expected metadata title, got %q", values.Title)
	}
	if values.Year != "2021" {
		t.Fatalf("expected year from metadata/folder, got %q", values.Year)
	}
}
func TestProcessJobMissingSourceReturnsHelpfulError(t *testing.T) {
	engine := NewEngine(Config{LibraryRoot: t.TempDir(), AllowCrossDeviceMove: true}, NewMemoryStore(), downloadqueue.NewStore(), nil)
	missingPath := filepath.Join(t.TempDir(), "missing-source")
	err := engine.processJob(context.Background(), Job{SourcePath: missingPath, TargetRoot: t.TempDir(), DownloadJobID: 1})
	if err == nil {
		t.Fatalf("expected missing source error")
	}
	if !strings.Contains(err.Error(), "source path no longer exists") {
		t.Fatalf("expected helpful missing source message, got %v", err)
	}
	if !strings.Contains(err.Error(), "retry the download") {
		t.Fatalf("expected retry hint in error, got %v", err)
	}
}
func TestCleanupIncomingRemovesOldUnreferencedDirectories(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	downloadStore := downloadqueue.NewStore()
	importStore := NewMemoryStore()
	engine := NewEngine(Config{
		LibraryRoot:          libraryRoot,
		AllowCrossDeviceMove: true,
		KeepIncoming:         true,
		KeepIncomingDays:     1,
	}, importStore, downloadStore, nil)

	oldDir := filepath.Join(libraryRoot, "_incoming", "99999")
	mustWriteFileEngine(t, filepath.Join(oldDir, "old.epub"), "stale")
	oldTime := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(oldDir, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes oldDir: %v", err)
	}

	removed, err := engine.cleanupIncoming(time.Now().UTC())
	if err != nil {
		t.Fatalf("cleanup incoming failed: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected one removed incoming directory, got %d", removed)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("expected old incoming directory removed")
	}
}

func TestCleanupTrashRemovesOldFiles(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	engine := NewEngine(Config{
		LibraryRoot:          libraryRoot,
		AllowCrossDeviceMove: true,
		KeepTrashDays:        1,
	}, NewMemoryStore(), downloadqueue.NewStore(), nil)

	trashDir := filepath.Join(libraryRoot, "_trash")
	oldFile := filepath.Join(trashDir, "old.epub")
	mustWriteFileEngine(t, oldFile, "old")
	oldTime := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes oldFile: %v", err)
	}

	removed, err := engine.cleanupTrash(time.Now().UTC())
	if err != nil {
		t.Fatalf("cleanup trash failed: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected one removed trash artifact, got %d", removed)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old trash file removed")
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
