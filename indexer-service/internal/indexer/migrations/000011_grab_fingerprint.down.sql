DROP INDEX IF EXISTS idx_grabs_fingerprint_entity;
ALTER TABLE indexer_grabs DROP COLUMN IF EXISTS fingerprint;
ALTER TABLE indexer_candidates DROP COLUMN IF EXISTS fingerprint;
