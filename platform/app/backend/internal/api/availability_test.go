package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app-backend/internal/integration/indexer"
	"app-backend/internal/integration/metadata"
	"app-backend/internal/store"
)

func TestAvailabilityFanout(t *testing.T) {
	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/work/"):
			_, _ = w.Write([]byte(`{"work":{"id":"work-1","title":"Dune","authors":[{"name":"Frank Herbert"}],"editions":[{"id":"ed-1","identifiers":[{"type":"ISBN_13","value":"9780441172719"}]}]}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer meta.Close()

	index := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/indexer/search" {
			t.Fatalf("unexpected indexer path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"work_id":"work-1","source":"multi-indexer","found":true,"candidates":[{"candidate_id":"c1","match_confidence":0.9}]}`))
	}))
	defer index.Close()

	h := NewHandlers(
		metadata.NewClient(metadata.Config{BaseURL: meta.URL, Timeout: time.Second}),
		indexer.NewClient(indexer.Config{BaseURL: index.URL, Timeout: time.Second}),
		store.NewInMemoryWatchlistStore(),
	)
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/works/work-1/availability?groups=prowlarr,non_prowlarr", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var parsed map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := parsed["availability"].(map[string]any); !ok {
		t.Fatalf("expected availability object, got %v", parsed["availability"])
	}
}
