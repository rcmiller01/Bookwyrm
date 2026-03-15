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

func TestWatchlistCRUD(t *testing.T) {
	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer meta.Close()

	h := NewHandlers(
		metadata.NewClient(metadata.Config{BaseURL: meta.URL, Timeout: time.Second}),
		indexer.NewClient(indexer.Config{BaseURL: meta.URL, Timeout: time.Second}),
		store.NewInMemoryWatchlistStore(),
	)
	router := NewRouter(h)

	createRR := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/watchlists", strings.NewReader(`{"target_type":"work","target_id":"work-123","label":"Dune"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-User-ID", "u1")
	router.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRR.Code)
	}

	var item map[string]any
	if err := json.NewDecoder(createRR.Body).Decode(&item); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	id, _ := item["id"].(string)
	if id == "" {
		t.Fatalf("expected created watchlist id")
	}

	listRR := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/watchlists", nil)
	listReq.Header.Set("X-User-ID", "u1")
	router.ServeHTTP(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRR.Code)
	}

	deleteRR := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/watchlists/"+id, nil)
	deleteReq.Header.Set("X-User-ID", "u1")
	router.ServeHTTP(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteRR.Code)
	}
}
