package provider

import (
	"sort"
	"sync"
	"time"
)

const defaultProviderTimeout = 10 * time.Second

type registeredProvider struct {
	provider Provider
	priority int
	enabled  bool
	timeout  time.Duration
}

type Registry struct {
	mu        sync.RWMutex
	providers map[string]*registeredProvider
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
	}
	var entries []entry
	for _, rp := range r.providers {
		if rp.enabled {
			entries = append(entries, entry{rp.provider, rp.priority})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
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
