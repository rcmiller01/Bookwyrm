package indexer

import "testing"

func ptr64(v int64) *int64 { return &v }
func ptrInt(v int) *int    { return &v }

func TestReleaseFingerprint_SameISBNDifferentTitle(t *testing.T) {
	a := Candidate{
		Title:       "Dune - Frank Herbert",
		Protocol:    "usenet",
		SizeBytes:   ptr64(5 * 1024 * 1024),
		Identifiers: map[string]any{"isbn": "9780441172719"},
	}
	b := Candidate{
		Title:       "Dune (Dune Chronicles #1)",
		Protocol:    "usenet",
		SizeBytes:   ptr64(5 * 1024 * 1024),
		Identifiers: map[string]any{"isbn": "9780441172719"},
	}
	if ReleaseFingerprint(a) != ReleaseFingerprint(b) {
		t.Error("candidates with same ISBN should have same fingerprint regardless of title")
	}
}

func TestReleaseFingerprint_DifferentProtocol(t *testing.T) {
	a := Candidate{Title: "Dune", Protocol: "usenet", SizeBytes: ptr64(5 * 1024 * 1024)}
	b := Candidate{Title: "Dune", Protocol: "torrent", SizeBytes: ptr64(5 * 1024 * 1024)}
	if ReleaseFingerprint(a) == ReleaseFingerprint(b) {
		t.Error("candidates with different protocols should have different fingerprints")
	}
}

func TestReleaseFingerprint_SizeBucketTolerance(t *testing.T) {
	// 5MB and 5.5MB should land in the same MB bucket (5)
	a := Candidate{Title: "Dune", Protocol: "usenet", SizeBytes: ptr64(5 * 1024 * 1024)}
	b := Candidate{Title: "Dune", Protocol: "usenet", SizeBytes: ptr64(5*1024*1024 + 500*1024)}
	if ReleaseFingerprint(a) != ReleaseFingerprint(b) {
		t.Error("candidates within the same MB bucket should have the same fingerprint")
	}
}

func TestReleaseFingerprint_SizeBucketDifferent(t *testing.T) {
	// 5MB and 7MB should be different buckets
	a := Candidate{Title: "Dune", Protocol: "usenet", SizeBytes: ptr64(5 * 1024 * 1024)}
	b := Candidate{Title: "Dune", Protocol: "usenet", SizeBytes: ptr64(7 * 1024 * 1024)}
	if ReleaseFingerprint(a) == ReleaseFingerprint(b) {
		t.Error("candidates in different MB buckets should have different fingerprints")
	}
}

func TestReleaseFingerprint_NoIdentifiersFallsBackToTitle(t *testing.T) {
	a := Candidate{Title: "Dune", Protocol: "usenet", SizeBytes: ptr64(5 * 1024 * 1024)}
	b := Candidate{Title: "Foundation", Protocol: "usenet", SizeBytes: ptr64(5 * 1024 * 1024)}
	if ReleaseFingerprint(a) == ReleaseFingerprint(b) {
		t.Error("candidates with no identifiers and different titles should differ")
	}
}

func TestReleaseFingerprint_EmptyIdentifiersIgnored(t *testing.T) {
	a := Candidate{
		Title:       "Dune",
		Protocol:    "usenet",
		SizeBytes:   ptr64(5 * 1024 * 1024),
		Identifiers: map[string]any{"isbn": ""},
	}
	b := Candidate{
		Title:    "Dune",
		Protocol: "usenet",
		SizeBytes: ptr64(5 * 1024 * 1024),
	}
	if ReleaseFingerprint(a) != ReleaseFingerprint(b) {
		t.Error("empty identifiers should be ignored; fingerprints should match")
	}
}

func TestDedupeCandidates_SameISBNDifferentTitle(t *testing.T) {
	candidates := []Candidate{
		{Title: "Dune - Frank Herbert", Protocol: "usenet", SizeBytes: ptr64(5 * 1024 * 1024), Identifiers: map[string]any{"isbn": "9780441172719"}},
		{Title: "Dune (Dune Chronicles #1)", Protocol: "usenet", SizeBytes: ptr64(5 * 1024 * 1024), Identifiers: map[string]any{"isbn": "9780441172719"}},
	}
	result := DedupeCandidates(candidates)
	if len(result) != 1 {
		t.Errorf("expected 1 candidate after dedup, got %d", len(result))
	}
}

