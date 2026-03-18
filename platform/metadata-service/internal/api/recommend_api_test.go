package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"metadata-service/internal/model"
	"metadata-service/internal/recommend"
)

type fakeRecommendService struct {
	results []recommend.RecommendationResult
	next    *recommend.RecommendationResult
}

func (f *fakeRecommendService) Recommend(_ context.Context, _ recommend.RecommendationRequest) ([]recommend.RecommendationResult, error) {
	return f.results, nil
}
func (f *fakeRecommendService) RecommendNextInSeries(_ context.Context, _ string) (*recommend.RecommendationResult, error) {
	return f.next, nil
}
func (f *fakeRecommendService) RecommendSimilar(_ context.Context, _ string, _ int, _ recommend.RecommendationPreferences) ([]recommend.RecommendationResult, error) {
	return f.results, nil
}

func TestGetWorkRecommendations_Contract(t *testing.T) {
	h := &Handlers{recommender: &fakeRecommendService{results: []recommend.RecommendationResult{{
		Work:  model.Work{ID: "w2", Title: "Dune Messiah"},
		Score: 0.95,
		Reasons: []recommend.Reason{{
			Type:   "series_neighbor",
			Weight: 0.95,
			Evidence: map[string]any{
				"series": "Dune",
				"delta":  1,
			},
		}},
	}}}}
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/work/w1/recommendations?limit=20&include=series,subjects", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["seed_work_id"] != "w1" {
		t.Fatalf("expected seed_work_id w1, got %v", body["seed_work_id"])
	}
	recs, ok := body["recommendations"].([]any)
	if !ok || len(recs) != 1 {
		t.Fatalf("expected one recommendation, got %v", body["recommendations"])
	}
	rec := recs[0].(map[string]any)
	if _, hasScore := rec["score"]; !hasScore {
		t.Fatalf("expected score field")
	}
	reasons, hasReasons := rec["reasons"].([]any)
	if !hasReasons || len(reasons) == 0 {
		t.Fatalf("expected reasons array")
	}
}

func TestGetNextInSeries_Contract(t *testing.T) {
	h := &Handlers{recommender: &fakeRecommendService{next: &recommend.RecommendationResult{
		Work:    model.Work{ID: "w2", Title: "Dune Messiah"},
		Score:   0.98,
		Reasons: []recommend.Reason{{Type: "series_neighbor", Weight: 0.98}},
	}}}
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/work/w1/next", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["seed_work_id"] != "w1" {
		t.Fatalf("expected seed_work_id w1, got %v", body["seed_work_id"])
	}
	next, ok := body["next"].(map[string]any)
	if !ok {
		t.Fatalf("expected next object")
	}
	if _, hasReasons := next["reasons"]; !hasReasons {
		t.Fatalf("expected reasons in next response")
	}
}

func TestGetSimilarWorks_Contract(t *testing.T) {
	h := &Handlers{recommender: &fakeRecommendService{results: []recommend.RecommendationResult{{
		Work:    model.Work{ID: "w9", Title: "Hyperion"},
		Score:   0.72,
		Reasons: []recommend.Reason{{Type: "shared_subject", Weight: 0.4}},
	}}}}
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/work/w1/similar?limit=10", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	recs, ok := body["recommendations"].([]any)
	if !ok || len(recs) == 0 {
		t.Fatalf("expected recommendations array")
	}
}
