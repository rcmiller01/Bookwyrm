package recommend

import "testing"

func TestDistancePenalty_Capped(t *testing.T) {
	if got := distancePenalty(0); got != 0 {
		t.Fatalf("expected zero penalty for delta=0, got %v", got)
	}
	if got := distancePenalty(2); got != 0.1 {
		t.Fatalf("expected penalty=0.1 for delta=2, got %v", got)
	}
	if got := distancePenalty(100); got != 0.25 {
		t.Fatalf("expected penalty capped at 0.25, got %v", got)
	}
}

func TestSubjectContribution_Bounded(t *testing.T) {
	if got := subjectContribution(0.55, 0.5); got != 0.275 {
		t.Fatalf("expected 0.275 contribution, got %v", got)
	}
	if got := subjectContribution(0.55, 2); got != 0.55 {
		t.Fatalf("expected overlap ratio clamp to 1.0, got %v", got)
	}
	if got := subjectContribution(0.55, -1); got != 0 {
		t.Fatalf("expected overlap ratio clamp to 0.0, got %v", got)
	}
}

func TestNormalizeScore_Clamps(t *testing.T) {
	if got := normalizeScore(-0.4); got != 0 {
		t.Fatalf("expected negative score clamp to 0, got %v", got)
	}
	if got := normalizeScore(1.4); got != 1 {
		t.Fatalf("expected >1 score clamp to 1, got %v", got)
	}
}
