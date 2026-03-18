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

func TestSystemReadiness_SetupRequiredWhenCoreDepsMissing(t *testing.T) {
	t.Setenv("DATABASE_DSN", "")
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
	if canFunction, _ := body["can_function_now"].(bool); canFunction {
		t.Fatalf("expected can_function_now=false")
	}
}

func TestSystemReadiness_ReadyWhenDependenciesConfigured(t *testing.T) {
	t.Setenv("DATABASE_DSN", "postgres://bookwyrm:test@localhost:5432/bookwyrm?sslmode=disable")
	prevPing := pingDatabaseDSN
	pingDatabaseDSN = func(_ context.Context, _ string) error { return nil }
	defer func() { pingDatabaseDSN = prevPing }()
	prevMigrations := querySchemaMigrations
	querySchemaMigrations = func(_ context.Context, _ string) ([]migrationRecord, error) {
		return []migrationRecord{
			{Version: 1, Name: "000001_download_core.up.sql", AppliedAt: time.Now().UTC()},
			{Version: 2, Name: "000002_download_reliability_and_import_flag.up.sql", AppliedAt: time.Now().UTC()},
			{Version: 3, Name: "000003_import_jobs_core.up.sql", AppliedAt: time.Now().UTC()},
			{Version: 4, Name: "000004_job_leases.up.sql", AppliedAt: time.Now().UTC()},
			{Version: 5, Name: "000005_download_upgrade_action.up.sql", AppliedAt: time.Now().UTC()},
		}, nil
	}
	defer func() { querySchemaMigrations = prevMigrations }()

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
	if canFunction, _ := body["can_function_now"].(bool); !canFunction {
		t.Fatalf("expected can_function_now=true")
	}
}
