package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProviderConfig holds database-driven configuration for a provider.
type ProviderConfig struct {
	ID         int
	Name       string
	Enabled    bool
	Priority   int
	TimeoutSec int
	RateLimit  int // requests per minute
	APIKey     string
	UpdatedAt  time.Time
}

// ProviderStatus holds runtime health state for a provider.
type ProviderStatus struct {
	Name         string
	Status       string // healthy, degraded, unreliable, quarantine, disabled
	FailureCount int
	LastSuccess  *time.Time
	LastFailure  *time.Time
	LastChecked  time.Time
	AvgLatencyMs int64
}

// ProviderConfigStore manages provider_configs rows.
type ProviderConfigStore interface {
	GetAll(ctx context.Context) ([]ProviderConfig, error)
	GetByName(ctx context.Context, name string) (*ProviderConfig, error)
	Upsert(ctx context.Context, cfg ProviderConfig) error
}

// ProviderStatusStore manages provider_status rows.
type ProviderStatusStore interface {
	GetAll(ctx context.Context) ([]ProviderStatus, error)
	GetByName(ctx context.Context, name string) (*ProviderStatus, error)
	UpdateStatus(ctx context.Context, name string, status string, failureCount int, latencyMs int64) error
	RecordSuccess(ctx context.Context, name string, latencyMs int64) error
	RecordFailure(ctx context.Context, name string) error
}

// --- ProviderConfigStore implementation ---

type pgProviderConfigStore struct {
	db *pgxpool.Pool
}

func NewProviderConfigStore(db *pgxpool.Pool) ProviderConfigStore {
	return &pgProviderConfigStore{db: db}
}

func (s *pgProviderConfigStore) GetAll(ctx context.Context) ([]ProviderConfig, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, name, enabled, priority, timeout_sec, rate_limit, COALESCE(api_key,''), updated_at
		 FROM provider_configs ORDER BY priority ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cfgs []ProviderConfig
	for rows.Next() {
		var c ProviderConfig
		if err := rows.Scan(&c.ID, &c.Name, &c.Enabled, &c.Priority, &c.TimeoutSec, &c.RateLimit, &c.APIKey, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cfgs = append(cfgs, c)
	}
	return cfgs, rows.Err()
}

func (s *pgProviderConfigStore) GetByName(ctx context.Context, name string) (*ProviderConfig, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, name, enabled, priority, timeout_sec, rate_limit, COALESCE(api_key,''), updated_at
		 FROM provider_configs WHERE name = $1`, name)
	var c ProviderConfig
	if err := row.Scan(&c.ID, &c.Name, &c.Enabled, &c.Priority, &c.TimeoutSec, &c.RateLimit, &c.APIKey, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *pgProviderConfigStore) Upsert(ctx context.Context, cfg ProviderConfig) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO provider_configs (name, enabled, priority, timeout_sec, rate_limit, api_key, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (name) DO UPDATE SET
		   enabled = EXCLUDED.enabled,
		   priority = EXCLUDED.priority,
		   timeout_sec = EXCLUDED.timeout_sec,
		   rate_limit = EXCLUDED.rate_limit,
		   api_key = EXCLUDED.api_key,
		   updated_at = NOW()`,
		cfg.Name, cfg.Enabled, cfg.Priority, cfg.TimeoutSec, cfg.RateLimit, cfg.APIKey,
	)
	return err
}

// --- ProviderStatusStore implementation ---

type pgProviderStatusStore struct {
	db *pgxpool.Pool
}

func NewProviderStatusStore(db *pgxpool.Pool) ProviderStatusStore {
	return &pgProviderStatusStore{db: db}
}

func (s *pgProviderStatusStore) GetAll(ctx context.Context) ([]ProviderStatus, error) {
	rows, err := s.db.Query(ctx,
		`SELECT name, status, failure_count, last_success, last_failure, last_checked, avg_latency_ms
		 FROM provider_status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []ProviderStatus
	for rows.Next() {
		var ps ProviderStatus
		if err := rows.Scan(&ps.Name, &ps.Status, &ps.FailureCount, &ps.LastSuccess, &ps.LastFailure, &ps.LastChecked, &ps.AvgLatencyMs); err != nil {
			return nil, err
		}
		statuses = append(statuses, ps)
	}
	return statuses, rows.Err()
}

func (s *pgProviderStatusStore) GetByName(ctx context.Context, name string) (*ProviderStatus, error) {
	row := s.db.QueryRow(ctx,
		`SELECT name, status, failure_count, last_success, last_failure, last_checked, avg_latency_ms
		 FROM provider_status WHERE name = $1`, name)
	var ps ProviderStatus
	if err := row.Scan(&ps.Name, &ps.Status, &ps.FailureCount, &ps.LastSuccess, &ps.LastFailure, &ps.LastChecked, &ps.AvgLatencyMs); err != nil {
		return nil, err
	}
	return &ps, nil
}

func (s *pgProviderStatusStore) UpdateStatus(ctx context.Context, name string, status string, failureCount int, latencyMs int64) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO provider_status (name, status, failure_count, avg_latency_ms, last_checked, updated_at)
		 VALUES ($1, $2, $3, $4, NOW(), NOW())
		 ON CONFLICT (name) DO UPDATE SET
		   status = EXCLUDED.status,
		   failure_count = EXCLUDED.failure_count,
		   avg_latency_ms = EXCLUDED.avg_latency_ms,
		   last_checked = NOW(),
		   updated_at = NOW()`,
		name, status, failureCount, latencyMs,
	)
	return err
}

func (s *pgProviderStatusStore) RecordSuccess(ctx context.Context, name string, latencyMs int64) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO provider_status (name, status, failure_count, last_success, avg_latency_ms, last_checked, updated_at)
		 VALUES ($1, 'healthy', 0, NOW(), $2, NOW(), NOW())
		 ON CONFLICT (name) DO UPDATE SET
		   status = CASE WHEN provider_status.failure_count > 0 THEN 'degraded' ELSE 'healthy' END,
		   failure_count = GREATEST(0, provider_status.failure_count - 1),
		   last_success = NOW(),
		   avg_latency_ms = ($2 + provider_status.avg_latency_ms) / 2,
		   last_checked = NOW(),
		   updated_at = NOW()`,
		name, latencyMs,
	)
	return err
}

func (s *pgProviderStatusStore) RecordFailure(ctx context.Context, name string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO provider_status (name, status, failure_count, last_failure, last_checked, updated_at)
		 VALUES ($1, 'degraded', 1, NOW(), NOW(), NOW())
		 ON CONFLICT (name) DO UPDATE SET
		   failure_count = provider_status.failure_count + 1,
		   status = CASE
		     WHEN provider_status.failure_count + 1 >= 10 THEN 'unreliable'
		     WHEN provider_status.failure_count + 1 >= 3  THEN 'degraded'
		     ELSE provider_status.status
		   END,
		   last_failure = NOW(),
		   last_checked = NOW(),
		   updated_at = NOW()`,
		name,
	)
	return err
}
