CREATE TABLE content_sources (
  id BIGSERIAL PRIMARY KEY,
  edition_id TEXT NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  source_type TEXT NOT NULL,
  source_name TEXT,
  source_url TEXT,
  availability TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_content_sources_edition ON content_sources(edition_id);
CREATE INDEX idx_content_sources_provider ON content_sources(provider);

CREATE TABLE file_metadata (
  id BIGSERIAL PRIMARY KEY,
  content_source_id BIGINT NOT NULL REFERENCES content_sources(id) ON DELETE CASCADE,
  file_name TEXT,
  file_format TEXT,
  file_size_bytes BIGINT,
  language TEXT,
  checksum TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_file_metadata_content_source ON file_metadata(content_source_id);
CREATE INDEX idx_file_metadata_format ON file_metadata(file_format);
