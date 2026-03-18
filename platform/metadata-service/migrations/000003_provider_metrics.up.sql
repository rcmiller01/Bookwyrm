-- Provider raw performance metrics
CREATE TABLE IF NOT EXISTS provider_metrics (
    provider              TEXT PRIMARY KEY,
    success_count         BIGINT NOT NULL DEFAULT 0,
    failure_count         BIGINT NOT NULL DEFAULT 0,
    total_latency_ms      BIGINT NOT NULL DEFAULT 0,
    request_count         BIGINT NOT NULL DEFAULT 0,
    identifier_matches    BIGINT NOT NULL DEFAULT 0,
    identifier_introduced BIGINT NOT NULL DEFAULT 0,
    last_success          TIMESTAMP,
    last_failure          TIMESTAMP,
    updated_at            TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_provider_metrics_updated
    ON provider_metrics(updated_at);

-- Provider computed reliability scores
CREATE TABLE IF NOT EXISTS provider_reliability (
    provider           TEXT PRIMARY KEY,
    availability       FLOAT NOT NULL DEFAULT 0.7,
    latency_score      FLOAT NOT NULL DEFAULT 0.7,
    agreement_score    FLOAT NOT NULL DEFAULT 0.7,
    identifier_quality FLOAT NOT NULL DEFAULT 0.7,
    composite_score    FLOAT NOT NULL DEFAULT 0.7,
    updated_at         TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Seed baseline scores for all known providers
INSERT INTO provider_reliability (provider, availability, latency_score, agreement_score, identifier_quality, composite_score)
VALUES
    ('openlibrary', 0.7, 0.7, 0.7, 0.7, 0.7),
    ('googlebooks', 0.7, 0.7, 0.7, 0.7, 0.7),
    ('hardcover',   0.7, 0.7, 0.7, 0.7, 0.7)
ON CONFLICT (provider) DO NOTHING;

-- Seed empty metrics rows so the worker can read them immediately
INSERT INTO provider_metrics (provider)
VALUES ('openlibrary'), ('googlebooks'), ('hardcover')
ON CONFLICT (provider) DO NOTHING;
