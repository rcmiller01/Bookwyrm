package store

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"metadata-service/internal/metrics"
	"metadata-service/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const maxEnrichmentBackoff = 6 * time.Hour

// ErrNoAvailableEnrichmentJobs indicates queue poll had no lockable jobs.
var ErrNoAvailableEnrichmentJobs = errors.New("no enrichment jobs available")

// EnrichmentJobStore manages enrichment job queue and execution history.
type EnrichmentJobStore interface {
	EnqueueJob(ctx context.Context, job model.EnrichmentJob) (int64, error)
	GetJobByID(ctx context.Context, id int64) (*model.EnrichmentJob, error)
	TryLockNextJob(ctx context.Context, workerID string) (*model.EnrichmentJob, error)
	MarkSucceeded(ctx context.Context, jobID int64) error
	MarkFailed(ctx context.Context, jobID int64, jobType string, errMsg string, backoff time.Duration) error
	MarkDead(ctx context.Context, jobID int64, errMsg string) error
	RecordRunStart(ctx context.Context, jobID int64) (int64, error)
	RecordRunFinish(ctx context.Context, runID int64, outcome string, errMsg string) error
	ListJobs(ctx context.Context, filters model.EnrichmentJobFilters) ([]model.EnrichmentJob, error)
	CountJobsByStatus(ctx context.Context) (map[string]int64, error)
	NextRunnableAt(ctx context.Context) (*time.Time, error)
}

type pgEnrichmentJobStore struct {
	db *pgxpool.Pool
}

func NewEnrichmentJobStore(db *pgxpool.Pool) EnrichmentJobStore {
	return &pgEnrichmentJobStore{db: db}
}

func (s *pgEnrichmentJobStore) EnqueueJob(ctx context.Context, job model.EnrichmentJob) (int64, error) {
	if job.Status == "" {
		job.Status = model.EnrichmentStatusQueued
	}
	if job.Priority == 0 {
		job.Priority = 100
	}
	if job.MaxAttempts == 0 {
		job.MaxAttempts = 5
	}

	var id int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO enrichment_jobs
		    (job_type, entity_type, entity_id, status, priority, max_attempts, not_before, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id`,
		job.JobType, job.EntityType, job.EntityID, model.EnrichmentStatusQueued,
		job.Priority, job.MaxAttempts, job.NotBefore,
	).Scan(&id)
	if err == nil {
		metrics.EnrichmentJobsEnqueuedTotal.WithLabelValues(job.JobType).Inc()
		return id, nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		// Duplicate queued/running job is treated as a no-op success.
		err = s.db.QueryRow(ctx, `
			SELECT id
			FROM enrichment_jobs
			WHERE job_type = $1
			  AND entity_type = $2
			  AND entity_id = $3
			  AND status IN ('queued', 'running')
			ORDER BY created_at DESC
			LIMIT 1`,
			job.JobType, job.EntityType, job.EntityID,
		).Scan(&id)
		if err == nil {
			return id, nil
		}
	}
	return 0, err
}

func (s *pgEnrichmentJobStore) GetJobByID(ctx context.Context, id int64) (*model.EnrichmentJob, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, job_type, entity_type, entity_id, status, priority, attempt_count,
		       max_attempts, not_before, locked_at, locked_by, last_error, created_at, updated_at
		FROM enrichment_jobs
		WHERE id = $1`, id)
	var job model.EnrichmentJob
	if err := row.Scan(
		&job.ID, &job.JobType, &job.EntityType, &job.EntityID, &job.Status,
		&job.Priority, &job.AttemptCount, &job.MaxAttempts, &job.NotBefore,
		&job.LockedAt, &job.LockedBy, &job.LastError, &job.CreatedAt, &job.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *pgEnrichmentJobStore) TryLockNextJob(ctx context.Context, workerID string) (*model.EnrichmentJob, error) {
	row := s.db.QueryRow(ctx, `
		WITH next_job AS (
			SELECT id
			FROM enrichment_jobs
			WHERE status = 'queued'
			  AND (not_before IS NULL OR not_before <= NOW())
			ORDER BY priority ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE enrichment_jobs AS j
		SET status = 'running',
		    locked_at = NOW(),
		    locked_by = $1,
		    updated_at = NOW()
		FROM next_job
		WHERE j.id = next_job.id
		RETURNING j.id, j.job_type, j.entity_type, j.entity_id, j.status, j.priority,
		          j.attempt_count, j.max_attempts, j.not_before, j.locked_at, j.locked_by,
		          j.last_error, j.created_at, j.updated_at`,
		workerID,
	)

	var job model.EnrichmentJob
	if err := row.Scan(
		&job.ID, &job.JobType, &job.EntityType, &job.EntityID, &job.Status,
		&job.Priority, &job.AttemptCount, &job.MaxAttempts, &job.NotBefore,
		&job.LockedAt, &job.LockedBy, &job.LastError, &job.CreatedAt, &job.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoAvailableEnrichmentJobs
		}
		return nil, err
	}

	return &job, nil
}

func (s *pgEnrichmentJobStore) MarkSucceeded(ctx context.Context, jobID int64) error {
	_, err := s.db.Exec(ctx, `
		UPDATE enrichment_jobs
		SET status = 'succeeded',
		    locked_at = NULL,
		    locked_by = NULL,
		    last_error = NULL,
		    updated_at = NOW()
		WHERE id = $1`,
		jobID,
	)
	return err
}

