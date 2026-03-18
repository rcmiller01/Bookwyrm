package metrics

import platformmetrics "bookwyrm/platform/metrics"

var (
	RecommendRequestsTotal            = platformmetrics.RecommendRequestsTotal
	RecommendCacheHitsTotal           = platformmetrics.RecommendCacheHitsTotal
	RecommendCacheMissesTotal         = platformmetrics.RecommendCacheMissesTotal
	RecommendLatencySeconds           = platformmetrics.RecommendLatencySeconds
	RecommendCandidatesGeneratedTotal = platformmetrics.RecommendCandidatesGeneratedTotal
	RecommendResultsReturnedTotal     = platformmetrics.RecommendResultsReturnedTotal
)
