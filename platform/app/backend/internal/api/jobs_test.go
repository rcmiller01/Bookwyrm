package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-backend/internal/jobs"
	"app-backend/internal/store"
)

func TestJobsEndpoints_EnqueueListGetRetryCancel(t *testing.T) {
	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	jobStore := store.NewInMemoryJobStore()
	jobService := jobs.NewService(
		jobStore,
		jobs.Options{WorkerCount: 1, PollInterval: 20 * time.Millisecond},
		jobs.NewNoopHandler("search_missing"),
	)
	h.SetJobService(jobService)
	router := NewRouter(h)

	payload := map[string]any{
		"type":    "search_missing",
		"payload": map[string]any{"metadata": map[string]any{"work_id": "w1", "title": "Dune"}},
	}
	body, _ := json.Marshal(payload)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewReader(body))
	router.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", res.Code)
	}

	var created map[string]any
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode created job: %v", err)
	}
	jobID, _ := created["id"].(string)
	if jobID == "" {
		t.Fatalf("missing job id")
	}

	listRes := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	router.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200 list, got %d", listRes.Code)
	}

	getRes := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	router.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("expected 200 get, got %d", getRes.Code)
	}

	_, _ = jobStore.MarkRetryableFailure(jobID, "boom", time.Now().UTC())

	retryRes := httptest.NewRecorder()
	retryReq := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/"+jobID+"/retry", nil)
	router.ServeHTTP(retryRes, retryReq)
	if retryRes.Code != http.StatusOK {
		t.Fatalf("expected 200 retry, got %d", retryRes.Code)
	}

	cancelRes := httptest.NewRecorder()
	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/"+jobID+"/cancel", nil)
	router.ServeHTTP(cancelRes, cancelReq)
	if cancelRes.Code != http.StatusOK {
		t.Fatalf("expected 200 cancel, got %d", cancelRes.Code)
	}
}
