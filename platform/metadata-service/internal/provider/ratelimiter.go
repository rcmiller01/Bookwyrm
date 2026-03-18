package provider

import (
	"sync"
	"time"
)

// RateLimiter implements a simple token bucket per provider.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
}

type bucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{buckets: make(map[string]*bucket)}
}

// Configure sets a provider's rate limit (requests per minute).
func (rl *RateLimiter) Configure(name string, requestsPerMinute int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rps := float64(requestsPerMinute) / 60.0
	rl.buckets[name] = &bucket{
		tokens:     float64(requestsPerMinute),
		maxTokens:  float64(requestsPerMinute),
		refillRate: rps,
		lastRefill: time.Now(),
	}
}

// Allow returns true if the provider has available token capacity.
func (rl *RateLimiter) Allow(name string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[name]
	if !ok {
		return true // unconfigured providers are not rate-limited
	}

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(b.maxTokens, b.tokens+elapsed*b.refillRate)
	b.lastRefill = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}
	return false
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
