package store

import (
	"context"

	"metadata-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkStore interface {
	GetWorkByID(ctx context.Context, id string) (*model.Work, error)
	SearchWorks(ctx context.Context, query string) ([]model.Work, error)
	InsertWork(ctx context.Context, work model.Work) error
	UpdateWork(ctx context.Context, work model.Work) error
	GetWorkByFingerprint(ctx context.Context, fingerprint string) (*model.Work, error)
}

type pgWorkStore struct {
	db *pgxpool.Pool
}

func NewWorkStore(db *pgxpool.Pool) WorkStore {
	return &pgWorkStore{db: db}
}

func (s *pgWorkStore) GetWorkByID(ctx context.Context, id string) (*model.Work, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, title, normalized_title, fingerprint, first_pub_year, series_name, series_index, COALESCE(subjects, '{}') FROM works WHERE id = $1`,
		id,
	)
	var w model.Work
	if err := row.Scan(&w.ID, &w.Title, &w.NormalizedTitle, &w.Fingerprint, &w.FirstPubYear, &w.SeriesName, &w.SeriesIndex, &w.Subjects); err != nil {
		return nil, err
	}
	return &w, nil
}

func (s *pgWorkStore) SearchWorks(ctx context.Context, query string) ([]model.Work, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, title, normalized_title, fingerprint, first_pub_year, series_name, series_index, COALESCE(subjects, '{}') FROM works WHERE normalized_title ILIKE $1 LIMIT 20`,
		"%"+query+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var works []model.Work
	for rows.Next() {
		var w model.Work
		if err := rows.Scan(&w.ID, &w.Title, &w.NormalizedTitle, &w.Fingerprint, &w.FirstPubYear, &w.SeriesName, &w.SeriesIndex, &w.Subjects); err != nil {
			return nil, err
		}
		works = append(works, w)
	}
	return works, rows.Err()
}

func (s *pgWorkStore) InsertWork(ctx context.Context, work model.Work) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO works (id, title, normalized_title, fingerprint, first_pub_year, series_name, series_index, subjects)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT (id) DO NOTHING`,
		work.ID, work.Title, work.NormalizedTitle, work.Fingerprint, work.FirstPubYear, work.SeriesName, work.SeriesIndex, work.Subjects,
	)
	return err
}

func (s *pgWorkStore) UpdateWork(ctx context.Context, work model.Work) error {
	_, err := s.db.Exec(ctx,
		`UPDATE works SET title=$1, normalized_title=$2, fingerprint=$3, first_pub_year=$4, series_name=$5, series_index=$6, subjects=$7, updated_at=NOW() WHERE id=$8`,
		work.Title, work.NormalizedTitle, work.Fingerprint, work.FirstPubYear, work.SeriesName, work.SeriesIndex, work.Subjects, work.ID,
	)
	return err
}

func (s *pgWorkStore) GetWorkByFingerprint(ctx context.Context, fingerprint string) (*model.Work, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, title, normalized_title, fingerprint, first_pub_year, series_name, series_index, COALESCE(subjects, '{}') FROM works WHERE fingerprint = $1`,
		fingerprint,
	)
	var w model.Work
	if err := row.Scan(&w.ID, &w.Title, &w.NormalizedTitle, &w.Fingerprint, &w.FirstPubYear, &w.SeriesName, &w.SeriesIndex, &w.Subjects); err != nil {
		return nil, err
	}
	return &w, nil
}
