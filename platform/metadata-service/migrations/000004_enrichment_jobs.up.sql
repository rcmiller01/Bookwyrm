CREATE TABLE IF NOT EXISTS enrichment_jobs (
    id            BIGSERIAL PRIMARY KEY,
    job_type      TEXT NOT NULL,
    entity_type   TEXT NOT NULL,
    entity_id     TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'queued',
    priority      INT NOT NULL DEFAULT 100,
    attempt_count INT NOT NULL DEFAULT 0,
    max_attempts  INT NOT NULL DEFAULT 5,
    not_before    TIMESTAMP NULL,
    locked_at     TIMESTAMP NULL,
    locked_by     TEXT NULL,
    last_error    TEXT NULL,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_enrichment_jobs_dedupe
ON enrichment_jobs(job_type, entity_type, entity_id)
WHERE status IN ('queued', 'running');

CREATE INDEX IF NOT EXISTS idx_enrichment_jobs_sched
ON enrichment_jobs(status, priority, not_before, created_at);

CREATE TABLE IF NOT EXISTS enrichment_job_runs (
    id          BIGSERIAL PRIMARY KEY,
    job_id      BIGINT NOT NULL REFERENCES enrichment_jobs(id) ON DELETE CASCADE,
    started_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMP NULL,
    outcome     TEXT NOT NULL,
    error       TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_enrichment_job_runs_job
ON enrichment_job_runs(job_id);
