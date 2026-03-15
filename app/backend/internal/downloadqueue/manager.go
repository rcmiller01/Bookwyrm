package downloadqueue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"time"

	"app-backend/internal/integration/download"
	"app-backend/internal/integration/indexer"
)

type Manager struct {
	store          Storage
	downloadSvc    *download.Service
	indexerClient  *indexer.Client
	quarantineMode string
}

func NewManager(store Storage, downloadSvc *download.Service, indexerClient *indexer.Client, quarantineMode string) *Manager {
	if strings.TrimSpace(quarantineMode) == "" {
		quarantineMode = "last_resort"
	}
	return &Manager{
		store:          store,
		downloadSvc:    downloadSvc,
		indexerClient:  indexerClient,
		quarantineMode: quarantineMode,
	}
}

func (m *Manager) EnqueueFromGrab(ctx context.Context, grabID int64, preferredClient string, upgradeAction string) (Job, error) {
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
	return m.enqueueGrabCandidate(grab, candidate, preferredClient, upgradeAction, appendAttemptedCandidate(nil, grab.CandidateID))
}

func (m *Manager) Start(ctx context.Context) {
	m.startWorker(ctx, "submit-worker", m.submitWorker)
	m.startWorker(ctx, "poll-worker", m.pollWorker)
	m.startWorker(ctx, "recovery-worker", m.recoveryWorker)
	m.startWorker(ctx, "reliability-worker", m.reliabilityWorker)
}

func (m *Manager) startWorker(ctx context.Context, name string, fn func(context.Context)) {
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			panicked := false
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						panicked = true
						log.Printf("download manager %s panic: %v\n%s", name, rec, string(debug.Stack()))
					}
				}()
				fn(ctx)
			}()
			if !panicked {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}()
}

func (m *Manager) ListJobs(filter JobFilter) []Job {
	return m.store.ListJobs(filter)
}

func (m *Manager) CountJobsByStatus() map[JobStatus]int {
	return m.store.CountJobsByStatus()
}

func (m *Manager) ListClients() []DownloadClientRecord {
	return m.store.ListClients()
}

func (m *Manager) UpdateClient(id string, enabled *bool, priority *int) (DownloadClientRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return DownloadClientRecord{}, ErrNotFound
	}
	for _, rec := range m.store.ListClients() {
		if rec.ID != id {
			continue
		}
		if enabled != nil {
			rec.Enabled = *enabled
		}
		if priority != nil {
			rec.Priority = *priority
		}
		return m.store.UpsertClient(rec), nil
	}
	return DownloadClientRecord{}, ErrNotFound
}

func (m *Manager) GetJob(id int64) (Job, error) {
	return m.store.GetJob(id)
}

func (m *Manager) ListEvents(jobID int64) []Event {
	return m.store.ListEvents(jobID)
}

func (m *Manager) CancelJob(id int64) error {
	if err := m.store.CancelJob(id); err != nil {
		return err
	}
	_, _ = m.store.AddEvent(Event{
		JobID:     id,
		EventType: "canceled",
		Message:   "download job canceled",
		Data:      map[string]any{"download_job_id": id},
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
		Data:      map[string]any{"download_job_id": id},
	})
	return nil
}

