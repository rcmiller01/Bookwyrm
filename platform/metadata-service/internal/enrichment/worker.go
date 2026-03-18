package enrichment

import (
	"context"
	"time"

	"metadata-service/internal/enrichment/handlers"
	"metadata-service/internal/metrics"
	"metadata-service/internal/model"
	"metadata-service/internal/store"

	"github.com/rs/zerolog/log"
)

// Worker polls the enrichment queue, executes jobs, and records outcomes.
type Worker struct {
	id       string
	store    store.EnrichmentJobStore
	handlers *handlers.Registry
}

func NewWorker(id string, jobStore store.EnrichmentJobStore, registry *handlers.Registry) *Worker {
	return &Worker{id: id, store: jobStore, handlers: registry}
}

func (w *Worker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, err := w.store.TryLockNextJob(ctx, w.id)
		if err != nil {
			if err == store.ErrNoAvailableEnrichmentJobs {
				metrics.EnrichmentWorkerIdleLoopsTotal.Inc()
				time.Sleep(300 * time.Millisecond)
				continue
			}
			log.Error().Err(err).Str("worker", w.id).Msg("failed to lock enrichment job")
			time.Sleep(time.Second)
			continue
		}

		metrics.EnrichmentJobsStartedTotal.WithLabelValues(job.JobType).Inc()
		metrics.EnrichmentWorkersActive.Inc()
		startedAt := time.Now()

		runID, err := w.store.RecordRunStart(ctx, job.ID)
		if err != nil {
			log.Error().Err(err).Int64("job_id", job.ID).Msg("failed to record enrichment run start")
			_ = w.store.MarkFailed(ctx, job.ID, job.JobType, err.Error(), time.Second)
			metrics.EnrichmentWorkersActive.Dec()
			metrics.EnrichmentJobsFailedTotal.WithLabelValues(job.JobType).Inc()
			metrics.EnrichmentJobDurationSeconds.WithLabelValues(job.JobType).Observe(time.Since(startedAt).Seconds())
			continue
		}

		handleErr := w.handlers.Handle(ctx, *job)
		if handleErr != nil {
			_ = w.store.RecordRunFinish(ctx, runID, model.EnrichmentOutcomeFailed, handleErr.Error())
			_ = w.store.MarkFailed(ctx, job.ID, job.JobType, handleErr.Error(), time.Second)
			updated, getErr := w.store.GetJobByID(ctx, job.ID)
			if getErr == nil && updated.Status == model.EnrichmentStatusDead {
				metrics.EnrichmentJobsDeadTotal.WithLabelValues(job.JobType).Inc()
			} else {
				metrics.EnrichmentJobsFailedTotal.WithLabelValues(job.JobType).Inc()
			}
			metrics.EnrichmentWorkersActive.Dec()
			metrics.EnrichmentJobDurationSeconds.WithLabelValues(job.JobType).Observe(time.Since(startedAt).Seconds())
			log.Warn().Err(handleErr).Int64("job_id", job.ID).Str("job_type", job.JobType).Msg("enrichment job failed")
			continue
		}

		_ = w.store.RecordRunFinish(ctx, runID, model.EnrichmentOutcomeSucceeded, "")
		_ = w.store.MarkSucceeded(ctx, job.ID)
		metrics.EnrichmentJobsSucceededTotal.WithLabelValues(job.JobType).Inc()
		metrics.EnrichmentWorkersActive.Dec()
		metrics.EnrichmentJobDurationSeconds.WithLabelValues(job.JobType).Observe(time.Since(startedAt).Seconds())
		log.Debug().Int64("job_id", job.ID).Str("job_type", job.JobType).Msg("enrichment job succeeded")
	}
}
