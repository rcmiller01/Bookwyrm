ALTER TABLE download_jobs
  ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMP NULL;

CREATE INDEX IF NOT EXISTS idx_download_jobs_lease_expires_at
ON download_jobs(lease_expires_at)
WHERE status = 'submitted';

ALTER TABLE import_jobs
  ADD COLUMN IF NOT EXISTS locked_at TIMESTAMP NULL,
  ADD COLUMN IF NOT EXISTS locked_by TEXT NULL,
  ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMP NULL;

CREATE INDEX IF NOT EXISTS idx_import_jobs_lease_expires_at
ON import_jobs(lease_expires_at)
WHERE status = 'running';
