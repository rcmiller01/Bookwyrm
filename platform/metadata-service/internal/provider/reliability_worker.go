package provider

import (
	"context"
	"time"

	"metadata-service/internal/metrics"
	"metadata-service/internal/store"

	"github.com/rs/zerolog/log"
)

// ReliabilityWorker periodically recomputes provider reliability scores from
// accumulated metrics and persists them to the provider_reliability table.
// It also updates in-memory dispatch tier metadata in the registry.
type ReliabilityWorker struct {
	metricsStore     store.ProviderMetricsStore
	reliabilityStore store.ReliabilityStore
	registry         *Registry
	interval         time.Duration
}

// NewReliabilityWorker creates a ReliabilityWorker that ticks at interval.
func NewReliabilityWorker(
	metricsStore store.ProviderMetricsStore,
	reliabilityStore store.ReliabilityStore,
	registry *Registry,
	interval time.Duration,
) *ReliabilityWorker {
	return &ReliabilityWorker{
		metricsStore:     metricsStore,
		reliabilityStore: reliabilityStore,
		registry:         registry,
		interval:         interval,
	}
}

// Start runs the reliability update loop. Call with go worker.Start(ctx).
// It fires once immediately on start so scores exist before the first tick.
func (w *ReliabilityWorker) Start(ctx context.Context) {
	log.Info().Dur("interval", w.interval).Msg("reliability worker started")

	// fire immediately; then continue on interval
	w.run(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("reliability worker stopped")
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

func (w *ReliabilityWorker) run(ctx context.Context) {
	allMetrics, err := w.metricsStore.GetAllMetrics(ctx)
	if err != nil {
		log.Error().Err(err).Msg("reliability worker: failed to fetch provider metrics")
		return
	}

	for _, m := range allMetrics {
		score := ComputeScore(m)

		if err := w.reliabilityStore.UpdateScore(ctx, score); err != nil {
			log.Error().Err(err).Str("provider", m.Provider).
				Msg("reliability worker: failed to persist reliability score")
			continue
		}

		// Expose score to Prometheus.
		metrics.ProviderReliabilityScore.WithLabelValues(m.Provider).Set(score.CompositeScore)

		// Update registry ordering metadata.
		// Quarantine providers remain enabled by default and are dispatched last.
		// Operators may opt into skipping quarantine by calling SetQuarantineDisables(true).
		w.registry.SetReliability(m.Provider, score.CompositeScore)
		status := HealthStatus(score.CompositeScore)

		log.Debug().
			Str("provider", m.Provider).
			Float64("score", score.CompositeScore).
			Str("status", status).
			Msg("reliability score updated")
	}
}