func (m *Manager) RecomputeReliability() error {
	return m.store.RecomputeClientReliability()
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
			start := time.Now()
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
				_ = m.store.RecordClientResult(job.ClientName, false, time.Since(start), false)
				if m.shouldTryCandidateFallback(addErr) {
					replacement, swapped, fallbackErr := m.tryFallbackCandidate(ctx, job)
					if swapped {
						_ = m.store.UpdateProgress(job.ID, JobStatusFailed, "", addErr.Error())
						_, _ = m.store.AddEvent(Event{
							JobID:     job.ID,
							EventType: "submit_failed",
							Message:   addErr.Error(),
							Data:      m.withCorrelation(job, map[string]any{"fallback_queued": true}),
						})
						_, _ = m.store.AddEvent(Event{
							JobID:     job.ID,
							EventType: "fallback_queued",
							Message:   "replacement candidate queued after fetch failure",
							Data: m.withCorrelation(job, map[string]any{
								"replacement_job_id":       replacement.ID,
								"replacement_candidate_id": replacement.CandidateID,
								"replacement_grab_id":      replacement.GrabID,
							}),
						})
						continue
					}
					if fallbackErr != nil {
						addErr = fmt.Errorf("%s; fallback error: %v", addErr.Error(), fallbackErr)
					}
				}
				_ = m.store.Reschedule(job.ID, addErr.Error(), time.Now().UTC().Add(backoffForAttempt(job.AttemptCount)), job.AttemptCount >= job.MaxAttempts)
				_, _ = m.store.AddEvent(Event{
					JobID:     job.ID,
					EventType: "submit_failed",
					Message:   addErr.Error(),
					Data:      m.withCorrelation(job, map[string]any{}),
				})
				continue
			}
			_ = m.store.RecordClientResult(job.ClientName, true, time.Since(start), false)
			_ = m.store.MarkSubmitted(job.ID, downloadID)
			_, _ = m.store.AddEvent(Event{
				JobID:     job.ID,
				EventType: "submitted",
				Message:   "download submitted",
				Data:      m.withCorrelation(job, map[string]any{"download_id": downloadID, "client": job.ClientName}),
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
				start := time.Now()
				status, err := m.getStatusWithFallback(ctx, job)
				if err != nil {
					_ = m.store.RecordClientResult(job.ClientName, false, time.Since(start), false)
					if errors.Is(err, download.ErrDownloadNotFound) {
						reason := "missing downstream job"
						_ = m.store.UpdateProgress(job.ID, JobStatusFailed, "", reason)
						_, _ = m.store.AddEvent(Event{
							JobID:     job.ID,
							EventType: "downstream_missing",
							Message:   reason,
							Data: m.withCorrelation(job, map[string]any{
								"download_id": job.DownloadID,
								"client":      job.ClientName,
							}),
						})
					} else {
						_ = m.store.UpdateProgress(job.ID, JobStatusFailed, "", err.Error())
						_, _ = m.store.AddEvent(Event{
							JobID:     job.ID,
							EventType: "poll_failed",
							Message:   err.Error(),
							Data:      m.withCorrelation(job, map[string]any{}),
						})
					}
					continue
				}
				nextStatus := normalizeState(status.State)
				_ = m.store.RecordClientResult(job.ClientName, true, time.Since(start), nextStatus == JobStatusCompleted)
				_ = m.store.UpdateProgress(job.ID, nextStatus, status.OutputPath, "")
				if nextStatus == JobStatusCompleted || nextStatus == JobStatusFailed {
					_, _ = m.store.AddEvent(Event{
						JobID:     job.ID,
						EventType: "terminal",
						Message:   string(nextStatus),
						Data:      m.withCorrelation(job, map[string]any{"output_path": status.OutputPath}),
					})
				}
			}
		}
	}
}

func (m *Manager) getStatusWithFallback(ctx context.Context, job Job) (download.DownloadStatus, error) {
	status, _, err := m.downloadSvc.GetStatus(ctx, job.ClientName, job.DownloadID)
	if err == nil {
		return status, nil
	}
	if !errors.Is(err, download.ErrDownloadNotFound) {
		return download.DownloadStatus{}, err
	}

	fallbackIDs := make([]string, 0, 2)
	if job.GrabID > 0 {
		fallbackIDs = append(fallbackIDs, fmt.Sprintf("tag:bookwyrm:grab:%d", job.GrabID))
	}
	if strings.TrimSpace(job.WorkID) != "" {
		fallbackIDs = append(fallbackIDs, fmt.Sprintf("tag:bookwyrm:work:%s", strings.TrimSpace(job.WorkID)))
	}

	for _, fallbackID := range fallbackIDs {
		resolved, _, fallbackErr := m.downloadSvc.GetStatus(ctx, job.ClientName, fallbackID)
		if fallbackErr == nil {
			return resolved, nil
		}
		if !errors.Is(fallbackErr, download.ErrDownloadNotFound) {
			return download.DownloadStatus{}, fallbackErr
		}
	}

	return download.DownloadStatus{}, download.ErrDownloadNotFound
}

