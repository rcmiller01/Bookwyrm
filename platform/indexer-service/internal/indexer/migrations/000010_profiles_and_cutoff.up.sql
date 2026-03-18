CREATE TABLE IF NOT EXISTS indexer_profiles (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  cutoff_quality TEXT NOT NULL,
  default_profile BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS indexer_profile_qualities (
  profile_id TEXT NOT NULL REFERENCES indexer_profiles(id) ON DELETE CASCADE,
  quality TEXT NOT NULL,
  rank INTEGER NOT NULL,
  PRIMARY KEY (profile_id, quality)
);

ALTER TABLE indexer_wanted_works
  ADD COLUMN IF NOT EXISTS profile_id TEXT REFERENCES indexer_profiles(id),
  ADD COLUMN IF NOT EXISTS ignore_upgrades BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE indexer_wanted_authors
  ADD COLUMN IF NOT EXISTS profile_id TEXT REFERENCES indexer_profiles(id);

INSERT INTO indexer_profiles (id, name, cutoff_quality, default_profile, created_at, updated_at)
VALUES ('default-ebook', 'Default Ebook', 'epub', TRUE, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  cutoff_quality = EXCLUDED.cutoff_quality,
  default_profile = TRUE,
  updated_at = NOW();

UPDATE indexer_profiles
SET default_profile = CASE WHEN id = 'default-ebook' THEN TRUE ELSE FALSE END;

INSERT INTO indexer_profile_qualities (profile_id, quality, rank)
VALUES
  ('default-ebook', 'epub', 1),
  ('default-ebook', 'azw3', 2),
  ('default-ebook', 'mobi', 3),
  ('default-ebook', 'pdf', 4)
ON CONFLICT (profile_id, quality) DO UPDATE SET rank = EXCLUDED.rank;

UPDATE indexer_wanted_works
SET profile_id = 'default-ebook'
WHERE profile_id IS NULL;

UPDATE indexer_wanted_authors
SET profile_id = 'default-ebook'
WHERE profile_id IS NULL;

