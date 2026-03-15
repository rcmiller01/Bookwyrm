CREATE TABLE IF NOT EXISTS download_clients (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  client_type TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT true,
  priority INT NOT NULL DEFAULT 100,
  config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS download_jobs (
  id BIGSERIAL PRIMARY KEY,
  grab_id BIGINT NOT NULL,
  candidate_id BIGINT NOT NULL,
  work_id TEXT NOT NULL,
  edition_id TEXT NULL,
  protocol TEXT NOT NULL,
  client_name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  download_id TEXT NULL,
  output_path TEXT NULL,
  request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  last_error TEXT NULL,
  attempt_count INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 3,
  not_before TIMESTAMP NOT NULL DEFAULT NOW(),
  locked_at TIMESTAMP NULL,
  locked_by TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_download_jobs_status_not_before ON download_jobs(status, not_before);
CREATE INDEX IF NOT EXISTS idx_download_jobs_work ON download_jobs(work_id);

CREATE TABLE IF NOT EXISTS download_events (
  id BIGSERIAL PRIMARY KEY,
  job_id BIGINT NOT NULL REFERENCES download_jobs(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  message TEXT NOT NULL,
  data_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
