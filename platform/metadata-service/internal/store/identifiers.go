package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"metadata-service/internal/model"
)

type IdentifierStore interface {
	InsertIdentifier(ctx context.Context, editionID string, id model.Identifier) error
	FindEditionByIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error)
	GetIdentifiersByEdition(ctx context.Context, editionID string) ([]model.Identifier, error)
}

type ProviderMappingStore interface {
	GetCanonicalID(ctx context.Context, provider string, providerID string) (string, error)
	InsertMapping(ctx context.Context, provider string, providerID string, entityType string, canonicalID string) error
}

// --- Identifier Store ---

type pgIdentifierStore struct {
	db *pgxpool.Pool
}

func NewIdentifierStore(db *pgxpool.Pool) IdentifierStore {
	return &pgIdentifierStore{db: db}
}

func (s *pgIdentifierStore) InsertIdentifier(ctx context.Context, editionID string, id model.Identifier) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO identifiers (edition_id, type, value) VALUES ($1, $2, $3) ON CONFLICT (type, value) DO NOTHING`,
		editionID, id.Type, id.Value,
	)
	return err
}

func (s *pgIdentifierStore) FindEditionByIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error) {
	row := s.db.QueryRow(ctx,
		`SELECT e.id, e.work_id, e.title, e.format, e.publisher, e.publication_year
		 FROM editions e
		 JOIN identifiers i ON i.edition_id = e.id
		 WHERE i.type = $1 AND i.value = $2
		 LIMIT 1`,
		idType, value,
	)
	var e model.Edition
	if err := row.Scan(&e.ID, &e.WorkID, &e.Title, &e.Format, &e.Publisher, &e.PublicationYear); err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *pgIdentifierStore) GetIdentifiersByEdition(ctx context.Context, editionID string) ([]model.Identifier, error) {
	rows, err := s.db.Query(ctx, `SELECT type, value FROM identifiers WHERE edition_id = $1`, editionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []model.Identifier
	for rows.Next() {
		var id model.Identifier
		if err := rows.Scan(&id.Type, &id.Value); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// --- Provider Mapping Store ---

type pgProviderMappingStore struct {
	db *pgxpool.Pool
}

func NewProviderMappingStore(db *pgxpool.Pool) ProviderMappingStore {
	return &pgProviderMappingStore{db: db}
}

func (s *pgProviderMappingStore) GetCanonicalID(ctx context.Context, provider string, providerID string) (string, error) {
	row := s.db.QueryRow(ctx,
		`SELECT canonical_id FROM provider_mappings WHERE provider = $1 AND provider_id = $2`,
		provider, providerID,
	)
	var id string
	err := row.Scan(&id)
	return id, err
}

func (s *pgProviderMappingStore) InsertMapping(ctx context.Context, provider string, providerID string, entityType string, canonicalID string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO provider_mappings (provider, provider_id, entity_type, canonical_id)
		 VALUES ($1, $2, $3, $4) ON CONFLICT (provider, provider_id) DO NOTHING`,
		provider, providerID, entityType, canonicalID,
	)
	return err
}
