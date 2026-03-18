package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"metadata-service/internal/model"
)

type AuthorStore interface {
	InsertAuthor(ctx context.Context, author model.Author) error
	GetAuthorByName(ctx context.Context, name string) (*model.Author, error)
	LinkWorkAuthor(ctx context.Context, workID string, authorID string) error
}

type pgAuthorStore struct {
	db *pgxpool.Pool
}

func NewAuthorStore(db *pgxpool.Pool) AuthorStore {
	return &pgAuthorStore{db: db}
}

func (s *pgAuthorStore) InsertAuthor(ctx context.Context, author model.Author) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO authors (id, name, sort_name) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`,
		author.ID, author.Name, author.SortName,
	)
	return err
}

func (s *pgAuthorStore) GetAuthorByName(ctx context.Context, name string) (*model.Author, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, name, sort_name FROM authors WHERE name ILIKE $1 LIMIT 1`,
		name,
	)
	var a model.Author
	if err := row.Scan(&a.ID, &a.Name, &a.SortName); err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *pgAuthorStore) LinkWorkAuthor(ctx context.Context, workID string, authorID string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO work_authors (work_id, author_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		workID, authorID,
	)
	return err
}
