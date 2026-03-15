package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"metadata-service/internal/model"
	"metadata-service/internal/store"
)

type fakeEnrichmentStatsStore struct {
	counts         map[string]int64
	nextRunnableAt *time.Time
	enqueueErr     error
	nextID         int64
	enqueued       []model.EnrichmentJob
}

func (f *fakeEnrichmentStatsStore) EnqueueJob(_ context.Context, job model.EnrichmentJob) (int64, error) {
	if f.enqueueErr != nil {
		return 0, f.enqueueErr
	}
	f.enqueued = append(f.enqueued, job)
	if f.nextID <= 0 {
		f.nextID = 1
	}
	id := f.nextID
	f.nextID++
	return id, nil
}

func (f *fakeEnrichmentStatsStore) GetJobByID(_ context.Context, _ int64) (*model.EnrichmentJob, error) {
	return nil, errors.New("not found")
}

func (f *fakeEnrichmentStatsStore) TryLockNextJob(_ context.Context, _ string) (*model.EnrichmentJob, error) {
	return nil, store.ErrNoAvailableEnrichmentJobs
}

func (f *fakeEnrichmentStatsStore) MarkSucceeded(_ context.Context, _ int64) error {
	return nil
}

func (f *fakeEnrichmentStatsStore) MarkFailed(_ context.Context, _ int64, _ string, _ string, _ time.Duration) error {
	return nil
}

func (f *fakeEnrichmentStatsStore) MarkDead(_ context.Context, _ int64, _ string) error {
	return nil
}

func (f *fakeEnrichmentStatsStore) RecordRunStart(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (f *fakeEnrichmentStatsStore) RecordRunFinish(_ context.Context, _ int64, _ string, _ string) error {
	return nil
}

func (f *fakeEnrichmentStatsStore) ListJobs(_ context.Context, _ model.EnrichmentJobFilters) ([]model.EnrichmentJob, error) {
	return nil, nil
}

func (f *fakeEnrichmentStatsStore) CountJobsByStatus(_ context.Context) (map[string]int64, error) {
	out := make(map[string]int64, len(f.counts))
	for k, v := range f.counts {
		out[k] = v
	}
	return out, nil
}

func (f *fakeEnrichmentStatsStore) NextRunnableAt(_ context.Context) (*time.Time, error) {
	return f.nextRunnableAt, nil
}

func TestGetEnrichmentStats_Contract_WithNextRunnableAt(t *testing.T) {
	fixed := time.Date(2026, 3, 5, 15, 12, 0, 0, time.UTC)
	fakeStore := &fakeEnrichmentStatsStore{
		counts:         map[string]int64{"queued": 2, "running": 1},
		nextRunnableAt: &fixed,
	}

	h := &Handlers{
		enrichmentStore:   fakeStore,
		enrichmentEnabled: true,
		enrichmentWorkers: 2,
	}
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/enrichment/stats", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	nextRaw, ok := body["next_runnable_at"]
	if !ok {
		t.Fatalf("expected next_runnable_at field to be present")
	}
	next, ok := nextRaw.(string)
	if !ok {
		t.Fatalf("expected next_runnable_at to be string, got %T", nextRaw)
	}
	if next != "2026-03-05T15:12:00Z" {
		t.Fatalf("expected next_runnable_at to equal 2026-03-05T15:12:00Z, got %q", next)
	}

	queueDepthRaw, ok := body["queue_depth"]
	if !ok {
		t.Fatalf("expected queue_depth field to be present")
	}
	queueDepth, ok := queueDepthRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected queue_depth to be object, got %T", queueDepthRaw)
	}
	if got := queueDepth["queued"]; got != float64(2) {
		t.Fatalf("expected queue_depth.queued = 2, got %v", got)
	}
}

func TestGetEnrichmentStats_Contract_NullNextRunnableAt(t *testing.T) {
	fakeStore := &fakeEnrichmentStatsStore{
		counts:         map[string]int64{"queued": 0},
		nextRunnableAt: nil,
	}

	h := &Handlers{enrichmentStore: fakeStore}
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/enrichment/stats", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}

	nextRaw, ok := body["next_runnable_at"]
	if !ok {
		t.Fatalf("expected next_runnable_at field to be present")
	}
	if nextRaw != nil {
		t.Fatalf("expected next_runnable_at to be null, got %v", nextRaw)
	}
}

func TestEnqueueEnrichmentJob_Contract_Success(t *testing.T) {
	fakeStore := &fakeEnrichmentStatsStore{nextID: 42}
	h := &Handlers{enrichmentStore: fakeStore}
	router := NewRouter(h)

	body := `{"job_type":"work_editions","entity_type":"work","entity_id":"w123","priority":50}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/enrichment/jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if got := resp["job_id"]; got != float64(42) {
		t.Fatalf("expected job_id=42, got %v", got)
	}

	if len(fakeStore.enqueued) != 1 {
		t.Fatalf("expected one enqueued job, got %d", len(fakeStore.enqueued))
	}
	job := fakeStore.enqueued[0]
	if job.JobType != "work_editions" || job.EntityType != "work" || job.EntityID != "w123" || job.Priority != 50 {
		t.Fatalf("unexpected enqueued job payload: %+v", job)
	}
}

func TestEnqueueEnrichmentJob_Contract_Validation(t *testing.T) {
	fakeStore := &fakeEnrichmentStatsStore{}
	h := &Handlers{enrichmentStore: fakeStore}
	router := NewRouter(h)

	body := `{"job_type":"","entity_type":"work","entity_id":"w123"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/enrichment/jobs", strings.NewReader(body))
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestEnqueueEnrichmentJob_Contract_StoreUnavailable(t *testing.T) {
	h := &Handlers{enrichmentStore: nil}
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/enrichment/jobs", strings.NewReader(`{"job_type":"work_editions","entity_type":"work","entity_id":"w123"}`))
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}
