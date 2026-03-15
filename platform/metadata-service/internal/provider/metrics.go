package provider

import "metadata-service/internal/store"

// ProviderMetrics is a type alias for the store-layer provider metrics struct,
// exposed here so callers can reference provider.ProviderMetrics without
// importing the store package directly.
type ProviderMetrics = store.ProviderMetrics
