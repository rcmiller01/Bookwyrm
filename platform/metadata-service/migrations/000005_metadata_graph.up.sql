CREATE TABLE series (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  normalized_name TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_series_normalized
ON series(normalized_name);

CREATE TABLE series_entries (
  series_id TEXT NOT NULL REFERENCES series(id) ON DELETE CASCADE,
  work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  series_index DOUBLE PRECISION NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  PRIMARY KEY(series_id, work_id)
);

CREATE INDEX idx_series_entries_series
ON series_entries(series_id, series_index);

CREATE INDEX idx_series_entries_work
ON series_entries(work_id);

CREATE TABLE subjects (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  normalized_name TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_subjects_normalized
ON subjects(normalized_name);

CREATE TABLE work_subjects (
  work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  subject_id TEXT NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  PRIMARY KEY(work_id, subject_id)
);

CREATE INDEX idx_work_subjects_subject
ON work_subjects(subject_id);

CREATE TABLE work_relationships (
  source_work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  target_work_id TEXT NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  relationship_type TEXT NOT NULL,
  confidence DOUBLE PRECISION NOT NULL DEFAULT 0.5,
  provider TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  PRIMARY KEY(source_work_id, target_work_id, relationship_type)
);

CREATE INDEX idx_workrels_source
ON work_relationships(source_work_id);

CREATE INDEX idx_workrels_target
ON work_relationships(target_work_id);

ALTER TABLE works
ADD COLUMN IF NOT EXISTS series_name TEXT,
ADD COLUMN IF NOT EXISTS series_index DOUBLE PRECISION,
ADD COLUMN IF NOT EXISTS subjects TEXT[];
