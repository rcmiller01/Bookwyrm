package recommend

import (
	"context"
	"sort"
	"testing"
	"time"

	"metadata-service/internal/cache"
	"metadata-service/internal/model"
	"metadata-service/internal/store"
)

type fakeRecommendStore struct {
	works          map[string]model.Work
	seriesPrev     map[string]*store.SeriesNeighbor
	seriesNext     map[string]*store.SeriesNeighbor
	seriesCands    map[string][]store.SeriesCandidate
	authorCands    map[string][]store.AuthorCandidate
	subjectCands   map[string][]store.SubjectCandidate
	relCands       map[string][]store.RelationshipCandidate
	editionsByWork map[string][]model.Edition
}

func (f *fakeRecommendStore) GetWorkByID(_ context.Context, id string) (*model.Work, error) {
	work, ok := f.works[id]
	if !ok {
		return nil, nil
	}
	return &work, nil
}

func (f *fakeRecommendStore) GetWorksByIDs(_ context.Context, ids []string) ([]model.Work, error) {
	out := make([]model.Work, 0, len(ids))
	for _, id := range ids {
		if work, ok := f.works[id]; ok {
			out = append(out, work)
		}
	}
	return out, nil
}

func (f *fakeRecommendStore) GetSeriesNeighborsForWork(_ context.Context, workID string) (*store.SeriesNeighbor, *store.SeriesNeighbor, error) {
	return f.seriesPrev[workID], f.seriesNext[workID], nil
}

func (f *fakeRecommendStore) GetSeriesCandidatesForWork(_ context.Context, workID string, limit int) ([]store.SeriesCandidate, error) {
	cands := f.seriesCands[workID]
	if len(cands) > limit {
		return cands[:limit], nil
	}
	return cands, nil
}

func (f *fakeRecommendStore) GetAuthorCandidatesForWork(_ context.Context, workID string, limit int) ([]store.AuthorCandidate, error) {
	cands := f.authorCands[workID]
	if len(cands) > limit {
		return cands[:limit], nil
	}
	return cands, nil
}

func (f *fakeRecommendStore) GetSubjectCandidatesForWork(_ context.Context, workID string, _, _ int) ([]store.SubjectCandidate, error) {
	return f.subjectCands[workID], nil
}

func (f *fakeRecommendStore) GetRelationshipCandidatesForWork(_ context.Context, workID string, limit int) ([]store.RelationshipCandidate, error) {
	cands := f.relCands[workID]
	if len(cands) > limit {
		return cands[:limit], nil
	}
	return cands, nil
}

func (f *fakeRecommendStore) GetEditionsByWorkIDs(_ context.Context, workIDs []string) (map[string][]model.Edition, error) {
	out := map[string][]model.Edition{}
	for _, id := range workIDs {
		if editions, ok := f.editionsByWork[id]; ok {
			out[id] = editions
		}
	}
	return out, nil
}

func baseFixtureStore() *fakeRecommendStore {
	works := map[string]model.Work{}
	for _, id := range []string{"w1", "w2", "w3", "w4", "w5", "w6"} {
		works[id] = model.Work{ID: id, Title: "Work " + id}
	}
	return &fakeRecommendStore{
		works: works,
		seriesPrev: map[string]*store.SeriesNeighbor{
			"w3": {WorkID: "w2", SeriesName: "Dune", Delta: 1},
		},
		seriesNext: map[string]*store.SeriesNeighbor{
			"w3": {WorkID: "w4", SeriesName: "Dune", Delta: 1},
		},
		seriesCands: map[string][]store.SeriesCandidate{
			"w3": {
				{WorkID: "w2", SeriesName: "Dune", Delta: 1},
				{WorkID: "w4", SeriesName: "Dune", Delta: 1},
				{WorkID: "w5", SeriesName: "Dune", Delta: 2},
			},
		},
		authorCands: map[string][]store.AuthorCandidate{
			"w3": {
				{WorkID: "w5", SharedAuthors: []string{"Frank Herbert"}},
				{WorkID: "w6", SharedAuthors: []string{"Frank Herbert"}},
			},
		},
		subjectCands: map[string][]store.SubjectCandidate{
			"w3": {
				{WorkID: "w6", SharedSubjects: []string{"science fiction", "space opera"}, OverlapRatio: 0.5},
				{WorkID: "w5", SharedSubjects: []string{"science fiction"}, OverlapRatio: 0.25},
			},
		},
		relCands: map[string][]store.RelationshipCandidate{
			"w3": {
				{WorkID: "w2", RelationshipType: "same_series", Confidence: 0.9},
			},
		},
		editionsByWork: map[string][]model.Edition{
			"w4": {{ID: "ed4", WorkID: "w4", Format: "epub"}},
			"w6": {{ID: "ed6", WorkID: "w6", Format: "audiobook"}},
		},
	}
}

