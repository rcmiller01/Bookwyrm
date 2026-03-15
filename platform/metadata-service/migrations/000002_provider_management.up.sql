-- Provider configurations (replaces YAML-only config)
CREATE TABLE IF NOT EXISTS provider_configs (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    enabled     BOOLEAN NOT NULL DEFAULT true,
    priority    INTEGER NOT NULL DEFAULT 100,
    timeout_sec INTEGER NOT NULL DEFAULT 10,
    rate_limit  INTEGER NOT NULL DEFAULT 60,  -- requests per minute
    api_key     TEXT,
    extra       JSONB,
    created_at  TIMESTAMP DEFAULT NOW(),
    updated_at  TIMESTAMP DEFAULT NOW()
);

-- Provider runtime health status
CREATE TABLE IF NOT EXISTS provider_status (
    id            SERIAL PRIMARY KEY,
    name          TEXT NOT NULL UNIQUE,
    status        TEXT NOT NULL DEFAULT 'healthy',  -- healthy, degraded, unreliable, disabled
    failure_count INTEGER NOT NULL DEFAULT 0,
    last_success  TIMESTAMP,
    last_failure  TIMESTAMP,
    last_checked  TIMESTAMP DEFAULT NOW(),
    avg_latency_ms BIGINT DEFAULT 0,
    updated_at    TIMESTAMP DEFAULT NOW()
);

-- Seed default provider entries
INSERT INTO provider_configs (name, enabled, priority, timeout_sec, rate_limit)
VALUES
    ('openlibrary', true,  100, 10, 100),
    ('googlebooks', false, 200, 10, 100),
    ('hardcover',   false, 300, 15, 60)
ON CONFLICT (name) DO NOTHING;

INSERT INTO provider_status (name, status)
VALUES
    ('openlibrary', 'healthy'),
    ('googlebooks', 'healthy'),
    ('hardcover',   'healthy')
ON CONFLICT (name) DO NOTHING;
