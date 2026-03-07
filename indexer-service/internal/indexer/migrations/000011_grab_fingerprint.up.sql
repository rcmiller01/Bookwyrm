ALTER TABLE indexer_candidates ADD COLUMN IF NOT EXISTS fingerprint TEXT;
ALTER TABLE indexer_grabs ADD COLUMN IF NOT EXISTS fingerprint TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_grabs_fingerprint_entity
  ON indexer_grabs (fingerprint, entity_type, entity_id)
  WHERE fingerprint IS NOT NULL;
