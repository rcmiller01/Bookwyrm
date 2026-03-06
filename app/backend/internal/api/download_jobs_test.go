package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/integration/download"
	"app-backend/internal/store"
)

func TestDownloadJobsImportedFilter(t *testing.T) {
	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	dStore := downloadqueue.NewStore()
	mgr := downloadqueue.NewManager(dStore, download.NewService(), nil, "last_resort")
	h.SetDownloadManager(mgr)
	router := NewRouter(h)

	job1, err := dStore.CreateJob(downloadqueue.Job{
		GrabID:      1,
		CandidateID: 1,
		WorkID:      "w1",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create job1: %v", err)
	}
	_ = dStore.UpdateProgress(job1.ID, downloadqueue.JobStatusCompleted, "/downloads/a", "")

	job2, err := dStore.CreateJob(downloadqueue.Job{
		GrabID:      2,
		CandidateID: 2,
		WorkID:      "w2",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create job2: %v", err)
	}
	_ = dStore.UpdateProgress(job2.ID, downloadqueue.JobStatusCompleted, "/downloads/b", "")
	_ = dStore.MarkImported(job2.ID, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/download/jobs?status=completed&imported=false", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	items, _ := payload["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected one unimported completed job, got %d", len(items))
	}
}
