package store

import (
	"testing"
	"time"
)

func TestNextBackoff_IncreasesByAttempt(t *testing.T) {
	b1 := nextBackoff(1)
	b2 := nextBackoff(2)
	b3 := nextBackoff(3)

	if b1 <= 0 {
		t.Fatalf("expected positive backoff for attempt 1, got %v", b1)
	}
	if b2 < b1 {
		t.Fatalf("expected backoff to grow for attempt 2 (%v < %v)", b2, b1)
	}
	if b3 < b2 {
		t.Fatalf("expected backoff to grow for attempt 3 (%v < %v)", b3, b2)
	}
}

func TestNextBackoff_IsCapped(t *testing.T) {
	b := nextBackoff(100)
	if b > maxEnrichmentBackoff {
		t.Fatalf("expected capped backoff <= %v, got %v", maxEnrichmentBackoff, b)
	}

	// Jitter can be zero, but we still expect long-attempt backoff to be near the cap.
	if b < time.Hour {
		t.Fatalf("expected large capped backoff for very high attempt, got %v", b)
	}
}
