package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"indexer-service/internal/indexer"
)

func testService() *indexer.Service {
	svc := indexer.NewService()
	svc.Register("prowlarr", indexer.NewMockAdapter("prowlarr-primary", "prowlarr", []string{"availability"}, true, 0))
	svc.Register("non_prowlarr", indexer.NewMockAdapter("archive-primary", "non_prowlarr", []string{"availability"}, true, 0))
	return svc
}

func TestSearchConcurrentGroups(t *testing.T) {
	h := NewHandlers(testService())
	r := NewRouter(h)

	payload := map[string]any{
		"metadata": map[string]any{
			"work_id": "work-1",
			"title":   "Dune",
		},
		"backend_groups": []string{"prowlarr", "non_prowlarr"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/indexer/search", bytes.NewReader(body))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var parsed map[string]any
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	candidates, ok := parsed["candidates"].([]any)
	if !ok || len(candidates) < 2 {
		t.Fatalf("expected merged candidates from both groups, got %v", parsed["candidates"])
	}
}

func TestProvidersEndpoint(t *testing.T) {
	h := NewHandlers(testService())
	r := NewRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/indexer/providers", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestSearchNoAdapters(t *testing.T) {
	h := NewHandlers(indexer.NewService())
	r := NewRouter(h)

	payload := map[string]any{
		"metadata": map[string]any{"work_id": "w1", "title": "X"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/indexer/search", bytes.NewReader(body))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}
