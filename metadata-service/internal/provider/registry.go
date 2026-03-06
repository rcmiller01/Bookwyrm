package provider

import (
	"sort"
	"sync"
	"time"

	"metadata-service/internal/policy"
)

const defaultProviderTimeout = 10 * time.Second

// DispatchTier defines the reliability-based execution tier for providers.
// Lower values are dispatched earlier.
type DispatchTier = policy.Tier

const (
	DispatchTierPrimary      = policy.TierPrimary
	DispatchTierSecondary    = policy.TierSecondary
	DispatchTierFallback     = policy.TierFallback
	DispatchTierQuarantine   = policy.TierQuarantine
	DispatchTierUnclassified = policy.TierUnclassified // score unavailable; fallback to configured priority
)

type registeredProvider struct {
	provider         Provider
	priority         int
	enabled          bool
	timeout          time.Duration
	hasScore         bool
	reliabilityScore float64
	tier             DispatchTier
}

type Registry struct {
	mu                 sync.RWMutex
	providers          map[string]*registeredProvider
	quarantineDisables bool
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]*registeredProvider)}
}

// Register adds a provider with default priority 100 and enabled=true.
func (r *Registry) Register(p Provider) {
	r.RegisterWithConfig(p, 100, true)
}

// RegisterWithConfig adds a provider with explicit priority and enabled state.
// Call SetTimeout after registration to override the default 10s timeout.
func (r *Registry) RegisterWithConfig(p Provider, priority int, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = &registeredProvider{
		provider: p,
		priority: priority,
		enabled:  enabled,
		timeout:  defaultProviderTimeout,
		tier:     DispatchTierUnclassified,
	}
}

// SetQuarantineDisables controls whether quarantine-tier providers are skipped.
// When false (default), quarantine providers remain enabled as last-resort sources.
func (r *Registry) SetQuarantineDisables(disable bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.quarantineDisables = disable
}

// QuarantineDisables returns whether quarantine-tier providers are skipped.
func (r *Registry) QuarantineDisables() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.quarantineDisables
}

// SetReliability updates a provider's runtime reliability metadata used for
// dispatch ordering.
func (r *Registry) SetReliability(name string, score float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rp, ok := r.providers[name]; ok {
		rp.hasScore = true
		rp.reliabilityScore = score
		rp.tier = policy.TierForScore(score)
	}
}

// SetEnabled toggles a provider's enabled state at runtime.
func (r *Registry) SetEnabled(name string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rp, ok := r.providers[name]; ok {
		rp.enabled = enabled
	}
}

// SetPriority updates a provider's execution priority at runtime.
// Lower numbers are dispatched first by EnabledProviders.
func (r *Registry) SetPriority(name string, priority int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rp, ok := r.providers[name]; ok {
		rp.priority = priority
	}
}

// SetTimeout stores the per-provider context deadline used by the resolver.
func (r *Registry) SetTimeout(name string, d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rp, ok := r.providers[name]; ok {
		rp.timeout = d
	}
}

// TimeoutFor returns the configured timeout for a provider.
// Falls back to defaultProviderTimeout if the provider is unknown.
func (r *Registry) TimeoutFor(name string) time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if rp, ok := r.providers[name]; ok {
		return rp.timeout
	}
	return defaultProviderTimeout
}

func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if rp, ok := r.providers[name]; ok {
		return rp.provider, true
	}
	return nil, false
}

// EnabledProviders returns all enabled providers sorted by priority (lowest first).
func (r *Registry) EnabledProviders() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	type entry struct {
		p        Provider
		priority int
		hasScore bool
		score    float64
		tier     DispatchTier
	}
	var entries []entry
	for _, rp := range r.providers {
		if !rp.enabled {
			continue
		}
		if r.quarantineDisables && rp.tier == DispatchTierQuarantine {
			continue
		}
		entries = append(entries, entry{rp.provider, rp.priority, rp.hasScore, rp.reliabilityScore, rp.tier})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].hasScore && entries[j].hasScore {
			left := policy.DispatchSortKey(entries[i].tier, entries[i].score, entries[i].priority)
			right := policy.DispatchSortKey(entries[j].tier, entries[j].score, entries[j].priority)
			return left.Less(right)
		}
		return entries[i].priority < entries[j].priority
	})

	result := make([]Provider, len(entries))
	for i, e := range entries {
		result[i] = e.p
	}
	return result
}

// AllProviders returns all providers regardless of enabled state.
func (r *Registry) AllProviders() map[string]*registeredProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*registeredProvider, len(r.providers))
	for k, v := range r.providers {
		out[k] = v
	}
	return out
}