func TestDedupeCandidates_DifferentProtocolKeptSeparate(t *testing.T) {
	candidates := []Candidate{
		{Title: "Dune", Protocol: "usenet", SizeBytes: ptr64(5 * 1024 * 1024)},
		{Title: "Dune", Protocol: "torrent", SizeBytes: ptr64(5 * 1024 * 1024)},
	}
	result := DedupeCandidates(candidates)
	if len(result) != 2 {
		t.Errorf("expected 2 candidates (different protocol), got %d", len(result))
	}
}

func TestDedupeCandidates_SizeBucketMerge(t *testing.T) {
	candidates := []Candidate{
		{Title: "Dune", Protocol: "usenet", SizeBytes: ptr64(5 * 1024 * 1024)},
		{Title: "Dune", Protocol: "usenet", SizeBytes: ptr64(5*1024*1024 + 100*1024)}, // same MB bucket
	}
	result := DedupeCandidates(candidates)
	if len(result) != 1 {
		t.Errorf("expected 1 candidate (same MB bucket), got %d", len(result))
	}
}

func TestApplyScoring_SortsByScoreDesc(t *testing.T) {
	backends := map[string]BackendRecord{
		"a": {ID: "a", ReliabilityScore: 0.90},
		"b": {ID: "b", ReliabilityScore: 0.50},
	}
	candidates := []Candidate{
		{CandidateID: "1", Title: "Dune", SourceBackendID: "b"},
		{CandidateID: "2", Title: "Dune", SourceBackendID: "a"},
	}
	result := ApplyScoring(candidates, backends, QuerySpec{})
	if len(result) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(result))
	}
	if result[0].Score < result[1].Score {
		t.Error("candidates should be sorted by score descending")
	}
	if result[0].SourceBackendID != "a" {
		t.Error("higher-reliability backend candidate should rank first")
	}
}

func TestApplyScoring_PreferredSourceBoost(t *testing.T) {
	backends := map[string]BackendRecord{
		"preferred": {ID: "preferred", ReliabilityScore: 0.70, Config: map[string]any{"preferred": true}},
		"normal":    {ID: "normal", ReliabilityScore: 0.70},
	}
	candidates := []Candidate{
		{CandidateID: "1", Title: "Dune", SourceBackendID: "normal"},
		{CandidateID: "2", Title: "Dune", SourceBackendID: "preferred"},
	}
	result := ApplyScoring(candidates, backends, QuerySpec{})
	if len(result) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(result))
	}
	if result[0].SourceBackendID != "preferred" {
		t.Error("preferred backend candidate should rank first")
	}
	scoreDiff := result[0].Score - result[1].Score
	if scoreDiff < 0.09 || scoreDiff > 0.11 {
		t.Errorf("preferred boost should be ~0.10, got diff %.4f", scoreDiff)
	}
	// Verify reason code exists on preferred candidate
	found := false
	for _, r := range result[0].Reasons {
		if r.Code == "preferred_source" {
			found = true
			break
		}
	}
	if !found {
		t.Error("preferred candidate should have 'preferred_source' reason code")
	}
}

func TestApplyScoring_PreferredFalseNoBoost(t *testing.T) {
	backends := map[string]BackendRecord{
		"a": {ID: "a", ReliabilityScore: 0.70, Config: map[string]any{"preferred": false}},
		"b": {ID: "b", ReliabilityScore: 0.70},
	}
	candidates := []Candidate{
		{CandidateID: "1", Title: "Dune", SourceBackendID: "a"},
		{CandidateID: "2", Title: "Dune", SourceBackendID: "b"},
	}
	result := ApplyScoring(candidates, backends, QuerySpec{})
	if result[0].Score != result[1].Score {
		t.Error("preferred=false should not boost score")
	}
}
