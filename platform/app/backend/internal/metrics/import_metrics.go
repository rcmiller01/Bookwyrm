package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	ImportJobsCreatedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "import_jobs_created_total",
		Help: "Total number of import jobs created.",
	})
	ImportJobsImportedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "import_jobs_imported_total",
		Help: "Total number of import jobs marked as imported.",
	})
	ImportJobsNeedsReviewTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "import_jobs_needs_review_total",
		Help: "Total number of import jobs marked as needs_review.",
	})
	ImportJobsFailedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "import_jobs_failed_total",
		Help: "Total number of import jobs marked as failed.",
	})
	ImportJobsSkippedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "import_jobs_skipped_total",
		Help: "Total number of import jobs marked as skipped.",
	})
	ImportJobDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "import_job_duration_seconds",
		Help:    "Duration from import job creation to terminal state.",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	prometheus.MustRegister(
		ImportJobsCreatedTotal,
		ImportJobsImportedTotal,
		ImportJobsNeedsReviewTotal,
		ImportJobsFailedTotal,
		ImportJobsSkippedTotal,
		ImportJobDurationSeconds,
	)
}

func ObserveImportTerminalDuration(createdAt time.Time) {
	if createdAt.IsZero() {
		return
	}
	seconds := time.Since(createdAt).Seconds()
	if seconds < 0 {
		seconds = 0
	}
	ImportJobDurationSeconds.Observe(seconds)
}
