CREATE TABLE IF NOT EXISTS indexer_backends (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  backend_type TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT true,
  tier TEXT NOT NULL DEFAULT 'unclassified',
  reliability_score DOUBLE PRECISION NOT NULL DEFAULT 0.70,
  priority INT NOT NULL DEFAULT 100,
  config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_indexer_backends_enabled ON indexer_backends(enabled);

CREATE TABLE IF NOT EXISTS mcp_servers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  source TEXT NOT NULL,
  source_ref TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT true,
  base_url TEXT NULL,
  env_schema JSONB NOT NULL DEFAULT '{}'::jsonb,
  env_mapping JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS indexer_search_requests (
  id BIGSERIAL PRIMARY KEY,
  request_key TEXT NOT NULL UNIQUE,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  query_json JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'queued',
  attempt_count INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 3,
  last_error TEXT NULL,
  not_before TIMESTAMP NOT NULL DEFAULT NOW(),
  locked_at TIMESTAMP NULL,
  locked_by TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_search_requests_status ON indexer_search_requests(status, created_at);
CREATE INDEX IF NOT EXISTS idx_search_requests_not_before ON indexer_search_requests(not_before);

CREATE TABLE IF NOT EXISTS indexer_candidates (
  id BIGSERIAL PRIMARY KEY,
  search_request_id BIGINT NOT NULL REFERENCES indexer_search_requests(id) ON DELETE CASCADE,
  source_pipeline TEXT NOT NULL,
  source_backend_id TEXT NOT NULL,
  title TEXT NOT NULL,
  normalized_title TEXT NOT NULL,
  protocol TEXT NOT NULL,
  size_bytes BIGINT NULL,
  seeders INT NULL,
  leechers INT NULL,
  published_at TIMESTAMP NULL,
  identifiers JSONB NOT NULL DEFAULT '{}'::jsonb,
  attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
  grab_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  score DOUBLE PRECISION NOT NULL DEFAULT 0.0,
  reasons JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_candidates_req ON indexer_candidates(search_request_id);
CREATE INDEX IF NOT EXISTS idx_candidates_score ON indexer_candidates(search_request_id, score DESC);

CREATE TABLE IF NOT EXISTS indexer_grabs (
  id BIGSERIAL PRIMARY KEY,
  candidate_id BIGINT NOT NULL REFERENCES indexer_candidates(id) ON DELETE CASCADE,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'created',
  downstream_ref TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
