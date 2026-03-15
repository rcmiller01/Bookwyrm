package metrics

import platformmetrics "bookwyrm/platform/metrics"

var (
	ProviderRequestsTotal = platformmetrics.ProviderRequestsTotal
	ProviderFailuresTotal = platformmetrics.ProviderFailuresTotal
	ProviderLatencyMs     = platformmetrics.ProviderLatencyMs
	ResolverRequestsTotal = platformmetrics.ResolverRequestsTotal
	CacheHitsTotal        = platformmetrics.CacheHitsTotal
	CacheMissesTotal      = platformmetrics.CacheMissesTotal
	ResolverLatencyMs     = platformmetrics.ResolverLatencyMs
)
