package downloadqueue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"app-backend/internal/integration/download"
	"app-backend/internal/integration/indexer"
)

type Manager struct {
	store         Storage
	downloadSvc   *download.Service
	indexerClient *indexer.Client
}

func NewManager(store Storage, downloadSvc *download.Service, indexerClient *indexer.Client) *Manager {
	return &Manager{
		store:         store,
		downloadSvc:   downloadSvc,
		indexerClient: indexerClient,
	}
}

func (m *Manager) EnqueueFromGrab(ctx context.Context, grabID int64, preferredClient string) (Job, error) {
	if m.indexerClient == nil {
		return Job{}, fmt.Errorf("indexer client not configured")
	}
	grab, err := m.indexerClient.GetGrab(ctx, grabID)
	if err != nil {
		return Job{}, err
	}
	candidate, err := m.indexerClient.GetCandidate(ctx, grab.CandidateID)
	if err != nil {
		return Job{}, err
	}
	payload := candidate.Candidate.GrabPayload
	uri := firstNonEmpty(
		asString(payload["nzb_url"]),
		asString(payload["downloadUrl"]),
		asString(payload["magnet"]),
		asString(payload["torrent_url"]),
	)
	if uri == "" {
		return Job{}, fmt.Errorf("candidate grab_payload missing downloadable uri")
	}
	protocol := strings.ToLower(strings.TrimSpace(candidate.Candidate.Protocol))
	if protocol == "" {
		if strings.HasPrefix(strings.ToLower(uri), "magnet:") {
			protocol = "torrent"
		} else {
			protocol = "usenet"
		}
	}
	clientName := strings.TrimSpace(preferredClient)
	if clientName == "" {
		switch protocol {
		case "usenet":
			clientName = "nzbget"
		case "torrent":
			clientName = "qbittorrent"
		default:
			clientName = "nzbget"
		}
	}

	job, err := m.store.CreateJob(Job{
		GrabID:      grab.ID,
		CandidateID: grab.CandidateID,
		WorkID:      grab.EntityID,
		Protocol:    protocol,
		ClientName:  clientName,
		RequestPayload: map[string]any{
			"uri":      uri,
			"protocol": protocol,
			"grab_id":  grab.ID,
		},
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	if err != nil {
		return Job{}, err
	}
	_, _ = m.store.AddEvent(Event{
		JobID:     job.ID,
		EventType: "queued",
		Message:   "download job queued from grab",
		Data:      map[string]any{"grab_id": grabID, "candidate_id": grab.CandidateID},
		CreatedAt: time.Now().UTC(),
	})
	return job, nil
}

func (m *Manager) Start(ctx context.Context) {
	go m.submitWorker(ctx)
	go m.pollWorker(ctx)
}

func (m *Manager) ListJobs(filter JobFilter) []Job {
	return m.store.ListJobs(filter)
}

func (m *Manager) GetJob(id int64) (Job, error) {
	return m.store.GetJob(id)
}

func (m *Manager) CancelJob(id int64) error {
	if err := m.store.CancelJob(id); err != nil {
		return err
	}
	_, _ = m.store.AddEvent(Event{
		JobID:     id,
		EventType: "canceled",
		Message:   "download job canceled",
		Data:      map[string]any{},
	})
	return nil
}

func (m *Manager) RetryJob(id int64) error {
	if err := m.store.RetryJob(id); err != nil {
		return err
	}
	_, _ = m.store.AddEvent(Event{
		JobID:     id,
		EventType: "retry",
		Message:   "download job retried",
		Data:      map[string]any{},
	})
	return nil
}

func (m *Manager) submitWorker(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	workerID := "download-submit-worker"
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, ok, err := m.store.ClaimNextQueued(workerID, time.Now().UTC())
			if err != nil || !ok {
				continue
			}
			uri := asString(job.RequestPayload["uri"])
			if uri == "" {
				_ = m.store.Reschedule(job.ID, "missing request_payload.uri", time.Now().UTC().Add(5*time.Second), job.AttemptCount >= job.MaxAttempts)
				continue
			}
			downloadID, _, addErr := m.downloadSvc.AddDownload(ctx, job.ClientName, download.AddRequest{
				URI: uri,
				Category: firstNonEmpty(
					"books",
					asString(job.RequestPayload["category"]),
				),
				Tags: []string{
					fmt.Sprintf("bookwyrm:grab:%d", job.GrabID),
					fmt.Sprintf("bookwyrm:work:%s", job.WorkID),
				},
			})
			if addErr != nil {
				_ = m.store.Reschedule(job.ID, addErr.Error(), time.Now().UTC().Add(backoffForAttempt(job.AttemptCount)), job.AttemptCount >= job.MaxAttempts)
				_, _ = m.store.AddEvent(Event{
					JobID:     job.ID,
					EventType: "submit_failed",
					Message:   addErr.Error(),
					Data:      map[string]any{},
				})
				continue
			}
			_ = m.store.MarkSubmitted(job.ID, downloadID)
			_, _ = m.store.AddEvent(Event{
				JobID:     job.ID,
				EventType: "submitted",
				Message:   "download submitted",
				Data:      map[string]any{"download_id": downloadID, "client": job.ClientName},
			})
		}
	}
}

func (m *Manager) pollWorker(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, job := range m.store.ListActiveJobs(100) {
				if strings.TrimSpace(job.DownloadID) == "" {
					continue
				}
				status, _, err := m.downloadSvc.GetStatus(ctx, job.ClientName, job.DownloadID)
				if err != nil {
					_ = m.store.UpdateProgress(job.ID, JobStatusFailed, "", err.Error())
					_, _ = m.store.AddEvent(Event{
						JobID:     job.ID,
						EventType: "poll_failed",
						Message:   err.Error(),
						Data:      map[string]any{},
					})
					continue
				}
				nextStatus := normalizeState(status.State)
				_ = m.store.UpdateProgress(job.ID, nextStatus, status.OutputPath, "")
				if nextStatus == JobStatusCompleted || nextStatus == JobStatusFailed {
					_, _ = m.store.AddEvent(Event{
						JobID:     job.ID,
						EventType: "terminal",
						Message:   string(nextStatus),
						Data:      map[string]any{"output_path": status.OutputPath},
					})
				}
			}
		}
	}
}

func normalizeState(state string) JobStatus {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "queued", "submitted":
		return JobStatusSubmitted
	case "repairing":
		return JobStatusRepairing
	case "unpacking":
		return JobStatusUnpacking
	case "completed":
		return JobStatusCompleted
	case "failed":
		return JobStatusFailed
	case "canceled":
		return JobStatusCanceled
	default:
		return JobStatusDownloading
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func backoffForAttempt(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Duration(1<<min(attempt, 7)) * time.Second
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
