package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProviderMetrics holds accumulated raw performance signals for a single provider.
type ProviderMetrics struct {
	Provider             string
	SuccessCount         int64
	FailureCount         int64
	TotalLatencyMs       int64
	RequestCount         int64
	IdentifierMatches    int64
	IdentifierIntroduced int64
	LastSuccess          *time.Time
	LastFailure          *time.Time
}

// ProviderMetricsStore manages provider_metrics rows.
type ProviderMetricsStore interface {
	RecordSuccess(ctx context.Context, providerName string, latency time.Duration) error
	RecordFailure(ctx context.Context, providerName string) error
	RecordIdentifierMatch(ctx context.Context, providerName string) error
	RecordIdentifierIntroduced(ctx context.Context, providerName string) error
	GetMetrics(ctx context.Context, providerName string) (*ProviderMetrics, error)
	GetAllMetrics(ctx context.Context) ([]ProviderMetrics, error)
}

// pgProviderMetricsStore is a PostgreSQL-backed ProviderMetricsStore.
type pgProviderMetricsStore struct {
	db *pgxpool.Pool
}

// NewProviderMetricsStore returns a PostgreSQL-backed ProviderMetricsStore.
func NewProviderMetricsStore(db *pgxpool.Pool) ProviderMetricsStore {
	return &pgProviderMetricsStore{db: db}
}

func (s *pgProviderMetricsStore) RecordSuccess(ctx context.Context, providerName string, latency time.Duration) error {
	latencyMs := latency.Milliseconds()
	_, err := s.db.Exec(ctx, `
		INSERT INTO provider_metrics
		    (provider, success_count, request_count, total_latency_ms, last_success, updated_at)
		VALUES ($1, 1, 1, $2, NOW(), NOW())
		ON CONFLICT (provider) DO UPDATE SET
		    success_count    = provider_metrics.success_count    + 1,
		    request_count    = provider_metrics.request_count    + 1,
		    total_latency_ms = provider_metrics.total_latency_ms + $2,
		    last_success     = NOW(),
		    updated_at       = NOW()`,
		providerName, latencyMs,
	)
	return err
}

func (s *pgProviderMetricsStore) RecordFailure(ctx context.Context, providerName string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO provider_metrics
		    (provider, failure_count, request_count, last_failure, updated_at)
		VALUES ($1, 1, 1, NOW(), NOW())
		ON CONFLICT (provider) DO UPDATE SET
		    failure_count = provider_metrics.failure_count + 1,
		    request_count = provider_metrics.request_count + 1,
		    last_failure  = NOW(),
		    updated_at    = NOW()`,
		providerName,
	)
	return err
}

func (s *pgProviderMetricsStore) RecordIdentifierMatch(ctx context.Context, providerName string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO provider_metrics (provider, identifier_matches, updated_at)
		VALUES ($1, 1, NOW())
		ON CONFLICT (provider) DO UPDATE SET
		    identifier_matches = provider_metrics.identifier_matches + 1,
		    updated_at         = NOW()`,
		providerName,
	)
	return err
}

func (s *pgProviderMetricsStore) RecordIdentifierIntroduced(ctx context.Context, providerName string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO provider_metrics (provider, identifier_introduced, updated_at)
		VALUES ($1, 1, NOW())
		ON CONFLICT (provider) DO UPDATE SET
		    identifier_introduced = provider_metrics.identifier_introduced + 1,
		    updated_at            = NOW()`,
		providerName,
	)
	return err
}

func (s *pgProviderMetricsStore) GetMetrics(ctx context.Context, providerName string) (*ProviderMetrics, error) {
	row := s.db.QueryRow(ctx, `
		SELECT provider, success_count, failure_count, total_latency_ms, request_count,
		       identifier_matches, identifier_introduced, last_success, last_failure
		FROM provider_metrics
		WHERE provider = $1`,
		providerName,
	)
	var m ProviderMetrics
	if err := row.Scan(
		&m.Provider, &m.SuccessCount, &m.FailureCount, &m.TotalLatencyMs, &m.RequestCount,
		&m.IdentifierMatches, &m.IdentifierIntroduced, &m.LastSuccess, &m.LastFailure,
	); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *pgProviderMetricsStore) GetAllMetrics(ctx context.Context) ([]ProviderMetrics, error) {
	rows, err := s.db.Query(ctx, `
		SELECT provider, success_count, failure_count, total_latency_ms, request_count,
		       identifier_matches, identifier_introduced, last_success, last_failure
		FROM provider_metrics
		ORDER BY provider ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProviderMetrics
	for rows.Next() {
		var m ProviderMetrics
		if err := rows.Scan(
			&m.Provider, &m.SuccessCount, &m.FailureCount, &m.TotalLatencyMs, &m.RequestCount,
			&m.IdentifierMatches, &m.IdentifierIntroduced, &m.LastSuccess, &m.LastFailure,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
