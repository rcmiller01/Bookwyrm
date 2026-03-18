package recommend

import "math"

type ScoringWeights struct {
	SeriesNeighbor  float64
	SameSeries      float64
	SameAuthor      float64
	SharedSubject   float64
	ExplicitRelated float64
	PreferenceBoost float64
}

func DefaultWeights() ScoringWeights {
	return ScoringWeights{
		SeriesNeighbor:  1.00,
		SameSeries:      0.85,
		SameAuthor:      0.70,
		SharedSubject:   0.55,
		ExplicitRelated: 0.90,
		PreferenceBoost: 0.05,
	}
}

func distancePenalty(delta float64) float64 {
	if delta <= 0 {
		return 0
	}
	penalty := delta * 0.05
	if penalty > 0.25 {
		penalty = 0.25
	}
	return penalty
}

func normalizeScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return math.Round(score*10000) / 10000
}

func subjectContribution(baseWeight float64, overlapRatio float64) float64 {
	if overlapRatio < 0 {
		overlapRatio = 0
	}
	if overlapRatio > 1 {
		overlapRatio = 1
	}
	return baseWeight * overlapRatio
}
