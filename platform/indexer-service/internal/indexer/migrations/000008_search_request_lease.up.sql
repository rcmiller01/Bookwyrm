ALTER TABLE indexer_search_requests
  ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMP NULL;

CREATE INDEX IF NOT EXISTS idx_search_requests_lease_expires_at
ON indexer_search_requests(lease_expires_at)
WHERE status = 'running';
