package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RecommendRequestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "recommend_requests_total",
		Help: "Total recommendation requests served.",
	})

	RecommendCacheHitsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "recommend_cache_hits_total",
		Help: "Total recommendation cache hits.",
	})

	RecommendCacheMissesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "recommend_cache_misses_total",
		Help: "Total recommendation cache misses.",
	})

	RecommendLatencySeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "recommend_latency_seconds",
		Help:    "Recommendation request latency in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	RecommendCandidatesGeneratedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "recommend_candidates_generated_total",
		Help: "Total recommendation candidates generated before final ranking.",
	})

	RecommendResultsReturnedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "recommend_results_returned_total",
		Help: "Total recommendation results returned.",
	})
)
