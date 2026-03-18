CREATE TABLE IF NOT EXISTS indexer_metrics (
  backend_id TEXT PRIMARY KEY REFERENCES indexer_backends(id) ON DELETE CASCADE,
  success_count BIGINT NOT NULL DEFAULT 0,
  failure_count BIGINT NOT NULL DEFAULT 0,
  total_latency_ms BIGINT NOT NULL DEFAULT 0,
  search_count BIGINT NOT NULL DEFAULT 0,
  candidate_yield_count BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS indexer_reliability (
  backend_id TEXT PRIMARY KEY REFERENCES indexer_backends(id) ON DELETE CASCADE,
  availability_score DOUBLE PRECISION NOT NULL DEFAULT 0.70,
  latency_score DOUBLE PRECISION NOT NULL DEFAULT 0.70,
  yield_score DOUBLE PRECISION NOT NULL DEFAULT 0.70,
  composite_score DOUBLE PRECISION NOT NULL DEFAULT 0.70,
  tier TEXT NOT NULL DEFAULT 'unclassified',
  computed_at TIMESTAMP NOT NULL DEFAULT NOW()
);
