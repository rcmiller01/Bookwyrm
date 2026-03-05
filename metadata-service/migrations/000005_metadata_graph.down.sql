DROP INDEX IF EXISTS idx_workrels_target;
DROP INDEX IF EXISTS idx_workrels_source;
DROP TABLE IF EXISTS work_relationships;

DROP INDEX IF EXISTS idx_work_subjects_subject;
DROP TABLE IF EXISTS work_subjects;

DROP INDEX IF EXISTS uq_subjects_normalized;
DROP TABLE IF EXISTS subjects;

DROP INDEX IF EXISTS idx_series_entries_work;
DROP INDEX IF EXISTS idx_series_entries_series;
DROP TABLE IF EXISTS series_entries;

DROP INDEX IF EXISTS uq_series_normalized;
DROP TABLE IF EXISTS series;

ALTER TABLE works
DROP COLUMN IF EXISTS subjects,
DROP COLUMN IF EXISTS series_index,
DROP COLUMN IF EXISTS series_name;
