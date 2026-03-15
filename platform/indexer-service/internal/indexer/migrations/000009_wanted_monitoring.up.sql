CREATE TABLE IF NOT EXISTS indexer_wanted_works (
  work_id TEXT PRIMARY KEY,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  priority INTEGER NOT NULL DEFAULT 100,
  cadence_minutes INTEGER NOT NULL DEFAULT 60,
  formats TEXT[] NOT NULL DEFAULT '{}',
  languages TEXT[] NOT NULL DEFAULT '{}',
  last_enqueued_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS indexer_wanted_authors (
  author_id TEXT PRIMARY KEY,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  priority INTEGER NOT NULL DEFAULT 100,
  cadence_minutes INTEGER NOT NULL DEFAULT 60,
  formats TEXT[] NOT NULL DEFAULT '{}',
  languages TEXT[] NOT NULL DEFAULT '{}',
  last_enqueued_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_indexer_wanted_works_enabled_due
  ON indexer_wanted_works (enabled, priority, last_enqueued_at);

CREATE INDEX IF NOT EXISTS idx_indexer_wanted_authors_enabled_due
  ON indexer_wanted_authors (enabled, priority, last_enqueued_at);
