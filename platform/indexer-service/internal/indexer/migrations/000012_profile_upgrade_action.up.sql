ALTER TABLE indexer_profiles ADD COLUMN IF NOT EXISTS upgrade_action TEXT NOT NULL DEFAULT 'ask';
