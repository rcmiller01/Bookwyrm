DROP INDEX IF EXISTS idx_import_jobs_lease_expires_at;
DROP INDEX IF EXISTS idx_download_jobs_lease_expires_at;

ALTER TABLE import_jobs
  DROP COLUMN IF EXISTS lease_expires_at,
  DROP COLUMN IF EXISTS locked_by,
  DROP COLUMN IF EXISTS locked_at;

ALTER TABLE download_jobs
  DROP COLUMN IF EXISTS lease_expires_at;
