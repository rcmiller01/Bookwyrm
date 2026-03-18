package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"metadata-service/internal/model"
)

type EditionStore interface {
	InsertEdition(ctx context.Context, edition model.Edition) error
	GetEditionsByWork(ctx context.Context, workID string) ([]model.Edition, error)
	GetEditionByID(ctx context.Context, id string) (*model.Edition, error)
}

type pgEditionStore struct {
	db *pgxpool.Pool
}

func NewEditionStore(db *pgxpool.Pool) EditionStore {
	return &pgEditionStore{db: db}
}

func (s *pgEditionStore) InsertEdition(ctx context.Context, edition model.Edition) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO editions (id, work_id, title, format, publisher, publication_year)
		 VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING`,
		edition.ID, edition.WorkID, edition.Title, edition.Format, edition.Publisher, edition.PublicationYear,
	)
	return err
}

func (s *pgEditionStore) GetEditionsByWork(ctx context.Context, workID string) ([]model.Edition, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, work_id, title, format, publisher, publication_year FROM editions WHERE work_id = $1`,
		workID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var editions []model.Edition
	for rows.Next() {
		var e model.Edition
		if err := rows.Scan(&e.ID, &e.WorkID, &e.Title, &e.Format, &e.Publisher, &e.PublicationYear); err != nil {
			return nil, err
		}
		editions = append(editions, e)
	}
	return editions, rows.Err()
}

func (s *pgEditionStore) GetEditionByID(ctx context.Context, id string) (*model.Edition, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, work_id, title, format, publisher, publication_year FROM editions WHERE id = $1`,
		id,
	)
	var e model.Edition
	if err := row.Scan(&e.ID, &e.WorkID, &e.Title, &e.Format, &e.Publisher, &e.PublicationYear); err != nil {
		return nil, err
	}
	return &e, nil
}
