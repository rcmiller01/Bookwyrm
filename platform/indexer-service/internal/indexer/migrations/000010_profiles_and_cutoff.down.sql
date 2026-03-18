ALTER TABLE indexer_wanted_authors
  DROP COLUMN IF EXISTS profile_id;

ALTER TABLE indexer_wanted_works
  DROP COLUMN IF EXISTS ignore_upgrades,
  DROP COLUMN IF EXISTS profile_id;

DROP TABLE IF EXISTS indexer_profile_qualities;
DROP TABLE IF EXISTS indexer_profiles;

