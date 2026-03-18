package store

import (
	"context"

	"metadata-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WorkRelationshipStore manages directional graph edges between works.
type WorkRelationshipStore interface {
	UpsertRelationship(ctx context.Context, sourceID string, targetID string, relationshipType string, confidence float64, provider *string) error
	GetRelatedWorks(ctx context.Context, sourceID string, relationshipType *string, limit int) ([]model.WorkRelationship, error)
	DeleteRelationshipsForWork(ctx context.Context, sourceID string, relationshipType *string) error
	CountRelationshipsByType(ctx context.Context) (map[string]int64, error)
}

type pgWorkRelationshipStore struct {
	db *pgxpool.Pool
}

func NewWorkRelationshipStore(db *pgxpool.Pool) WorkRelationshipStore {
	return &pgWorkRelationshipStore{db: db}
}

func (s *pgWorkRelationshipStore) UpsertRelationship(
	ctx context.Context,
	sourceID string,
	targetID string,
	relationshipType string,
	confidence float64,
	provider *string,
) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO work_relationships
			(source_work_id, target_work_id, relationship_type, confidence, provider, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (source_work_id, target_work_id, relationship_type) DO UPDATE SET
			confidence = EXCLUDED.confidence,
			provider = EXCLUDED.provider,
			updated_at = NOW()`,
		sourceID, targetID, relationshipType, confidence, provider,
	)
	return err
}

func (s *pgWorkRelationshipStore) GetRelatedWorks(
	ctx context.Context,
	sourceID string,
	relationshipType *string,
	limit int,
) ([]model.WorkRelationship, error) {
	if limit <= 0 {
		limit = 50
	}

	var (
		rows pgxRows
		err  error
	)
	if relationshipType != nil && *relationshipType != "" {
		rows, err = s.db.Query(ctx, `
			SELECT source_work_id, target_work_id, relationship_type, confidence, provider, created_at, updated_at
			FROM work_relationships
			WHERE source_work_id = $1 AND relationship_type = $2
			ORDER BY confidence DESC, updated_at DESC
			LIMIT $3`,
			sourceID, *relationshipType, limit,
		)
	} else {
		rows, err = s.db.Query(ctx, `
			SELECT source_work_id, target_work_id, relationship_type, confidence, provider, created_at, updated_at
			FROM work_relationships
			WHERE source_work_id = $1
			ORDER BY confidence DESC, updated_at DESC
			LIMIT $2`,
			sourceID, limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []model.WorkRelationship
	for rows.Next() {
		var relation model.WorkRelationship
		if scanErr := rows.Scan(
			&relation.SourceWorkID,
			&relation.TargetWorkID,
			&relation.RelationshipType,
			&relation.Confidence,
			&relation.Provider,
			&relation.CreatedAt,
			&relation.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		relationships = append(relationships, relation)
	}
	return relationships, rows.Err()
}

func (s *pgWorkRelationshipStore) DeleteRelationshipsForWork(ctx context.Context, sourceID string, relationshipType *string) error {
	if relationshipType != nil && *relationshipType != "" {
		_, err := s.db.Exec(ctx, `
			DELETE FROM work_relationships
			WHERE source_work_id = $1 AND relationship_type = $2`,
			sourceID, *relationshipType,
		)
		return err
	}

	_, err := s.db.Exec(ctx, `
		DELETE FROM work_relationships
		WHERE source_work_id = $1`,
		sourceID,
	)
	return err
}

func (s *pgWorkRelationshipStore) CountRelationshipsByType(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.Query(ctx, `
		SELECT relationship_type, COUNT(*)
		FROM work_relationships
		GROUP BY relationship_type`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int64{}
	for rows.Next() {
		var relationshipType string
		var count int64
		if scanErr := rows.Scan(&relationshipType, &count); scanErr != nil {
			return nil, scanErr
		}
		out[relationshipType] = count
	}
	return out, rows.Err()
}

type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}
