package provider

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"metadata-service/internal/store"
)

// HealthMonitor periodically pings providers and updates their status.
type HealthMonitor struct {
	registry    *Registry
	statusStore store.ProviderStatusStore
	interval    time.Duration
}

func NewHealthMonitor(registry *Registry, statusStore store.ProviderStatusStore, interval time.Duration) *HealthMonitor {
	return &HealthMonitor{
		registry:    registry,
		statusStore: statusStore,
		interval:    interval,
	}
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
}
