package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/importer"
	"app-backend/internal/integration/download"
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

func TestSystemStatsEndpoint(t *testing.T) {
	indexerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/indexer/stats" {
			_, _ = w.Write([]byte(`{"stats":{"searches_executed":30,"candidates_evaluated":240,"grabs_performed":0}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer indexerSrv.Close()

	h := NewHandlers(
		metadata.NewClient(metadata.Config{BaseURL: indexerSrv.URL, Timeout: time.Second}),
		indexer.NewClient(indexer.Config{BaseURL: indexerSrv.URL, Timeout: time.Second}),
		store.NewInMemoryWatchlistStore(),
	)

	downloadStore := downloadqueue.NewStore()
	downloadMgr := downloadqueue.NewManager(downloadStore, download.NewService(), nil, "last_resort")
	h.SetDownloadManager(downloadMgr)

	importStore := importer.NewMemoryStore()
	h.SetImportStore(importStore)

	_, _ = downloadStore.CreateJob(downloadqueue.Job{WorkID: "w1", ClientName: "c1", Protocol: "usenet"})
	_, _ = downloadStore.CreateJob(downloadqueue.Job{WorkID: "w2", ClientName: "c1", Protocol: "usenet"})
	_ = downloadStore.UpdateProgress(1, downloadqueue.JobStatusCompleted, "", "")
	_, _ = importStore.CreateOrGetFromDownload(downloadqueue.Job{ID: 11, WorkID: "w1", OutputPath: "a"}, "library")
	_, _ = importStore.CreateOrGetFromDownload(downloadqueue.Job{ID: 12, WorkID: "w2", OutputPath: "b"}, "library")
	_ = importStore.MarkImported(1, "library/a", map[string]any{}, map[string]any{})
	_ = importStore.MarkNeedsReview(2, "collision", map[string]any{}, map[string]any{})

	router := NewRouter(h)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/stats", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stats, _ := payload["stats"].(map[string]any)
	if stats["searches_executed"] != float64(30) {
		t.Fatalf("expected searches_executed 30, got %v", stats["searches_executed"])
	}
	if stats["candidates_evaluated"] != float64(240) {
		t.Fatalf("expected candidates_evaluated 240, got %v", stats["candidates_evaluated"])
	}
	if stats["grabs_performed"] != float64(0) {
		t.Fatalf("expected grabs_performed 0, got %v", stats["grabs_performed"])
	}
	if stats["downloads_completed"] != float64(1) {
		t.Fatalf("expected downloads_completed 1, got %v", stats["downloads_completed"])
	}
	if stats["imports_completed"] != float64(1) {
		t.Fatalf("expected imports_completed 1, got %v", stats["imports_completed"])
	}
	if stats["imports_needs_review"] != float64(1) {
		t.Fatalf("expected imports_needs_review 1, got %v", stats["imports_needs_review"])
	}
}
