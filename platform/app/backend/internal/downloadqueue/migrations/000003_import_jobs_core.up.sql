CREATE TABLE IF NOT EXISTS import_jobs (
  id BIGSERIAL PRIMARY KEY,
  download_job_id BIGINT NOT NULL REFERENCES download_jobs(id) ON DELETE CASCADE,
  work_id TEXT NULL,
  edition_id TEXT NULL,
  source_path TEXT NOT NULL,
  target_root TEXT NOT NULL,
  target_path TEXT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  attempt_count INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 3,
  rename_template TEXT NULL,
  naming_result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  decision_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  last_error TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_import_jobs_status ON import_jobs(status, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS uq_import_jobs_download ON import_jobs(download_job_id);

CREATE TABLE IF NOT EXISTS import_events (
  id BIGSERIAL PRIMARY KEY,
  import_job_id BIGINT NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
  ts TIMESTAMP NOT NULL DEFAULT NOW(),
  event_type TEXT NOT NULL,
  message TEXT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_import_events_job ON import_events(import_job_id);

CREATE TABLE IF NOT EXISTS library_items (
  id BIGSERIAL PRIMARY KEY,
  work_id TEXT NOT NULL,
  edition_id TEXT NULL,
  path TEXT NOT NULL UNIQUE,
  format TEXT NOT NULL,
  size_bytes BIGINT NULL,
  checksum TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_library_items_work ON library_items(work_id);
