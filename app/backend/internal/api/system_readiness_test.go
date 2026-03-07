package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/store"
)

func TestSystemReadiness_SetupRequiredWhenCoreDepsMissing(t *testing.T) {
	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	h.SetImportConfig(ImportConfig{})
	h.SetStartupTime(time.Now().UTC())
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/readiness", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode readiness body: %v", err)
	}
	if body["status"] != "setup_required" {
		t.Fatalf("expected setup_required, got %v", body["status"])
	}
	if ready, _ := body["ready"].(bool); ready {
		t.Fatalf("expected ready=false")
	}
}

func TestSystemReadiness_ReadyWhenDependenciesConfigured(t *testing.T) {
	metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case "/v1/providers":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"providers": []map[string]any{
					{"name": "openlibrary", "enabled": true},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer metadata.Close()

	indexer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/indexer/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case "/v1/indexer/backends":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"backends": []map[string]any{
					{"id": "prowlarr:main", "backend_type": "prowlarr", "enabled": true},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer indexer.Close()

	libraryRoot := t.TempDir()
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
	h.SetImportConfig(ImportConfig{LibraryRoot: filepath.Clean(libraryRoot)})
	h.SetUpstreamURLs(metadata.URL, indexer.URL)
	h.SetDownloadManager(dMgr)
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/readiness", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode readiness body: %v", err)
	}
	if body["status"] == "setup_required" {
		t.Fatalf("expected not setup_required: %#v", body)
	}
	if ready, _ := body["ready"].(bool); !ready {
		t.Fatalf("expected ready=true")
	}
}
