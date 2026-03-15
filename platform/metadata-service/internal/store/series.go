package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"metadata-service/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeriesStore manages graph series and series entries.
type SeriesStore interface {
	UpsertSeries(ctx context.Context, name string, normalized string) (string, error)
	UpsertSeriesEntry(ctx context.Context, seriesID string, workID string, seriesIndex *float64) error
	GetSeriesByID(ctx context.Context, id string) (*model.Series, error)
	GetSeriesEntries(ctx context.Context, seriesID string) ([]model.SeriesEntry, error)
	GetSeriesForWork(ctx context.Context, workID string) (*model.Series, error)
	CountSeries(ctx context.Context) (int64, error)
}

type pgSeriesStore struct {
	db *pgxpool.Pool
}

func NewSeriesStore(db *pgxpool.Pool) SeriesStore {
	return &pgSeriesStore{db: db}
}

func (s *pgSeriesStore) UpsertSeries(ctx context.Context, name string, normalized string) (string, error) {
	name = strings.TrimSpace(name)
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return "", errors.New("normalized series name is required")
	}
	id := fmt.Sprintf("series:%s", normalized)

	var returnedID string
	err := s.db.QueryRow(ctx, `
		INSERT INTO series (id, name, normalized_name, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (normalized_name) DO UPDATE SET
			name = EXCLUDED.name,
			updated_at = NOW()
		RETURNING id`,
		id, name, normalized,
	).Scan(&returnedID)
	if err != nil {
		return "", err
	}
	return returnedID, nil
}

func (s *pgSeriesStore) UpsertSeriesEntry(ctx context.Context, seriesID string, workID string, seriesIndex *float64) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO series_entries (series_id, work_id, series_index, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (series_id, work_id) DO UPDATE SET
			series_index = EXCLUDED.series_index,
			updated_at = NOW()`,
		seriesID, workID, seriesIndex,
	)
	return err
}

func (s *pgSeriesStore) GetSeriesByID(ctx context.Context, id string) (*model.Series, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, name, normalized_name, created_at, updated_at
		FROM series
		WHERE id = $1`, id)
	var series model.Series
	if err := row.Scan(&series.ID, &series.Name, &series.NormalizedName, &series.CreatedAt, &series.UpdatedAt); err != nil {
		return nil, err
	}
	return &series, nil
}

func (s *pgSeriesStore) GetSeriesEntries(ctx context.Context, seriesID string) ([]model.SeriesEntry, error) {
	rows, err := s.db.Query(ctx, `
		SELECT series_id, work_id, series_index, created_at, updated_at
		FROM series_entries
		WHERE series_id = $1
		ORDER BY series_index ASC NULLS LAST, created_at ASC`,
		seriesID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []model.SeriesEntry
	for rows.Next() {
		var entry model.SeriesEntry
		if scanErr := rows.Scan(
			&entry.SeriesID,
			&entry.WorkID,
			&entry.SeriesIndex,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *pgSeriesStore) GetSeriesForWork(ctx context.Context, workID string) (*model.Series, error) {
	row := s.db.QueryRow(ctx, `
		SELECT s.id, s.name, s.normalized_name, s.created_at, s.updated_at
		FROM series s
		JOIN series_entries se ON se.series_id = s.id
		WHERE se.work_id = $1
		ORDER BY se.series_index ASC NULLS LAST, se.created_at ASC
		LIMIT 1`,
		workID,
	)
	var series model.Series
	if err := row.Scan(&series.ID, &series.Name, &series.NormalizedName, &series.CreatedAt, &series.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}
	return &series, nil
}

func (s *pgSeriesStore) CountSeries(ctx context.Context) (int64, error) {
	row := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM series`)
	var count int64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
