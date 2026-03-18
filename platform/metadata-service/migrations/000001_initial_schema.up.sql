-- Authors
CREATE TABLE IF NOT EXISTS authors (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    sort_name   TEXT,
    created_at  TIMESTAMP DEFAULT NOW(),
    updated_at  TIMESTAMP DEFAULT NOW()
);

-- Works
CREATE TABLE IF NOT EXISTS works (
    id                TEXT PRIMARY KEY,
    title             TEXT NOT NULL,
    normalized_title  TEXT,
    fingerprint       TEXT,
    first_pub_year    INTEGER,
    created_at        TIMESTAMP DEFAULT NOW(),
    updated_at        TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_works_title       ON works(normalized_title);
CREATE INDEX IF NOT EXISTS idx_works_fingerprint ON works(fingerprint);

-- Work Authors
CREATE TABLE IF NOT EXISTS work_authors (
    work_id    TEXT NOT NULL REFERENCES works(id),
    author_id  TEXT NOT NULL REFERENCES authors(id),
    PRIMARY KEY(work_id, author_id)
);

-- Editions
CREATE TABLE IF NOT EXISTS editions (
    id                TEXT PRIMARY KEY,
    work_id           TEXT NOT NULL REFERENCES works(id),
    title             TEXT,
    format            TEXT,
    publisher         TEXT,
    publication_year  INTEGER,
    created_at        TIMESTAMP DEFAULT NOW(),
    updated_at        TIMESTAMP DEFAULT NOW()
);

-- Identifiers
CREATE TABLE IF NOT EXISTS identifiers (
    id          SERIAL PRIMARY KEY,
    edition_id  TEXT NOT NULL REFERENCES editions(id),
    type        TEXT NOT NULL,
    value       TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_identifier_unique ON identifiers(type, value);

-- Provider Mappings
CREATE TABLE IF NOT EXISTS provider_mappings (
    id            SERIAL PRIMARY KEY,
    provider      TEXT NOT NULL,
    provider_id   TEXT NOT NULL,
    entity_type   TEXT NOT NULL,
    canonical_id  TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_provider_mapping ON provider_mappings(provider, provider_id);
