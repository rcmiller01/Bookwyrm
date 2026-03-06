package downloadqueue

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-backend/internal/integration/download"
	"app-backend/internal/integration/indexer"
)

type fakeDownloadClient struct{}

func (f *fakeDownloadClient) Name() string { return "nzbget" }
func (f *fakeDownloadClient) AddDownload(_ context.Context, _ download.AddRequest) (string, error) {
	return "dl-1", nil
}
func (f *fakeDownloadClient) GetStatus(_ context.Context, _ string) (download.DownloadStatus, error) {
	return download.DownloadStatus{ID: "dl-1", State: "completed", OutputPath: "/downloads/completed/Dune"}, nil
}
func (f *fakeDownloadClient) Remove(_ context.Context, _ string, _ bool) error { return nil }

func TestManagerEnqueueFromGrab(t *testing.T) {
	idx := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/indexer/grabs/10":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"grab": map[string]any{
					"id":           10,
					"candidate_id": 99,
					"entity_type":  "work",
					"entity_id":    "work-1",
					"status":       "created",
				},
			})
		case "/v1/indexer/candidates/id/99":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"candidate": map[string]any{
					"id": 99,
					"candidate": map[string]any{
						"protocol": "usenet",
						"grab_payload": map[string]any{
							"nzb_url": "https://example.invalid/dune.nzb",
							"guid":    "guid-99",
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer idx.Close()

	store := NewStore()
	downloadSvc := download.NewService(&fakeDownloadClient{})
	indexerClient := indexer.NewClient(indexer.Config{BaseURL: idx.URL, Timeout: time.Second})
	manager := NewManager(store, downloadSvc, indexerClient)

	job, err := manager.EnqueueFromGrab(context.Background(), 10, "nzbget")
	if err != nil {
		t.Fatalf("enqueue from grab failed: %v", err)
	}
	if job.GrabID != 10 {
		t.Fatalf("expected grab id 10, got %d", job.GrabID)
	}
	if job.CandidateID != 99 {
		t.Fatalf("expected candidate id 99, got %d", job.CandidateID)
	}
	if job.ClientName != "nzbget" {
		t.Fatalf("expected client nzbget, got %s", job.ClientName)
	}
	if got := asString(job.RequestPayload["uri"]); got != "https://example.invalid/dune.nzb" {
		t.Fatalf("unexpected queued uri: %s", got)
	}
}