func (m *Manager) reliabilityWorker(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = m.store.RecomputeClientReliability()
		}
	}
}

func (m *Manager) recoveryWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = m.store.RecoverExpiredLeases(time.Now().UTC(), 100)
		}
	}
}

func (m *Manager) pickClient(protocol string) string {
	for _, rec := range m.store.ListClients() {
		if !rec.Enabled {
			continue
		}
		if rec.Tier == "quarantine" && m.quarantineMode == "disabled" {
			continue
		}
		if protocol == "usenet" && rec.ClientType != "nzbget" && rec.ClientType != "sabnzbd" {
			continue
		}
		if protocol == "torrent" && rec.ClientType != "qbittorrent" {
			continue
		}
		if m.downloadSvc.HasClient(rec.ID) {
			return rec.ID
		}
	}
	return ""
}

func (m *Manager) withCorrelation(job Job, payload map[string]any) map[string]any {
	out := map[string]any{
		"download_job_id": job.ID,
		"grab_id":         job.GrabID,
		"candidate_id":    job.CandidateID,
		"work_id":         job.WorkID,
		"edition_id":      job.EditionID,
	}
	for k, v := range payload {
		out[k] = v
	}
	return out
}

func (m *Manager) enqueueGrabCandidate(grab indexer.GrabRecord, candidate indexer.CandidateRecord, preferredClient string, upgradeAction string, attemptedCandidates []int64) (Job, error) {
	uri := candidateURI(candidate)
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
		clientName = m.pickClient(protocol)
	}
	if clientName == "" {
		return Job{}, fmt.Errorf("no enabled download client for protocol %s", protocol)
	}
	if strings.TrimSpace(upgradeAction) == "" {
		upgradeAction = "ask"
	}

	payload := map[string]any{
		"uri":                     uri,
		"protocol":                protocol,
		"grab_id":                 grab.ID,
		"attempted_candidate_ids": attemptedCandidates,
	}
	if candidate.SearchRequestID > 0 {
		payload["search_request_id"] = candidate.SearchRequestID
	}

	job, err := m.store.CreateJob(Job{
		GrabID:         grab.ID,
		CandidateID:    grab.CandidateID,
		WorkID:         grab.EntityID,
		Protocol:       protocol,
		ClientName:     clientName,
		UpgradeAction:  upgradeAction,
		RequestPayload: payload,
		MaxAttempts:    3,
		NotBefore:      time.Now().UTC(),
	})
	if err != nil {
		return Job{}, err
	}
	_, _ = m.store.AddEvent(Event{
		JobID:     job.ID,
		EventType: "queued",
		Message:   "download job queued from grab",
		Data:      m.withCorrelation(job, map[string]any{"grab_id": grab.ID, "candidate_id": grab.CandidateID}),
		CreatedAt: time.Now().UTC(),
	})
	return job, nil
}

func (m *Manager) shouldTryCandidateFallback(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "nzb fetch status 401") ||
		strings.Contains(message, "nzb fetch status 403") ||
		strings.Contains(message, "nzb fetch status 404") ||
		strings.Contains(message, "nzb fetch status 410")
}

