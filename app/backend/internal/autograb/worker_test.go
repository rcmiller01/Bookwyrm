package autograb

import (
	"context"
	"testing"

	"app-backend/internal/integration/indexer"
)

type stubIndexerClient struct {
	candidates []indexer.CandidateRecord
}

func (s *stubIndexerClient) ListCandidates(_ context.Context, _ int64, _ int) ([]indexer.CandidateRecord, error) {
	return s.candidates, nil
}

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
	req.Query.Author = "Andy Weir"
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

func TestCandidateEligible_RejectsAudiobookForEbookRequest(t *testing.T) {
	req := indexer.SearchRequestRecord{}
	req.Query.Title = "Project Hail Mary"
	req.Query.Author = "Andy Weir"
	req.Query.AutoGrab = true
	req.Query.Preferences.Formats = []string{"epub", "azw3"}

	candidate := indexer.CandidateRecord{ID: 4}
	candidate.Candidate.Title = "Andy Weir - Project Hail Mary Audiobook m4b"
	candidate.Candidate.Protocol = "usenet"
	candidate.Candidate.Score = 0.95
	candidate.Candidate.GrabPayload = map[string]any{"nzb_url": "http://example.test"}

	if candidateEligible(candidate, req) {
		t.Fatalf("expected audiobook release to be rejected for ebook-only request")
	}
}

func TestCandidateEligible_AcceptsAudiobookForAudioRequest(t *testing.T) {
	req := indexer.SearchRequestRecord{}
	req.Query.Title = "Project Hail Mary"
	req.Query.Author = "Andy Weir"
	req.Query.AutoGrab = true
	req.Query.Preferences.Formats = []string{"m4b", "mp3"}

	candidate := indexer.CandidateRecord{ID: 5}
	candidate.Candidate.Title = "Andy Weir - Project Hail Mary Audiobook m4b"
	candidate.Candidate.Protocol = "usenet"
	candidate.Candidate.Score = 0.95
	candidate.Candidate.GrabPayload = map[string]any{"nzb_url": "http://example.test"}

	if !candidateEligible(candidate, req) {
		t.Fatalf("expected audiobook release to be accepted for audio request")
	}
}

func TestPickCandidate_PrefersAuthorAndFormatAlignedCandidate(t *testing.T) {
	req := indexer.SearchRequestRecord{ID: 11}
	req.Query.Title = "Project Hail Mary"
	req.Query.Author = "Andy Weir"
	req.Query.Preferences.Formats = []string{"epub", "azw3"}

	worker := &Worker{
		indexerClient: &indexer.Client{},
	}
	client := &stubIndexerClient{
		candidates: []indexer.CandidateRecord{
			{
				ID: 10,
				Candidate: struct {
					Title       string         `json:"title"`
					Protocol    string         `json:"protocol"`
					Score       float64        `json:"score,omitempty"`
					GrabPayload map[string]any `json:"grab_payload"`
				}{
					Title:       "Project Hail Mary Audiobook m4b",
					Protocol:    "usenet",
					Score:       0.98,
					GrabPayload: map[string]any{"nzb_url": "http://example.test/a"},
				},
			},
			{
				ID: 11,
				Candidate: struct {
					Title       string         `json:"title"`
					Protocol    string         `json:"protocol"`
					Score       float64        `json:"score,omitempty"`
					GrabPayload map[string]any `json:"grab_payload"`
				}{
					Title:       "Andy Weir - Project Hail Mary retail epub",
					Protocol:    "usenet",
					Score:       0.88,
					GrabPayload: map[string]any{"nzb_url": "http://example.test/b"},
				},
			},
		},
	}
	worker.indexerClient = (*indexer.Client)(nil)
	got, ok := pickCandidateWithClient(worker, client, req)
	if !ok {
		t.Fatalf("expected a candidate to be selected")
	}
	if got.ID != 11 {
		t.Fatalf("expected ebook/author-aligned candidate to win, got %d", got.ID)
	}
}

func pickCandidateWithClient(_ *Worker, client *stubIndexerClient, req indexer.SearchRequestRecord) (indexer.CandidateRecord, bool) {
	candidates, err := client.ListCandidates(context.Background(), req.ID, 10)
	if err != nil {
		return indexer.CandidateRecord{}, false
	}
	type scoredCandidate struct {
		record indexer.CandidateRecord
		score  float64
	}
	scored := make([]scoredCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidateEligible(candidate, req) {
			continue
		}
		scored = append(scored, scoredCandidate{record: candidate, score: candidatePreferenceScore(candidate, req)})
	}
	if len(scored) == 0 {
		return indexer.CandidateRecord{}, false
	}
	best := scored[0]
	for _, candidate := range scored[1:] {
		if candidate.score > best.score {
			best = candidate
		}
	}
	return best.record, true
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
