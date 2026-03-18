package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ProviderRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bookwyrm_provider_requests_total",
		Help: "Total number of requests sent to each provider.",
	}, []string{"provider"})

	ProviderFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bookwyrm_provider_failures_total",
		Help: "Total number of failed requests per provider.",
	}, []string{"provider"})

	ProviderLatencyMs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bookwyrm_provider_latency_ms",
		Help:    "Latency of provider requests in milliseconds.",
		Buckets: []float64{50, 100, 250, 500, 1000, 2000, 5000},
	}, []string{"provider"})

	ResolverRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bookwyrm_resolver_requests_total",
		Help: "Total number of resolver search requests.",
	})

	CacheHitsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bookwyrm_cache_hits_total",
		Help: "Total number of cache hits.",
	})

	CacheMissesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bookwyrm_cache_misses_total",
		Help: "Total number of cache misses.",
	})

	ResolverLatencyMs = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "bookwyrm_resolver_latency_ms",
		Help:    "End-to-end resolver latency in milliseconds.",
		Buckets: []float64{50, 100, 250, 500, 1000, 2000, 5000},
	})
)
