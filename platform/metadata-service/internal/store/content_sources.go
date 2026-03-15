package store

import (
	"context"

	"metadata-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ContentSourceStore interface {
	InsertContentSource(ctx context.Context, source model.ContentSource) (int64, error)
	ListContentSourcesByEdition(ctx context.Context, editionID string) ([]model.ContentSource, error)
	InsertFileMetadata(ctx context.Context, file model.FileMetadata) (int64, error)
	ListFileMetadataByContentSource(ctx context.Context, contentSourceID int64) ([]model.FileMetadata, error)
}

type pgContentSourceStore struct {
	db *pgxpool.Pool
}

func NewContentSourceStore(db *pgxpool.Pool) ContentSourceStore {
	return &pgContentSourceStore{db: db}
}

func (s *pgContentSourceStore) InsertContentSource(ctx context.Context, source model.ContentSource) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO content_sources (edition_id, provider, source_type, source_name, source_url, availability, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id`,
		source.EditionID,
		source.Provider,
		source.SourceType,
		source.SourceName,
		source.SourceURL,
		source.Availability,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *pgContentSourceStore) ListContentSourcesByEdition(ctx context.Context, editionID string) ([]model.ContentSource, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, edition_id, provider, source_type, source_name, source_url, availability, created_at, updated_at
		FROM content_sources
		WHERE edition_id = $1
		ORDER BY created_at DESC, id DESC`, editionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ContentSource, 0)
	for rows.Next() {
		var source model.ContentSource
		if scanErr := rows.Scan(
			&source.ID,
			&source.EditionID,
			&source.Provider,
			&source.SourceType,
			&source.SourceName,
			&source.SourceURL,
			&source.Availability,
			&source.CreatedAt,
			&source.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, source)
	}
	return out, rows.Err()
}

func (s *pgContentSourceStore) InsertFileMetadata(ctx context.Context, file model.FileMetadata) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO file_metadata (content_source_id, file_name, file_format, file_size_bytes, language, checksum, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id`,
		file.ContentSourceID,
		file.FileName,
		file.FileFormat,
		file.FileSizeBytes,
		file.Language,
		file.Checksum,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *pgContentSourceStore) ListFileMetadataByContentSource(ctx context.Context, contentSourceID int64) ([]model.FileMetadata, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, content_source_id, file_name, file_format, file_size_bytes, language, checksum, created_at, updated_at
		FROM file_metadata
		WHERE content_source_id = $1
		ORDER BY created_at DESC, id DESC`, contentSourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.FileMetadata, 0)
	for rows.Next() {
		var file model.FileMetadata
		if scanErr := rows.Scan(
			&file.ID,
			&file.ContentSourceID,
			&file.FileName,
			&file.FileFormat,
			&file.FileSizeBytes,
			&file.Language,
			&file.Checksum,
			&file.CreatedAt,
			&file.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, file)
	}
	return out, rows.Err()
}
