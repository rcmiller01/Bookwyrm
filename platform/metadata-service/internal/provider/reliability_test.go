package provider

import (
	"testing"
	"time"

	"metadata-service/internal/store"
)

func TestComputeScore_FullData(t *testing.T) {
	now := time.Now()
	m := store.ProviderMetrics{
		Provider:             "openlibrary",
		SuccessCount:         90,
		FailureCount:         10,
		TotalLatencyMs:       90_000, // avg 900 ms
		RequestCount:         100,
		IdentifierMatches:    45,
		IdentifierIntroduced: 50,
		LastSuccess:          &now,
	}

	score := ComputeScore(m)

	if score.Provider != "openlibrary" {
		t.Errorf("expected provider openlibrary, got %s", score.Provider)
	}

	// availability = 90/100 = 0.90
	if score.Availability < 0.89 || score.Availability > 0.91 {
		t.Errorf("availability = %f, want ~0.90", score.Availability)
	}

	// latency_score = 1 - (900/2000) = 0.55
	if score.LatencyScore < 0.54 || score.LatencyScore > 0.56 {
		t.Errorf("latency_score = %f, want ~0.55", score.LatencyScore)
	}

	// agreement = 45/50 = 0.90
	if score.AgreementScore < 0.89 || score.AgreementScore > 0.91 {
		t.Errorf("agreement_score = %f, want ~0.90", score.AgreementScore)
	}

	// composite = 0.90*0.35 + 0.90*0.35 + 0.55*0.15 + 0.90*0.15
	//           = 0.315 + 0.315 + 0.0825 + 0.135 = 0.8475
	if score.CompositeScore < 0.84 || score.CompositeScore > 0.86 {
		t.Errorf("composite score = %f, want ~0.8475", score.CompositeScore)
	}
}

func TestComputeScore_ZeroRequests(t *testing.T) {
	m := store.ProviderMetrics{
		Provider:     "newprovider",
		RequestCount: 0,
	}
	score := ComputeScore(m)

	// all fields should fall back to DecayBaseline = 0.7
	if score.Availability != DecayBaseline {
		t.Errorf("expected availability %f for zero requests, got %f", DecayBaseline, score.Availability)
	}
	if score.CompositeScore != DecayBaseline {
		t.Errorf("expected composite %f for zero requests, got %f", DecayBaseline, score.CompositeScore)
	}
}

func TestComputeScore_Clamping(t *testing.T) {
	now := time.Now()
	// simulate extremely fast provider (would produce latency_score > 1)
	m := store.ProviderMetrics{
		Provider:       "fast",
		SuccessCount:   100,
		RequestCount:   100,
		TotalLatencyMs: 10, // avg 0.1 ms — score would be > 1 without clamping
		LastSuccess:    &now,
	}
	score := ComputeScore(m)

	if score.LatencyScore > 1.0 {
		t.Errorf("latency_score %f exceeds 1.0 (clamping failed)", score.LatencyScore)
	}
	if score.CompositeScore > 1.0 {
		t.Errorf("composite score %f exceeds 1.0 (clamping failed)", score.CompositeScore)
	}
	if score.CompositeScore < 0.0 {
		t.Errorf("composite score %f is below 0.0 (clamping failed)", score.CompositeScore)
	}
}

func TestComputeScore_Decay(t *testing.T) {
	// last success was 61 days ago — well past the 30-day threshold
	lastSuccess := time.Now().Add(-61 * 24 * time.Hour)
	m := store.ProviderMetrics{
		Provider:       "old",
		SuccessCount:   100,
		FailureCount:   0,
		TotalLatencyMs: 50_000,
		RequestCount:   100,
		LastSuccess:    &lastSuccess,
	}

	score := ComputeScore(m)

	// Without decay the score would be near 1.0; with decay it should be pulled toward 0.7.
	// At 61 days (factor ≈ 1.0), the composite should be ~ baseline 0.7.
	if score.CompositeScore > 0.80 {
		t.Errorf("expected decay to reduce score toward 0.7, got %f", score.CompositeScore)
	}
}

func TestHealthStatus(t *testing.T) {
	cases := []struct {
		score  float64
		expect string
	}{
		{0.95, "healthy"},
		{0.81, "healthy"},
		{0.80, "degraded"}, // boundary — >0.80 is healthy, <=0.80 is degraded
		{0.70, "degraded"},
		{0.60, "degraded"},
		{0.59, "unreliable"},
		{0.40, "unreliable"},
		{0.39, "quarantine"},
		{0.00, "quarantine"},
	}

	for _, tc := range cases {
		got := HealthStatus(tc.score)
		if got != tc.expect {
			t.Errorf("HealthStatus(%f) = %q, want %q", tc.score, got, tc.expect)
		}
	}
}
