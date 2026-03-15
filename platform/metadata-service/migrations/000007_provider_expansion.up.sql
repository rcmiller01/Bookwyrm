-- Phase 10.1 provider expansion defaults (metadata-only providers)
INSERT INTO provider_configs (name, enabled, priority, timeout_sec, rate_limit)
VALUES
    ('crossref', false, 350, 6, 3)
ON CONFLICT (name) DO NOTHING;

INSERT INTO provider_status (name, status)
VALUES
    ('crossref', 'healthy')
ON CONFLICT (name) DO NOTHING;

UPDATE provider_configs
SET timeout_sec = 6, rate_limit = 8
WHERE name = 'googlebooks';

UPDATE provider_configs
SET timeout_sec = 10, rate_limit = 2
WHERE name = 'hardcover';