func (m *Manager) tryFallbackCandidate(ctx context.Context, job Job) (Job, bool, error) {
	if m.indexerClient == nil {
		return Job{}, false, nil
	}
	searchRequestID := requestSearchRequestID(job)
	if searchRequestID == 0 {
		candidate, err := m.indexerClient.GetCandidate(ctx, job.CandidateID)
		if err != nil {
			return Job{}, false, err
		}
		searchRequestID = candidate.SearchRequestID
	}
	if searchRequestID == 0 {
		return Job{}, false, nil
	}

	candidates, err := m.indexerClient.ListCandidates(ctx, searchRequestID, 25)
	if err != nil {
		return Job{}, false, err
	}
	attempted := attemptedCandidateSet(job)
	attempted[job.CandidateID] = struct{}{}
	usedForWork := usedCandidateSetForWork(m.store.ListJobs(JobFilter{Limit: 500}), job.WorkID)

	var lastErr error
	for _, candidate := range candidates {
		if candidate.ID == 0 {
			continue
		}
		if _, seen := attempted[candidate.ID]; seen {
			continue
		}
		if _, used := usedForWork[candidate.ID]; used {
			continue
		}
		if candidateURI(candidate) == "" {
			continue
		}
		if candidate.SearchRequestID == 0 {
			candidate.SearchRequestID = searchRequestID
		}
		grab, grabErr := m.indexerClient.GrabCandidate(ctx, candidate.ID)
		if grabErr != nil {
			lastErr = grabErr
			continue
		}
		replacement, enqueueErr := m.enqueueGrabCandidate(grab, candidate, job.ClientName, job.UpgradeAction, appendAttemptedCandidate(candidateIDsFromSet(attempted), candidate.ID))
		if enqueueErr != nil {
			lastErr = enqueueErr
			continue
		}
		return replacement, true, nil
	}
	return Job{}, false, lastErr
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

func candidateURI(candidate indexer.CandidateRecord) string {
	payload := candidate.Candidate.GrabPayload
	return firstNonEmpty(
		asString(payload["nzb_url"]),
		asString(payload["downloadUrl"]),
		asString(payload["magnet"]),
		asString(payload["torrent_url"]),
	)
}

func requestSearchRequestID(job Job) int64 {
	switch raw := job.RequestPayload["search_request_id"].(type) {
	case int64:
		return raw
	case int:
		return int64(raw)
	case float64:
		return int64(raw)
	default:
		return 0
	}
}

func attemptedCandidateSet(job Job) map[int64]struct{} {
	out := map[int64]struct{}{}
	for _, id := range attemptedCandidateIDs(job) {
		if id > 0 {
			out[id] = struct{}{}
		}
	}
	return out
}

func attemptedCandidateIDs(job Job) []int64 {
	raw, ok := job.RequestPayload["attempted_candidate_ids"]
	if !ok {
		if job.CandidateID > 0 {
			return []int64{job.CandidateID}
		}
		return nil
	}
	return anyToInt64Slice(raw)
}

func anyToInt64Slice(raw any) []int64 {
	switch values := raw.(type) {
	case []int64:
		out := make([]int64, 0, len(values))
		for _, value := range values {
			if value > 0 {
				out = append(out, value)
			}
		}
		return out
	case []int:
		out := make([]int64, 0, len(values))
		for _, value := range values {
			if value > 0 {
				out = append(out, int64(value))
			}
		}
		return out
	case []any:
		out := make([]int64, 0, len(values))
		for _, value := range values {
			switch typed := value.(type) {
			case int64:
				if typed > 0 {
					out = append(out, typed)
				}
			case int:
				if typed > 0 {
					out = append(out, int64(typed))
				}
			case float64:
				if typed > 0 {
					out = append(out, int64(typed))
				}
			}
		}
		return out
	default:
		return nil
	}
}

func appendAttemptedCandidate(existing []int64, candidateID int64) []int64 {
	out := make([]int64, 0, len(existing)+1)
	seen := map[int64]struct{}{}
	for _, id := range existing {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if candidateID > 0 {
		if _, ok := seen[candidateID]; !ok {
			out = append(out, candidateID)
		}
	}
	return out
}

func candidateIDsFromSet(values map[int64]struct{}) []int64 {
	out := make([]int64, 0, len(values))
	for id := range values {
		if id > 0 {
			out = append(out, id)
		}
	}
	return out
}

func usedCandidateSetForWork(jobs []Job, workID string) map[int64]struct{} {
	out := map[int64]struct{}{}
	for _, item := range jobs {
		if strings.TrimSpace(item.WorkID) != strings.TrimSpace(workID) {
			continue
		}
		if item.CandidateID > 0 {
			out[item.CandidateID] = struct{}{}
		}
		for _, id := range attemptedCandidateIDs(item) {
			if id > 0 {
				out[id] = struct{}{}
			}
		}
	}
	return out
}
