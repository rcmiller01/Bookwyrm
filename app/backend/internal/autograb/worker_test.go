package autograb

import (
	"testing"

	"app-backend/internal/integration/indexer"
)

func TestCandidateEligible_RejectsNonBookRecursionMatches(t *testing.T) {
	req := indexer.SearchRequestRecord{}
	req.Query.Title = "Recursion"
	req.Query.AutoGrab = true
	req.Query.Preferences.Formats = []string{"epub", "azw3"}

	candidate := indexer.CandidateRecord{ID: 1}
	candidate.Candidate.Title = "Recursion.Deluxe.MacOSX-NOY"
	candidate.Candidate.Protocol = "usenet"
	candidate.Candidate.Score = 0.75
	candidate.Candidate.GrabPayload = map[string]any{"nzb_url": "http://example.test"}

	if candidateEligible(candidate, req) {
		t.Fatalf("expected non-book recursion candidate to be rejected")
	}
}

func TestCandidateEligible_AcceptsMatchingEbookRelease(t *testing.T) {
	req := indexer.SearchRequestRecord{}
	req.Query.Title = "Project Hail Mary"
	req.Query.AutoGrab = true
	req.Query.Preferences.Formats = []string{"epub", "azw3"}

	candidate := indexer.CandidateRecord{ID: 2}
	candidate.Candidate.Title = `REQ: Project Hail Mary epub by Weir, Andy - "Project Hail Mary - Andy Weir;.azw3"`
	candidate.Candidate.Protocol = "usenet"
	candidate.Candidate.Score = 0.91
	candidate.Candidate.GrabPayload = map[string]any{"nzb_url": "http://example.test"}

	if !candidateEligible(candidate, req) {
		t.Fatalf("expected ebook release candidate to be accepted")
	}
}

func TestCandidateEligible_RejectsCourseAndEpisodeNoise(t *testing.T) {
	req := indexer.SearchRequestRecord{}
	req.Query.Title = "Recursion"
	req.Query.AutoGrab = true
	req.Query.Preferences.Formats = []string{"epub"}

	cases := []string{
		"Lynda.com.Code.Clinic.Python.Problem.5.Recursion.and.Directories-ELOHiM",
		"[Prof] Episode 12 - Mother Goose of Mutual Recursion - Recursive Mother Goose",
	}
	for _, title := range cases {
		candidate := indexer.CandidateRecord{ID: 3}
		candidate.Candidate.Title = title
		candidate.Candidate.Protocol = "usenet"
		candidate.Candidate.Score = 0.80
		candidate.Candidate.GrabPayload = map[string]any{"nzb_url": "http://example.test"}
		if candidateEligible(candidate, req) {
			t.Fatalf("expected %q to be rejected", title)
		}
	}
}
