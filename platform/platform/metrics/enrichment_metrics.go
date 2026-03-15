package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	EnrichmentJobsEnqueuedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "enrichment_jobs_enqueued_total",
		Help: "Total number of enrichment jobs enqueued, labeled by job_type.",
	}, []string{"job_type"})

	EnrichmentJobsStartedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "enrichment_jobs_started_total",
		Help: "Total number of enrichment jobs started, labeled by job_type.",
	}, []string{"job_type"})

	EnrichmentJobsSucceededTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "enrichment_jobs_succeeded_total",
		Help: "Total number of enrichment jobs succeeded, labeled by job_type.",
	}, []string{"job_type"})

	EnrichmentJobsFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "enrichment_jobs_failed_total",
		Help: "Total number of enrichment jobs failed (retryable), labeled by job_type.",
	}, []string{"job_type"})

	EnrichmentJobsDeadTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "enrichment_jobs_dead_total",
		Help: "Total number of enrichment jobs marked dead, labeled by job_type.",
	}, []string{"job_type"})

	EnrichmentJobDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "enrichment_job_duration_seconds",
		Help:    "Enrichment job execution duration in seconds, labeled by job_type.",
		Buckets: prometheus.DefBuckets,
	}, []string{"job_type"})

	EnrichmentJobBackoffSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "enrichment_job_backoff_seconds",
		Help:    "Scheduled backoff delay in seconds for failed enrichment jobs, labeled by job_type.",
		Buckets: []float64{1, 2, 5, 10, 30, 60, 120, 300, 600, 1800, 3600, 21600},
	}, []string{"job_type"})

	EnrichmentQueueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "enrichment_queue_depth",
		Help: "Current enrichment queue depth, labeled by status.",
	}, []string{"status"})

	EnrichmentWorkersActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "enrichment_workers_active",
		Help: "Number of enrichment workers currently executing a job.",
	})

	EnrichmentWorkersTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "enrichment_workers_total",
		Help: "Configured number of enrichment workers.",
	})

	EnrichmentWorkerIdleLoopsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "enrichment_worker_idle_loops_total",
		Help: "Total number of worker loops that found no available jobs.",
	})
)
