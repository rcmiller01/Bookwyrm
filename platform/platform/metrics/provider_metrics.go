package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ProviderSuccessTotal counts successful provider responses (distinct from
	// ProviderRequestsTotal which includes all attempts).
	ProviderSuccessTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bookwyrm_provider_success_total",
		Help: "Total successful responses from each provider.",
	}, []string{"provider"})

	// ProviderReliabilityScore tracks the current composite reliability score
	// per provider as computed by the reliability worker.
	ProviderReliabilityScore = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bookwyrm_provider_reliability_score",
		Help: "Current composite reliability score per provider (0.0 to 1.0).",
	}, []string{"provider"})
)
