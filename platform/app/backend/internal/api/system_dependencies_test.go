package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/store"
)

func TestSystemDependencies_DegradedWhenCoreDependenciesMissing(t *testing.T) {
	t.Setenv("DATABASE_DSN", "")
	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/dependencies", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode dependencies body: %v", err)
	}
	if body["status"] != "degraded" {
		t.Fatalf("expected degraded status, got %v", body["status"])
	}
	if canFunction, _ := body["can_function_now"].(bool); canFunction {
		t.Fatalf("expected can_function_now=false")
	}
}

func TestSystemDependencies_ReadyWhenDependenciesConfigured(t *testing.T) {
	t.Setenv("DATABASE_DSN", "postgres://bookwyrm:test@localhost:5432/bookwyrm?sslmode=disable")
	prevPing := pingDatabaseDSN
	pingDatabaseDSN = func(_ context.Context, _ string) error { return nil }
	defer func() { pingDatabaseDSN = prevPing }()

	metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer metadata.Close()

	indexer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/indexer/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case "/v1/indexer/backends":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"backends": []map[string]any{
					{"id": "prowlarr:main", "enabled": true},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer indexer.Close()

	dStore := downloadqueue.NewStore()
	dStore.UpsertClient(downloadqueue.DownloadClientRecord{
		ID:         "nzbget",
		Name:       "NZBGet",
		ClientType: "nzbget",
		Enabled:    true,
		Priority:   1,
		Tier:       "primary",
	})
	dMgr := downloadqueue.NewManager(dStore, nil, nil, "last_resort")

	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	h.SetImportConfig(ImportConfig{LibraryRoot: filepath.Clean(t.TempDir())})
	h.SetUpstreamURLs(metadata.URL, indexer.URL)
	h.SetDownloadManager(dMgr)
	h.SetStartupTime(time.Now().UTC())
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/dependencies", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode dependencies body: %v", err)
	}
	if body["status"] != "ready" {
		t.Fatalf("expected ready status, got %v", body["status"])
	}
	if canFunction, _ := body["can_function_now"].(bool); !canFunction {
		t.Fatalf("expected can_function_now=true")
	}
}
