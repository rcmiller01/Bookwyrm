package metrics

import platformmetrics "bookwyrm/platform/metrics"

var (
	// ProviderSuccessTotal counts successful provider responses (distinct from
	// ProviderRequestsTotal which includes all attempts).
	ProviderSuccessTotal = platformmetrics.ProviderSuccessTotal

	// ProviderReliabilityScore tracks the current composite reliability score
	// per provider as computed by the reliability worker.
	ProviderReliabilityScore = platformmetrics.ProviderReliabilityScore
)
