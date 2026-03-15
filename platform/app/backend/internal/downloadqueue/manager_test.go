package downloadqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

type fallbackSubmitClient struct {
	mu         sync.Mutex
	addedURIs  []string
	successIDs int
}

func (c *fallbackSubmitClient) Name() string { return "nzbget" }

func (c *fallbackSubmitClient) AddDownload(_ context.Context, req download.AddRequest) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addedURIs = append(c.addedURIs, req.URI)
	if strings.Contains(req.URI, "forbidden") {
		return "", fmt.Errorf("nzb fetch status 403")
	}
	c.successIDs++
	return fmt.Sprintf("dl-%d", c.successIDs), nil
}

func (c *fallbackSubmitClient) GetStatus(_ context.Context, downloadID string) (download.DownloadStatus, error) {
	return download.DownloadStatus{ID: downloadID, State: "submitted"}, nil
}

func (c *fallbackSubmitClient) Remove(_ context.Context, _ string, _ bool) error { return nil }

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
					"id":                99,
					"search_request_id": 55,
					"candidate": map[string]any{
						"title":    "Dune",
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
	if got := requestSearchRequestID(job); got != 55 {
		t.Fatalf("expected search request id 55, got %d", got)
	}
	attempted := attemptedCandidateIDs(job)
	if len(attempted) != 1 || attempted[0] != 99 {
		t.Fatalf("expected candidate 99 to be tracked, got %v", attempted)
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

func TestSubmitWorkerQueuesFallbackCandidateAfterFetch403(t *testing.T) {
	idx := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/indexer/grabs/10":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"grab": map[string]any{
					"id":           10,
					"candidate_id": 99,
					"entity_type":  "work",
					"entity_id":    "work-1",
					"status":       "created",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/indexer/candidates/id/99":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"candidate": map[string]any{
					"id":                99,
					"search_request_id": 123,
					"candidate": map[string]any{
						"title":    "Bad candidate",
						"protocol": "usenet",
						"score":    0.9,
						"grab_payload": map[string]any{
							"nzb_url": "https://example.invalid/forbidden.nzb",
						},
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/indexer/candidates/123":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{
						"id":                99,
						"search_request_id": 123,
						"candidate": map[string]any{
							"title":        "Bad candidate",
							"protocol":     "usenet",
							"score":        0.9,
							"grab_payload": map[string]any{"nzb_url": "https://example.invalid/forbidden.nzb"},
						},
					},
					{
						"id":                100,
						"search_request_id": 123,
						"candidate": map[string]any{
							"title":        "Good candidate",
							"protocol":     "usenet",
							"score":        0.8,
							"grab_payload": map[string]any{"nzb_url": "https://example.invalid/good.nzb"},
						},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/indexer/grab/100":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"grab": map[string]any{
					"id":           11,
					"candidate_id": 100,
					"entity_type":  "work",
					"entity_id":    "work-1",
					"status":       "created",
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer idx.Close()

	store := NewStore()
	store.UpsertClient(DownloadClientRecord{ID: "nzbget", Name: "NZBGet", ClientType: "nzbget", Enabled: true, Tier: "primary", Priority: 1})
	client := &fallbackSubmitClient{}
	manager := NewManager(store, download.NewService(client), indexer.NewClient(indexer.Config{BaseURL: idx.URL, Timeout: time.Second}), "last_resort")

	original, err := manager.EnqueueFromGrab(context.Background(), 10, "nzbget", "")
	if err != nil {
		t.Fatalf("enqueue from grab: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.submitWorker(ctx)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		jobs := store.ListJobs(JobFilter{Limit: 20})
		if len(jobs) >= 2 {
			var failedOriginal Job
			var replacement Job
			for _, job := range jobs {
				if job.ID == original.ID {
					failedOriginal = job
				} else if job.CandidateID == 100 {
					replacement = job
				}
			}
			if failedOriginal.Status == JobStatusFailed && replacement.Status == JobStatusDownloading {
				if failedOriginal.LastError != "nzb fetch status 403" {
					t.Fatalf("expected original error to be preserved, got %q", failedOriginal.LastError)
				}
				attempted := attemptedCandidateIDs(replacement)
				if len(attempted) != 2 || attempted[0] != 99 || attempted[1] != 100 {
					t.Fatalf("expected fallback job to track both candidates, got %v", attempted)
				}
				events := store.ListEvents(original.ID)
				foundFallbackEvent := false
				for _, event := range events {
					if event.EventType == "fallback_queued" {
						foundFallbackEvent = true
						break
					}
				}
				if !foundFallbackEvent {
					t.Fatalf("expected fallback_queued event on original job")
				}
				client.mu.Lock()
				added := append([]string(nil), client.addedURIs...)
				client.mu.Unlock()
				if len(added) < 2 || added[0] != "https://example.invalid/forbidden.nzb" || added[1] != "https://example.invalid/good.nzb" {
					t.Fatalf("unexpected submit order: %v", added)
				}
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	jobs := store.ListJobs(JobFilter{Limit: 20})
	t.Fatalf("expected fallback replacement to be queued and submitted, jobs=%+v", jobs)
}
