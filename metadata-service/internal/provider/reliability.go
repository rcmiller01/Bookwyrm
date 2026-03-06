package provider

import (
	"metadata-service/internal/policy"
	"metadata-service/internal/store"
	"time"
)

// Score thresholds — determining health status.
const (
	ScoreHealthy    = policy.ScoreHealthy
	ScoreDegraded   = policy.ScoreDegraded
	ScoreUnreliable = policy.ScoreUnreliable

	// DecayBaseline is the score a provider regresses toward after inactivity.
	DecayBaseline = policy.DecayBaseline
	// DecayDays is the number of days of inactivity before decay begins.
	DecayDays = policy.DecayDays

	// LatencyThresholdMs is the target upper-bound for average latency.
	LatencyThresholdMs = policy.LatencyThresholdMs
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

	availability := policy.ComputeAvailability(m.SuccessCount, m.RequestCount)
	latencyScore := policy.ComputeLatencyScore(m.TotalLatencyMs, m.RequestCount)
	agreementScore := policy.ComputeAgreementScore(m.IdentifierMatches, m.IdentifierIntroduced)

	// identifier quality mirrors agreement when the provider is actively contributing identifiers
	identifierQuality := agreementScore

	// --- composite ---
	composite := availability*0.35 + agreementScore*0.35 + latencyScore*0.15 + identifierQuality*0.15

	// apply inactivity decay
	composite = applyDecay(composite, m.LastSuccess)

	rs.Availability = policy.Clamp01(availability)
	rs.LatencyScore = policy.Clamp01(latencyScore)
	rs.AgreementScore = policy.Clamp01(agreementScore)
	rs.IdentifierQuality = policy.Clamp01(identifierQuality)
	rs.CompositeScore = policy.Clamp01(composite)
	return rs
}

// HealthStatus returns the human-readable status label for a composite score.
//
//	> 0.80  → healthy
//	≥ 0.60  → degraded
//	≥ 0.40  → unreliable
//	< 0.40  → quarantine
func HealthStatus(score float64) string {
	return policy.HealthStatus(score)
}

// TierForScore maps a reliability score to dispatch tier.
//
//	> 0.80  → primary
//	≥ 0.60  → secondary
//	≥ 0.40  → fallback
//	< 0.40  → quarantine
func TierForScore(score float64) DispatchTier {
	return policy.TierForScore(score)
}

// applyDecay moves the composite score toward DecayBaseline when the provider
// has not been seen for more than DecayDays. The interpolation is linear and
// completes after a second full DecayDays period of inactivity.
func applyDecay(score float64, lastSuccess *time.Time) float64 {
	return policy.DecayTowardBaseline(score, lastSuccess)
}
