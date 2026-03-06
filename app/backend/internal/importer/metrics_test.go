package importer

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
	appmetrics "app-backend/internal/metrics"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestImportMetrics_IncrementOnSuccessfulLifecycle(t *testing.T) {
	createdBefore := testutil.ToFloat64(appmetrics.ImportJobsCreatedTotal)
	importedBefore := testutil.ToFloat64(appmetrics.ImportJobsImportedTotal)
	durationBefore := histogramSampleCount(t, "import_job_duration_seconds")

	libraryRoot := filepath.Join(t.TempDir(), "library")
	sourceRoot := filepath.Join(t.TempDir(), "completed", "job_metrics_success")
	mustWriteFileEngine(t, filepath.Join(sourceRoot, "MetricBook.epub"), "content")

	downloadStore := downloadqueue.NewStore()
	dj, _ := downloadStore.CreateJob(downloadqueue.Job{
		GrabID:      101,
		CandidateID: 101,
		WorkID:      "work-metrics-success",
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

	imported := waitForImportStatus(t, importStore, JobStatusImported, 8*time.Second)
	if imported.ID == 0 {
		t.Fatalf("expected imported job")
	}

	if got := testutil.ToFloat64(appmetrics.ImportJobsCreatedTotal) - createdBefore; got < 1 {
		t.Fatalf("expected import_jobs_created_total to increment, delta=%f", got)
	}
	if got := testutil.ToFloat64(appmetrics.ImportJobsImportedTotal) - importedBefore; got < 1 {
		t.Fatalf("expected import_jobs_imported_total to increment, delta=%f", got)
	}
	if got := histogramSampleCount(t, "import_job_duration_seconds") - durationBefore; got < 1 {
		t.Fatalf("expected import_job_duration_seconds observations, delta=%f", got)
	}
}

func TestImportMetrics_IncrementOnNeedsReviewAndFailed(t *testing.T) {
	needsReviewBefore := testutil.ToFloat64(appmetrics.ImportJobsNeedsReviewTotal)
	failedBefore := testutil.ToFloat64(appmetrics.ImportJobsFailedTotal)

	// needs_review path via collision
	{
		libraryRoot := filepath.Join(t.TempDir(), "library")
		sourceRoot := filepath.Join(t.TempDir(), "completed", "job_metrics_review")
		mustWriteFileEngine(t, filepath.Join(sourceRoot, "Dune.epub"), "new-content")
		mustWriteFileEngine(t, filepath.Join(libraryRoot, "Unknown Author", "Dune", "Dune.epub"), "old")

		downloadStore := downloadqueue.NewStore()
		dj, _ := downloadStore.CreateJob(downloadqueue.Job{
			GrabID:      102,
			CandidateID: 102,
			WorkID:      "work-metrics-review",
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
		engine.Start(ctx)
		_ = waitForImportStatus(t, importStore, JobStatusNeedsReview, 8*time.Second)
		cancel()
	}

	// failed path via unsupported cross-device move (rename fails with EXDEV and fallback disallowed)
	{
		libraryRoot := filepath.Join(t.TempDir(), "library")
		sourceRoot := filepath.Join(t.TempDir(), "completed", "job_metrics_failed")
		mustWriteFileEngine(t, filepath.Join(sourceRoot, "Broken.epub"), "content")
		downloadStore := downloadqueue.NewStore()
		dj, _ := downloadStore.CreateJob(downloadqueue.Job{
			GrabID:      103,
			CandidateID: 103,
			WorkID:      "work-metrics-failed",
			Protocol:    "usenet",
			ClientName:  "nzbget",
			MaxAttempts: 1,
			NotBefore:   time.Now().UTC(),
		})
		_ = downloadStore.UpdateProgress(dj.ID, downloadqueue.JobStatusCompleted, sourceRoot, "")
		importStore := NewMemoryStore()
		engine := NewEngine(Config{
			LibraryRoot:             libraryRoot,
			AllowCrossDeviceMove:    false,
			MaxScanFiles:            100,
			TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
			TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
			TemplateAudiobookFolder: "{Author}/{Title}",
		}, importStore, downloadStore, nil)
		engine.renameFn = func(src, dst string) error {
			return errors.New("forced rename failure")
		}
		ctx, cancel := context.WithCancel(context.Background())
		engine.Start(ctx)
		_ = waitForImportStatus(t, importStore, JobStatusFailed, 8*time.Second)
		cancel()
	}

	if got := testutil.ToFloat64(appmetrics.ImportJobsNeedsReviewTotal) - needsReviewBefore; got < 1 {
		t.Fatalf("expected import_jobs_needs_review_total to increment, delta=%f", got)
	}
	if got := testutil.ToFloat64(appmetrics.ImportJobsFailedTotal) - failedBefore; got < 1 {
		t.Fatalf("expected import_jobs_failed_total to increment, delta=%f", got)
	}
}

func TestImportMetrics_IncrementOnSkipped(t *testing.T) {
	skippedBefore := testutil.ToFloat64(appmetrics.ImportJobsSkippedTotal)
	s := NewMemoryStore()
	job, err := s.CreateOrGetFromDownload(downloadqueue.Job{
		ID:         9001,
		OutputPath: filepath.Join(t.TempDir(), "noop"),
	}, filepath.Join(t.TempDir(), "library"))
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := s.Skip(job.ID, "operator skip"); err != nil {
		t.Fatalf("skip job: %v", err)
	}
	if got := testutil.ToFloat64(appmetrics.ImportJobsSkippedTotal) - skippedBefore; got < 1 {
		t.Fatalf("expected import_jobs_skipped_total to increment, delta=%f", got)
	}
}

func histogramSampleCount(t *testing.T, metricName string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, fam := range families {
		if fam.GetName() != metricName {
			continue
		}
		if len(fam.GetMetric()) == 0 {
			return 0
		}
		h := fam.GetMetric()[0].GetHistogram()
		if h == nil {
			return 0
		}
		return float64(h.GetSampleCount())
	}
	return 0
}
