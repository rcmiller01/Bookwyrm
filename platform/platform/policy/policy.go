package policy

import (
	"math"
	"strings"
	"time"
)

type Tier int

const (
	TierPrimary Tier = iota
	TierSecondary
	TierFallback
	TierQuarantine
	TierUnclassified
)

const (
	QuarantineModeLastResort = "last_resort"
	QuarantineModeDisabled   = "disabled"
)

const (
	ScoreHealthy    = 0.80
	ScoreDegraded   = 0.60
	ScoreUnreliable = 0.40

	DecayBaseline = 0.7
	DecayDays     = 30

	LatencyThresholdMs = 2000.0
)

type DispatchKey struct {
	Tier     Tier
	Score    float64
	Priority int
}

func DispatchSortKey(tier Tier, score float64, priority int) DispatchKey {
	return DispatchKey{Tier: tier, Score: score, Priority: priority}
}

func (k DispatchKey) Less(other DispatchKey) bool {
	if k.Tier != other.Tier {
		return k.Tier < other.Tier
	}
	if k.Score != other.Score {
		return k.Score > other.Score
	}
	return k.Priority < other.Priority
}

func TierForScore(score float64) Tier {
	switch {
	case score > ScoreHealthy:
		return TierPrimary
	case score >= ScoreDegraded:
		return TierSecondary
	case score >= ScoreUnreliable:
		return TierFallback
	default:
		return TierQuarantine
	}
}

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

func ComputeAvailability(successCount, requestCount int64) float64 {
	if requestCount <= 0 {
		return DecayBaseline
	}
	return Clamp01(float64(successCount) / float64(requestCount))
}

func ComputeLatencyScore(totalLatencyMs, requestCount int64) float64 {
	if requestCount <= 0 || totalLatencyMs <= 0 {
		return DecayBaseline
	}
	avgLatency := float64(totalLatencyMs) / float64(requestCount)
	return Clamp01(1.0 - (avgLatency / LatencyThresholdMs))
}

func ComputeAgreementScore(identifierMatches, identifierIntroduced int64) float64 {
	if identifierIntroduced <= 0 {
		return DecayBaseline
	}
	return Clamp01(float64(identifierMatches) / float64(identifierIntroduced))
}

func DecayTowardBaseline(score float64, lastSuccess *time.Time) float64 {
	if lastSuccess == nil {
		return score
	}
	inactive := time.Since(*lastSuccess)
	threshold := time.Duration(DecayDays) * 24 * time.Hour
	if inactive <= threshold {
		return score
	}
	factor := math.Min(1.0, inactive.Hours()/(float64(DecayDays)*24)-1.0)
	return score + factor*(DecayBaseline-score)
}

func Clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func NormalizeQuarantineMode(mode string, disableDispatch bool) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == QuarantineModeLastResort || m == QuarantineModeDisabled {
		return m
	}
	if disableDispatch {
		return QuarantineModeDisabled
	}
	return QuarantineModeLastResort
}
