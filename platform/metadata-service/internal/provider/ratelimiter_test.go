package provider

import (
	"testing"
	"time"
)

func TestRateLimiter_Unconfigured_AlwaysAllows(t *testing.T) {
	rl := NewRateLimiter()
	for i := 0; i < 100; i++ {
		if !rl.Allow("unknown") {
			t.Errorf("unconfigured provider should always be allowed (call %d)", i)
		}
	}
}

func TestRateLimiter_ExhaustsTokens(t *testing.T) {
	rl := NewRateLimiter()
	rl.Configure("p", 3) // 3 requests/minute → starts with 3 tokens

	allowed := 0
	for i := 0; i < 10; i++ {
		if rl.Allow("p") {
			allowed++
		}
	}
	if allowed != 3 {
		t.Errorf("expected exactly 3 allowed calls before exhaustion, got %d", allowed)
	}
}

func TestRateLimiter_DeniesWhenExhausted(t *testing.T) {
	rl := NewRateLimiter()
	rl.Configure("q", 1)

	if !rl.Allow("q") {
		t.Fatal("first call should succeed")
	}
	if rl.Allow("q") {
		t.Error("second call should be denied when bucket is empty")
	}
}

func TestRateLimiter_RefillsOverTime(t *testing.T) {
	rl := NewRateLimiter()
	// 60 req/min = 1 req/second refill rate.
	rl.Configure("r", 60)

	// exhaust all 60 tokens
	for i := 0; i < 60; i++ {
		rl.Allow("r")
	}
	if rl.Allow("r") {
		t.Fatal("bucket should be empty after 60 calls")
	}

	// wait just over 1 second for ~1 token to refill
	time.Sleep(1100 * time.Millisecond)

	if !rl.Allow("r") {
		t.Error("bucket should have refilled at least 1 token after 1s")
	}
}

func TestRateLimiter_MultipleProviders_Independent(t *testing.T) {
	rl := NewRateLimiter()
	rl.Configure("a", 1)
	rl.Configure("b", 5)

	// exhaust a
	rl.Allow("a")
	if rl.Allow("a") {
		t.Error("provider a should be exhausted")
	}

	// b should still have tokens
	if !rl.Allow("b") {
		t.Error("provider b should still allow requests")
	}
}

func TestRateLimiter_Reconfigure(t *testing.T) {
	rl := NewRateLimiter()
	rl.Configure("s", 1)
	rl.Allow("s") // exhaust

	// reconfigure with higher limit: resets bucket
	rl.Configure("s", 10)
	if !rl.Allow("s") {
		t.Error("reconfigured provider should allow requests again")
	}
}

func TestRateLimiter_ZeroRate_AlwaysDenies(t *testing.T) {
	rl := NewRateLimiter()
	rl.Configure("blocked", 0) // 0 tokens/min → never allows

	for i := 0; i < 5; i++ {
		if rl.Allow("blocked") {
			t.Errorf("provider with 0 rate should always be denied (call %d)", i)
		}
	}
}
