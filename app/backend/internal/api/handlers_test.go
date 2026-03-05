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

func TestSearchProxy(t *testing.T) {
	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/search") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"works":[{"id":"w1","title":"Dune"}]}`))
	}))
	defer meta.Close()

	h := NewHandlers(
		metadata.NewClient(metadata.Config{BaseURL: meta.URL, Timeout: time.Second}),
		indexer.NewClient(indexer.Config{BaseURL: meta.URL, Timeout: time.Second}),
		store.NewInMemoryWatchlistStore(),
	)
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=dune", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	works, ok := body["works"].([]any)
	if !ok || len(works) != 1 {
		t.Fatalf("expected 1 work, got %v", body["works"])
	}
}
