package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ReliabilityScore holds the computed composite reliability score for a provider.
type ReliabilityScore struct {
	Provider          string
	Availability      float64
	LatencyScore      float64
	AgreementScore    float64
	IdentifierQuality float64
	CompositeScore    float64
	UpdatedAt         time.Time
}

// ReliabilityStore manages provider_reliability rows.
type ReliabilityStore interface {
	GetScore(ctx context.Context, providerName string) (*ReliabilityScore, error)
	GetAllScores(ctx context.Context) ([]ReliabilityScore, error)
	UpdateScore(ctx context.Context, score ReliabilityScore) error
}

// pgReliabilityStore is a PostgreSQL-backed ReliabilityStore.
type pgReliabilityStore struct {
	db *pgxpool.Pool
}

// NewReliabilityStore returns a PostgreSQL-backed ReliabilityStore.
func NewReliabilityStore(db *pgxpool.Pool) ReliabilityStore {
	return &pgReliabilityStore{db: db}
}

func (s *pgReliabilityStore) GetScore(ctx context.Context, providerName string) (*ReliabilityScore, error) {
	row := s.db.QueryRow(ctx, `
		SELECT provider, availability, latency_score, agreement_score,
		       identifier_quality, composite_score, updated_at
		FROM provider_reliability
		WHERE provider = $1`,
		providerName,
	)
	var rs ReliabilityScore
	if err := row.Scan(
		&rs.Provider, &rs.Availability, &rs.LatencyScore, &rs.AgreementScore,
		&rs.IdentifierQuality, &rs.CompositeScore, &rs.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &rs, nil
}

func (s *pgReliabilityStore) GetAllScores(ctx context.Context) ([]ReliabilityScore, error) {
	rows, err := s.db.Query(ctx, `
		SELECT provider, availability, latency_score, agreement_score,
		       identifier_quality, composite_score, updated_at
		FROM provider_reliability
		ORDER BY composite_score DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ReliabilityScore
	for rows.Next() {
		var rs ReliabilityScore
		if err := rows.Scan(
			&rs.Provider, &rs.Availability, &rs.LatencyScore, &rs.AgreementScore,
			&rs.IdentifierQuality, &rs.CompositeScore, &rs.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rs)
	}
	return out, rows.Err()
}

func (s *pgReliabilityStore) UpdateScore(ctx context.Context, score ReliabilityScore) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO provider_reliability
		    (provider, availability, latency_score, agreement_score,
		     identifier_quality, composite_score, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (provider) DO UPDATE SET
		    availability       = EXCLUDED.availability,
		    latency_score      = EXCLUDED.latency_score,
		    agreement_score    = EXCLUDED.agreement_score,
		    identifier_quality = EXCLUDED.identifier_quality,
		    composite_score    = EXCLUDED.composite_score,
		    updated_at         = NOW()`,
		score.Provider, score.Availability, score.LatencyScore, score.AgreementScore,
		score.IdentifierQuality, score.CompositeScore,
	)
	return err
}