func (s *pgEnrichmentJobStore) MarkFailed(ctx context.Context, jobID int64, jobType string, errMsg string, backoff time.Duration) error {
	if backoff <= 0 {
		backoff = nextBackoff(1)
	}
	if backoff > maxEnrichmentBackoff {
		backoff = maxEnrichmentBackoff
	}
	metrics.EnrichmentJobBackoffSeconds.WithLabelValues(jobType).Observe(backoff.Seconds())

	var attempt int
	var notBefore *time.Time
	var status string
	err := s.db.QueryRow(ctx, `
		UPDATE enrichment_jobs
		SET attempt_count = attempt_count + 1,
		    status = CASE
		      WHEN attempt_count + 1 >= max_attempts THEN 'dead'
		      ELSE 'queued'
		    END,
		    not_before = CASE
		      WHEN attempt_count + 1 >= max_attempts THEN NULL
		      ELSE NOW() + ($2 * INTERVAL '1 millisecond')
		    END,
		    last_error = $3,
		    locked_at = NULL,
		    locked_by = NULL,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING attempt_count, not_before, status`,
		jobID, backoff.Milliseconds(), errMsg,
	).Scan(&attempt, &notBefore, &status)
	if err == nil {
		logger := log.Warn().
			Int64("job_id", jobID).
			Str("job_type", jobType).
			Int("attempt", attempt).
			Float64("backoff_seconds", backoff.Seconds())
		if notBefore != nil {
			logger = logger.Time("not_before", *notBefore)
		}
		logger.Str("next_status", status).Msg("enrichment job scheduled after failure")
	}
	return err
}

func (s *pgEnrichmentJobStore) MarkDead(ctx context.Context, jobID int64, errMsg string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE enrichment_jobs
		SET status = 'dead',
		    last_error = $2,
		    not_before = NULL,
		    locked_at = NULL,
		    locked_by = NULL,
		    updated_at = NOW()
		WHERE id = $1`,
		jobID, errMsg,
	)
	return err
}

func (s *pgEnrichmentJobStore) RecordRunStart(ctx context.Context, jobID int64) (int64, error) {
	var runID int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO enrichment_job_runs (job_id, started_at, outcome)
		VALUES ($1, NOW(), 'running')
		RETURNING id`,
		jobID,
	).Scan(&runID)
	if err != nil {
		return 0, err
	}
	return runID, nil
}

func (s *pgEnrichmentJobStore) RecordRunFinish(ctx context.Context, runID int64, outcome string, errMsg string) error {
	var errValue any
	if errMsg != "" {
		errValue = errMsg
	}
	_, err := s.db.Exec(ctx, `
		UPDATE enrichment_job_runs
		SET finished_at = NOW(),
		    outcome = $2,
		    error = $3
		WHERE id = $1`,
		runID, outcome, errValue,
	)
	return err
}

func (s *pgEnrichmentJobStore) ListJobs(ctx context.Context, filters model.EnrichmentJobFilters) ([]model.EnrichmentJob, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}

	var rows pgx.Rows
	var err error
	if filters.Status != "" {
		rows, err = s.db.Query(ctx, `
			SELECT id, job_type, entity_type, entity_id, status, priority, attempt_count,
			       max_attempts, not_before, locked_at, locked_by, last_error, created_at, updated_at
			FROM enrichment_jobs
			WHERE status = $1
			ORDER BY created_at DESC
			LIMIT $2`,
			filters.Status, limit,
		)
	} else {
		rows, err = s.db.Query(ctx, `
			SELECT id, job_type, entity_type, entity_id, status, priority, attempt_count,
			       max_attempts, not_before, locked_at, locked_by, last_error, created_at, updated_at
			FROM enrichment_jobs
			ORDER BY created_at DESC
			LIMIT $1`,
			limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.EnrichmentJob
	for rows.Next() {
		var job model.EnrichmentJob
		if scanErr := rows.Scan(
			&job.ID, &job.JobType, &job.EntityType, &job.EntityID, &job.Status,
			&job.Priority, &job.AttemptCount, &job.MaxAttempts, &job.NotBefore,
			&job.LockedAt, &job.LockedBy, &job.LastError, &job.CreatedAt, &job.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *pgEnrichmentJobStore) CountJobsByStatus(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.Query(ctx, `
		SELECT status, COUNT(*)
		FROM enrichment_jobs
		GROUP BY status`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int64{}
	for rows.Next() {
		var status string
		var count int64
		if scanErr := rows.Scan(&status, &count); scanErr != nil {
			return nil, scanErr
		}
		out[status] = count
	}
	return out, rows.Err()
}

func (s *pgEnrichmentJobStore) NextRunnableAt(ctx context.Context) (*time.Time, error) {
	var nextAt *time.Time
	err := s.db.QueryRow(ctx, `
		SELECT MIN(
			CASE
				WHEN not_before IS NULL OR not_before <= NOW() THEN NOW()
				ELSE not_before
			END
		)
		FROM enrichment_jobs
		WHERE status = 'queued'`,
	).Scan(&nextAt)
	if err != nil {
		return nil, err
	}
	return nextAt, nil
}

// nextBackoff returns exponential backoff with jitter, capped at maxEnrichmentBackoff.
func nextBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	base := time.Second * time.Duration(1<<minInt(attempt-1, 12))
	if base > maxEnrichmentBackoff {
		base = maxEnrichmentBackoff
	}
	jitter := time.Duration(rand.Int63n(int64(base / 5))) // up to 20%
	wait := base + jitter
	if wait > maxEnrichmentBackoff {
		return maxEnrichmentBackoff
	}
	return wait
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