func TestEngine_GoldenRankingDeterministic(t *testing.T) {
	engine := NewEngine(baseFixtureStore(), nil, DefaultOptions())

	req := RecommendationRequest{SeedWorkIDs: []string{"w3"}, Limit: 5}
	results, err := engine.Recommend(context.Background(), req)
	if err != nil {
		t.Fatalf("recommend failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected recommendations")
	}

	gotTop := results[0].Work.ID
	if gotTop != "w2" && gotTop != "w4" {
		t.Fatalf("expected top recommendation to be immediate series neighbor (w2 or w4), got %q", gotTop)
	}

	for _, result := range results {
		if len(result.Reasons) == 0 {
			t.Fatalf("expected reasons for all recommendations, missing for %s", result.Work.ID)
		}
	}

	results2, err := engine.Recommend(context.Background(), req)
	if err != nil {
		t.Fatalf("second recommend failed: %v", err)
	}
	if len(results2) != len(results) {
		t.Fatalf("expected deterministic result length, got %d and %d", len(results), len(results2))
	}
	for i := range results {
		if results[i].Work.ID != results2[i].Work.ID || results[i].Score != results2[i].Score {
			t.Fatalf("expected deterministic ordering/scores at index %d", i)
		}
	}
}

func TestEngine_DedupeAndCapEnforcement(t *testing.T) {
	fixture := baseFixtureStore()
	many := make([]store.SubjectCandidate, 0, 100)
	for i := 0; i < 100; i++ {
		id := "w" + string(rune('a'+(i%6)))
		many = append(many, store.SubjectCandidate{WorkID: id, SharedSubjects: []string{"sci-fi"}, OverlapRatio: 0.1})
	}
	fixture.subjectCands["w3"] = many

	opts := DefaultOptions()
	opts.MaxCandidatePool = 3
	engine := NewEngine(fixture, nil, opts)

	results, err := engine.Recommend(context.Background(), RecommendationRequest{SeedWorkIDs: []string{"w3"}, Limit: 10})
	if err != nil {
		t.Fatalf("recommend failed: %v", err)
	}
	if len(results) > 3 {
		t.Fatalf("expected capped results <=3 due to candidate pool cap, got %d", len(results))
	}
}

func TestEngine_PreferenceBoostApplied(t *testing.T) {
	cacheStore, _ := cache.NewRistrettoCache()
	opts := DefaultOptions()
	opts.CacheTTL = time.Hour
	engine := NewEngine(baseFixtureStore(), cacheStore, opts)

	results, err := engine.Recommend(context.Background(), RecommendationRequest{
		SeedWorkIDs: []string{"w3"},
		Limit:       5,
		Preferences: RecommendationPreferences{Formats: []string{"epub"}},
	})
	if err != nil {
		t.Fatalf("recommend failed: %v", err)
	}

	hasPreferenceReason := false
	for _, result := range results {
		if result.Work.ID == "w4" {
			for _, reason := range result.Reasons {
				if reason.Type == "preference_match" {
					hasPreferenceReason = true
				}
			}
		}
	}
	if !hasPreferenceReason {
		t.Fatalf("expected preference_match reason for work w4")
	}
}

func TestIncludeSet_DefaultsAndFiltering(t *testing.T) {
	all := includeSet(nil)
	if !(all["series"] && all["author"] && all["subjects"] && all["relationships"]) {
		t.Fatalf("expected default include set to enable all traversals")
	}

	filtered := includeSet([]string{"subjects", "author"})
	if filtered["series"] || filtered["relationships"] {
		t.Fatalf("expected series/relationships disabled in filtered include set")
	}
	if !(filtered["subjects"] && filtered["author"]) {
		t.Fatalf("expected requested include types enabled")
	}
}

func TestExcludeSet_ContainsSeedsAndExcludes(t *testing.T) {
	set := excludeSet([]string{"w9"}, []string{"w1", "w2"})
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) != 3 {
		t.Fatalf("expected 3 exclude entries, got %d", len(keys))
	}
}
