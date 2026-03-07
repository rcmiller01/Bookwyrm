package downloadqueue

import (
	"context"
	"encoding/json"
	"errors"
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

type statusLookupClient struct {
	responses map[string]download.DownloadStatus
}

func (c *statusLookupClient) Name() string { return "qbittorrent" }

func (c *statusLookupClient) AddDownload(_ context.Context, _ download.AddRequest) (string, error) {
	return "unused", nil
}

func (c *statusLookupClient) GetStatus(_ context.Context, downloadID string) (download.DownloadStatus, error) {
	if status, ok := c.responses[downloadID]; ok {
		return status, nil
	}
	return download.DownloadStatus{}, download.ErrDownloadNotFound
}

func (c *statusLookupClient) Remove(_ context.Context, _ string, _ bool) error { return nil }

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
	manager := NewManager(store, downloadSvc, indexerClient, "last_resort")

	job, err := manager.EnqueueFromGrab(context.Background(), 10, "nzbget", "")
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
	events := store.ListEvents(job.ID)
	if len(events) == 0 {
		t.Fatalf("expected queued event to be recorded")
	}
	data := events[0].Data
	if data["download_job_id"] != job.ID {
		t.Fatalf("expected download_job_id correlation field")
	}
	if data["grab_id"] != job.GrabID {
		t.Fatalf("expected grab_id correlation field")
	}
	if data["candidate_id"] != job.CandidateID {
		t.Fatalf("expected candidate_id correlation field")
	}
	if data["work_id"] != job.WorkID {
		t.Fatalf("expected work_id correlation field")
	}
}

func TestManagerGetStatusWithFallbackByTag(t *testing.T) {
	client := &statusLookupClient{
		responses: map[string]download.DownloadStatus{
			"tag:bookwyrm:grab:42": {
				ID:         "tag:bookwyrm:grab:42",
				State:      "completed",
				OutputPath: "/downloads/completed/fallback",
			},
		},
	}

	manager := NewManager(NewStore(), download.NewService(client), nil, "last_resort")
	status, err := manager.getStatusWithFallback(context.Background(), Job{
		ID:         1,
		GrabID:     42,
		WorkID:     "work-42",
		ClientName: "qbittorrent",
		DownloadID: "missing-primary-id",
	})
	if err != nil {
		t.Fatalf("expected fallback status lookup to succeed, got err=%v", err)
	}
	if status.State != "completed" {
		t.Fatalf("expected completed status from fallback, got %s", status.State)
	}
}

func TestManagerGetStatusWithFallbackReturnsNotFound(t *testing.T) {
	manager := NewManager(NewStore(), download.NewService(&statusLookupClient{responses: map[string]download.DownloadStatus{}}), nil, "last_resort")
	_, err := manager.getStatusWithFallback(context.Background(), Job{
		ID:         1,
		GrabID:     999,
		WorkID:     "work-999",
		ClientName: "qbittorrent",
		DownloadID: "missing-primary-id",
	})
	if !errors.Is(err, download.ErrDownloadNotFound) {
		t.Fatalf("expected ErrDownloadNotFound, got %v", err)
	}
}

func TestPollWorkerMarksMissingDownstreamAsFailed(t *testing.T) {
	store := NewStore()
	job, err := store.CreateJob(Job{
		GrabID:      77,
		CandidateID: 77,
		WorkID:      "work-77",
		Protocol:    "torrent",
		ClientName:  "qbittorrent",
		NotBefore:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	locked, ok, err := store.ClaimNextQueued("worker-a", time.Now().UTC())
	if err != nil || !ok {
		t.Fatalf("claim job: ok=%v err=%v", ok, err)
	}
	if err := store.MarkSubmitted(locked.ID, "missing-downstream-id"); err != nil {
		t.Fatalf("mark submitted: %v", err)
	}

	manager := NewManager(store, download.NewService(&statusLookupClient{responses: map[string]download.DownloadStatus{}}), nil, "last_resort")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.pollWorker(ctx)

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		updated, getErr := store.GetJob(job.ID)
		if getErr == nil && updated.Status == JobStatusFailed {
			if updated.LastError != "missing downstream job" {
				t.Fatalf("expected missing downstream reason, got %q", updated.LastError)
			}
			store.mu.RLock()
			events := append([]Event(nil), store.events[job.ID]...)
			store.mu.RUnlock()
			found := false
			for _, event := range events {
				if event.EventType == "downstream_missing" {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected downstream_missing event to be recorded")
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("expected job to be failed by reconciliation poll worker")
}
