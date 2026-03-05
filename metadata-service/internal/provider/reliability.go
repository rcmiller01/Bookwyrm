package provider

import (
	"math"
	"time"

	"metadata-service/internal/store"
)

// Score thresholds — determining health status.
const (
	ScoreHealthy    = 0.80
	ScoreDegraded   = 0.60
	ScoreUnreliable = 0.40

	// DecayBaseline is the score a provider regresses toward after inactivity.
	DecayBaseline = 0.7
	// DecayDays is the number of days of inactivity before decay begins.
	DecayDays = 30

	// LatencyThresholdMs is the target upper-bound for average latency.
	LatencyThresholdMs = 2000.0
)

// ReliabilityScore is a type alias for the store-layer reliability score struct.
type ReliabilityScore = store.ReliabilityScore

// ComputeScore derives a ReliabilityScore from raw ProviderMetrics using the
// weighted composite formula:
//
//	composite = availability*0.35 + agreement*0.35 + latency_score*0.15 + identifier_quality*0.15
//
// All sub-scores are clamped to [0.0, 1.0]. If a provider has been inactive for
// more than DecayDays the composite score decays linearly toward DecayBaseline.
func ComputeScore(m store.ProviderMetrics) store.ReliabilityScore {
	rs := store.ReliabilityScore{Provider: m.Provider}

	// --- availability = success / total requests ---
	var availability float64
	if m.RequestCount > 0 {
		availability = float64(m.SuccessCount) / float64(m.RequestCount)
	} else {
		availability = DecayBaseline
	}

	// --- latency_score = 1 - (avg_latency_ms / threshold) ---
	var latencyScore float64
	if m.RequestCount > 0 && m.TotalLatencyMs > 0 {
		avgLatency := float64(m.TotalLatencyMs) / float64(m.RequestCount)
		latencyScore = 1.0 - (avgLatency / LatencyThresholdMs)
	} else {
		latencyScore = DecayBaseline
	}

	// --- agreement = identifier_matches / identifier_introduced ---
	var agreementScore float64
	if m.IdentifierIntroduced > 0 {
		agreementScore = float64(m.IdentifierMatches) / float64(m.IdentifierIntroduced)
	} else {
		agreementScore = DecayBaseline
	}

	// identifier quality mirrors agreement when the provider is actively contributing identifiers
	identifierQuality := agreementScore

	// --- composite ---
	composite := availability*0.35 + agreementScore*0.35 + latencyScore*0.15 + identifierQuality*0.15

	// apply inactivity decay
	composite = applyDecay(composite, m.LastSuccess)

	rs.Availability = clamp(availability)
	rs.LatencyScore = clamp(latencyScore)
	rs.AgreementScore = clamp(agreementScore)
	rs.IdentifierQuality = clamp(identifierQuality)
	rs.CompositeScore = clamp(composite)
	return rs
}

// HealthStatus returns the human-readable status label for a composite score.
//
//	> 0.80  → healthy
//	≥ 0.60  → degraded
//	≥ 0.40  → unreliable
//	< 0.40  → quarantine
func HealthStatus(score float64) string {
	switch {
	case score > ScoreHealthy:
		return "healthy"
	case score >= ScoreDegraded:
		return "degraded"
	case score >= ScoreUnreliable:
		return "unreliable"
	default:
		return "quarantine"
	}
}

// TierForScore maps a reliability score to dispatch tier.
//
//	> 0.80  → primary
//	≥ 0.60  → secondary
//	≥ 0.40  → fallback
//	< 0.40  → quarantine
func TierForScore(score float64) DispatchTier {
	switch {
	case score > ScoreHealthy:
		return DispatchTierPrimary
	case score >= ScoreDegraded:
		return DispatchTierSecondary
	case score >= ScoreUnreliable:
		return DispatchTierFallback
	default:
		return DispatchTierQuarantine
	}
}

// applyDecay moves the composite score toward DecayBaseline when the provider
// has not been seen for more than DecayDays. The interpolation is linear and
// completes after a second full DecayDays period of inactivity.
func applyDecay(score float64, lastSuccess *time.Time) float64 {
	if lastSuccess == nil {
		return score
	}
	inactive := time.Since(*lastSuccess)
	threshold := time.Duration(DecayDays) * 24 * time.Hour
	if inactive <= threshold {
		return score
	}
	// factor ∈ [0, 1]: how far past the threshold we are (caps at 1 after 2×DecayDays)
	factor := math.Min(1.0, inactive.Hours()/(float64(DecayDays)*24)-1.0)
	return score + factor*(DecayBaseline-score)
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
