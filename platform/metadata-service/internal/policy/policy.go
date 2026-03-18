package policy

import (
	platformpolicy "bookwyrm/platform/policy"
	"time"
)

type Tier = platformpolicy.Tier

const (
	TierPrimary      = platformpolicy.TierPrimary
	TierSecondary    = platformpolicy.TierSecondary
	TierFallback     = platformpolicy.TierFallback
	TierQuarantine   = platformpolicy.TierQuarantine
	TierUnclassified = platformpolicy.TierUnclassified
)

const (
	QuarantineModeLastResort = platformpolicy.QuarantineModeLastResort
	QuarantineModeDisabled   = platformpolicy.QuarantineModeDisabled
)

const (
	ScoreHealthy    = platformpolicy.ScoreHealthy
	ScoreDegraded   = platformpolicy.ScoreDegraded
	ScoreUnreliable = platformpolicy.ScoreUnreliable

	DecayBaseline = platformpolicy.DecayBaseline
	DecayDays     = platformpolicy.DecayDays

	LatencyThresholdMs = platformpolicy.LatencyThresholdMs
)

type DispatchKey = platformpolicy.DispatchKey

func DispatchSortKey(tier Tier, score float64, priority int) DispatchKey {
	return platformpolicy.DispatchSortKey(tier, score, priority)
}

func TierForScore(score float64) Tier {
	return platformpolicy.TierForScore(score)
}

func HealthStatus(score float64) string {
	return platformpolicy.HealthStatus(score)
}

func ComputeAvailability(successCount, requestCount int64) float64 {
	return platformpolicy.ComputeAvailability(successCount, requestCount)
}

func ComputeLatencyScore(totalLatencyMs, requestCount int64) float64 {
	return platformpolicy.ComputeLatencyScore(totalLatencyMs, requestCount)
}

func ComputeAgreementScore(identifierMatches, identifierIntroduced int64) float64 {
	return platformpolicy.ComputeAgreementScore(identifierMatches, identifierIntroduced)
}

func DecayTowardBaseline(score float64, lastSuccess *time.Time) float64 {
	return platformpolicy.DecayTowardBaseline(score, lastSuccess)
}

func Clamp01(v float64) float64 {
	return platformpolicy.Clamp01(v)
}

func NormalizeQuarantineMode(mode string, disableDispatch bool) string {
	return platformpolicy.NormalizeQuarantineMode(mode, disableDispatch)
}
