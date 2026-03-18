package provider

import (
	"context"
	"time"

	"metadata-service/internal/store"

	"github.com/rs/zerolog/log"
)

// HealthMonitor periodically pings providers and updates their status.
// It maps the composite reliability score to a human-readable status string.
type HealthMonitor struct {
	registry         *Registry
	statusStore      store.ProviderStatusStore
	reliabilityStore store.ReliabilityStore // may be nil before Phase 3 migration
	interval         time.Duration
}

func NewHealthMonitor(registry *Registry, statusStore store.ProviderStatusStore, interval time.Duration) *HealthMonitor {
	return &HealthMonitor{
		registry:    registry,
		statusStore: statusStore,
		interval:    interval,
	}
}

// WithReliabilityStore attaches a reliability store so the monitor can map
// composite scores to health statuses.
func (m *HealthMonitor) WithReliabilityStore(rs store.ReliabilityStore) *HealthMonitor {
	m.reliabilityStore = rs
	return m
}

// Start launches the background health check loop. Call with go monitor.Start(ctx).
func (m *HealthMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	log.Info().Dur("interval", m.interval).Msg("provider health monitor started")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("provider health monitor stopped")
			return
		case <-ticker.C:
			m.checkAll(ctx)
		}
	}
}

func (m *HealthMonitor) checkAll(ctx context.Context) {
	providers := m.registry.EnabledProviders()
	for _, p := range providers {
		go m.checkOne(ctx, p)
	}
}

func (m *HealthMonitor) checkOne(ctx context.Context, p Provider) {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	start := time.Now()
	_, err := p.SearchWorks(checkCtx, "test")
	latency := time.Since(start).Milliseconds()

	if err != nil {
		log.Warn().Err(err).Str("provider", p.Name()).Msg("health check failed")
		if dbErr := m.statusStore.RecordFailure(ctx, p.Name()); dbErr != nil {
			log.Error().Err(dbErr).Str("provider", p.Name()).Msg("failed to record provider failure")
		}
		return
	}

	if dbErr := m.statusStore.RecordSuccess(ctx, p.Name(), latency); dbErr != nil {
		log.Error().Err(dbErr).Str("provider", p.Name()).Msg("failed to record provider success")
	}
	log.Debug().Str("provider", p.Name()).Int64("latency_ms", latency).Msg("health check passed")

	// If we have reliability scores, update the provider_status table with the
	// score-derived status label so the existing provider management API reflects
	// Phase 3 thresholds:  >0.80 healthy | 0.60-0.80 degraded | 0.40-0.60 unreliable | <0.40 quarantine
	if m.reliabilityStore == nil {
		return
	}
	score, err := m.reliabilityStore.GetScore(ctx, p.Name())
	if err != nil {
		return // score not yet computed — leave status as-is
	}
	status := HealthStatus(score.CompositeScore)
	if updateErr := m.statusStore.UpdateStatus(ctx, p.Name(), status, 0, latency); updateErr != nil {
		log.Warn().Err(updateErr).Str("provider", p.Name()).Msg("failed to update provider status from reliability score")
	}
}
