DROP INDEX IF EXISTS idx_search_requests_lease_expires_at;

ALTER TABLE indexer_search_requests
  DROP COLUMN IF EXISTS lease_expires_at;
