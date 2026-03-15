DROP TABLE IF EXISTS download_client_reliability;
DROP TABLE IF EXISTS download_client_metrics;
ALTER TABLE download_jobs DROP COLUMN IF EXISTS imported;
ALTER TABLE download_clients DROP COLUMN IF EXISTS reliability_score;
ALTER TABLE download_clients DROP COLUMN IF EXISTS tier;
