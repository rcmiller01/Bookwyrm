package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/importer"
	"app-backend/internal/store"
)

func TestImportNeedsReviewEndpoints(t *testing.T) {
	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	importStore := importer.NewMemoryStore()
	h.SetImportStore(importStore)
	h.SetImportConfig(ImportConfig{KeepIncoming: true, Source: "env"})
	router := NewRouter(h)

	dStore := downloadqueue.NewStore()
	dj, err := dStore.CreateJob(downloadqueue.Job{
		GrabID:      1,
		CandidateID: 1,
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
		OutputPath:  "C:\\downloads\\job1",
	})
	if err != nil {
		t.Fatalf("create download job: %v", err)
	}
	job, err := importStore.CreateOrGetFromDownload(dj, "C:\\library")
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}
	if err := importStore.MarkNeedsReview(job.ID, "ambiguous", map[string]any{}, map[string]any{
		"candidates": []map[string]any{{"work_id": "w-1", "score": 0.4}},
	}); err != nil {
		t.Fatalf("mark needs_review: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/import/jobs?status=needs_review", nil)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list status: got %d", listRes.Code)
	}
	var listBody map[string]any
	if err := json.NewDecoder(listRes.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list body: %v", err)
	}
	items, _ := listBody["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected one needs_review item, got %d", len(items))
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/import/jobs/"+itoa(job.ID), nil)
	getRes := httptest.NewRecorder()
	router.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("get status: got %d", getRes.Code)
	}
	var getBody map[string]any
	if err := json.NewDecoder(getRes.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get body: %v", err)
	}
	if _, ok := getBody["job"].(map[string]any); !ok {
		t.Fatalf("expected job envelope")
	}

	approveReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/jobs/"+itoa(job.ID)+"/approve", strings.NewReader(`{"work_id":"work-approved","edition_id":"ed-1","template_override":"{Author}/{Title}/{Title}.{Ext}"}`))
	approveReq.Header.Set("Content-Type", "application/json")
	approveRes := httptest.NewRecorder()
	router.ServeHTTP(approveRes, approveReq)
	if approveRes.Code != http.StatusNoContent {
		t.Fatalf("approve status: got %d", approveRes.Code)
	}
	approved, err := importStore.GetJob(job.ID)
	if err != nil {
		t.Fatalf("load approved job: %v", err)
	}
	if approved.WorkID != "work-approved" || approved.Status != importer.JobStatusQueued {
		t.Fatalf("unexpected approve state: %+v", approved)
	}

	retryReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/jobs/"+itoa(job.ID)+"/retry", nil)
	retryRes := httptest.NewRecorder()
	router.ServeHTTP(retryRes, retryReq)
	if retryRes.Code != http.StatusNoContent {
		t.Fatalf("retry status: got %d", retryRes.Code)
	}

	skipReq := httptest.NewRequest(http.MethodPost, "/api/v1/import/jobs/"+itoa(job.ID)+"/skip", strings.NewReader(`{"reason":"operator skip"}`))
	skipReq.Header.Set("Content-Type", "application/json")
	skipRes := httptest.NewRecorder()
	router.ServeHTTP(skipRes, skipReq)
	if skipRes.Code != http.StatusNoContent {
		t.Fatalf("skip status: got %d", skipRes.Code)
	}
	skipped, err := importStore.GetJob(job.ID)
	if err != nil {
		t.Fatalf("load skipped job: %v", err)
	}
	if skipped.Status != importer.JobStatusSkipped {
		t.Fatalf("expected skipped status, got %s", skipped.Status)
	}

	statsReq := httptest.NewRequest(http.MethodGet, "/api/v1/import/stats", nil)
	statsRes := httptest.NewRecorder()
	router.ServeHTTP(statsRes, statsReq)
	if statsRes.Code != http.StatusOK {
		t.Fatalf("stats status: got %d", statsRes.Code)
	}
	var statsBody map[string]any
	if err := json.NewDecoder(statsRes.Body).Decode(&statsBody); err != nil {
		t.Fatalf("decode stats body: %v", err)
	}
	if _, ok := statsBody["next_runnable_at"]; !ok {
		t.Fatalf("expected next_runnable_at in stats response")
	}
	keepIncoming, ok := statsBody["keep_incoming"].(bool)
	if !ok {
		t.Fatalf("expected keep_incoming bool")
	}
	if !keepIncoming {
		t.Fatalf("expected keep_incoming true")
	}
	if statsBody["keep_incoming_source"] != "env" {
		t.Fatalf("expected keep_incoming_source=env, got %v", statsBody["keep_incoming_source"])
	}
	counts, ok := statsBody["counts_by_status"].(map[string]any)
	if !ok {
		t.Fatalf("expected counts_by_status object")
	}
	for _, key := range []string{"queued", "running", "needs_review", "imported", "failed", "skipped"} {
		if _, exists := counts[key]; !exists {
			t.Fatalf("missing status count key %q", key)
		}
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRes := httptest.NewRecorder()
	router.ServeHTTP(metricsRes, metricsReq)
	if metricsRes.Code != http.StatusOK {
		t.Fatalf("metrics status: got %d", metricsRes.Code)
	}
	bodyBytes, _ := io.ReadAll(metricsRes.Body)
	bodyText := string(bodyBytes)
	if !strings.Contains(bodyText, "import_jobs_created_total") || !strings.Contains(bodyText, "import_job_duration_seconds") {
		t.Fatalf("expected import metrics in /metrics output")
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
