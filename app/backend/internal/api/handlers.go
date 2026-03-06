package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"app-backend/internal/domain"
	"app-backend/internal/downloadqueue"
	"app-backend/internal/importer"
	"app-backend/internal/integration/indexer"
	"app-backend/internal/integration/metadata"
	"app-backend/internal/jobs"
	"app-backend/internal/store"

	"github.com/gorilla/mux"
)

type Handlers struct {
	metaClient     *metadata.Client
	indexerClient  *indexer.Client
	watchlistStore store.WatchlistStore
	jobService     *jobs.Service
	downloadMgr    *downloadqueue.Manager
	importStore    importer.Store
	importConfig   ImportConfig
}

type ImportConfig struct {
	KeepIncoming bool
	Source       string
}

func NewHandlers(metaClient *metadata.Client, indexerClient *indexer.Client, watchlistStore store.WatchlistStore) *Handlers {
	return &Handlers{metaClient: metaClient, indexerClient: indexerClient, watchlistStore: watchlistStore}
}

func (h *Handlers) SetJobService(jobService *jobs.Service) {
	h.jobService = jobService
}

func (h *Handlers) SetDownloadManager(downloadMgr *downloadqueue.Manager) {
	h.downloadMgr = downloadMgr
}

func (h *Handlers) SetImportStore(importStore importer.Store) {
	h.importStore = importStore
}

func (h *Handlers) SetImportConfig(cfg ImportConfig) {
	h.importConfig = cfg
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"status": "ok"})
}

func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, "missing query parameter q", http.StatusBadRequest)
		return
	}
	res, err := h.metaClient.Search(r.Context(), q)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, res)
}

func (h *Handlers) GetWorkIntelligence(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	if id == "" {
		writeError(w, "missing work id", http.StatusBadRequest)
		return
	}

	workEnvelope, err := h.metaClient.GetWork(r.Context(), id)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadGateway)
		return
	}
	graph, err := h.metaClient.GetGraph(r.Context(), id)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadGateway)
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}
	recs, err := h.metaClient.GetRecommendations(r.Context(), id, limit)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadGateway)
		return
	}

	out := domain.WorkIntelligence{
		Work:            extractMap(workEnvelope, "work"),
		Graph:           graph,
		Recommendations: extractSliceMap(recs, "recommendations"),
	}
	writeJSON(w, out)
}

func (h *Handlers) GetAvailability(w http.ResponseWriter, r *http.Request) {
	if h.indexerClient == nil {
		writeError(w, "indexer client not configured", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(mux.Vars(r)["id"])
	if id == "" {
		writeError(w, "missing work id", http.StatusBadRequest)
		return
	}
	workEnvelope, err := h.metaClient.GetWork(r.Context(), id)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadGateway)
		return
	}
	work := extractMap(workEnvelope, "work")
	snapshot := h.metaClient.BuildSnapshotFromWork(work)

	groups := splitCSV(r.URL.Query().Get("groups"))
	if len(groups) == 0 {
		groups = []string{"prowlarr", "non_prowlarr"}
	}

	result, err := h.indexerClient.Search(r.Context(), indexer.SearchRequest{
		Metadata: map[string]any{
			"work_id":          snapshot.WorkID,
			"edition_id":       snapshot.EditionID,
			"isbn_10":          snapshot.ISBN10,
			"isbn_13":          snapshot.ISBN13,
			"title":            snapshot.Title,
			"authors":          snapshot.Authors,
			"language":         snapshot.Language,
			"publication_year": snapshot.PublicationYear,
		},
		RequestedCapabilities: splitCSV(r.URL.Query().Get("capabilities")),
		Priority:              strings.TrimSpace(r.URL.Query().Get("priority")),
		PolicyProfile:         strings.TrimSpace(r.URL.Query().Get("policy_profile")),
		BackendGroups:         groups,
	})
	if err != nil {
		writeError(w, err.Error(), http.StatusBadGateway)
		return
	}

	writeJSON(w, map[string]any{
		"work":             work,
		"availability":     result,
		"requested_groups": groups,
	})
}

func (h *Handlers) GetQualityReport(w http.ResponseWriter, r *http.Request) {
	limit := 25
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}
	res, err := h.metaClient.GetQualityReport(r.Context(), limit)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, res)
}

func (h *Handlers) RepairQuality(w http.ResponseWriter, r *http.Request) {
	var req domain.QualityRepairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !req.DryRun {
		writeError(w, "phase 11 backend currently allows dry-run quality repairs only", http.StatusBadRequest)
		return
	}
	res, err := h.metaClient.RepairQuality(r.Context(), req)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, res)
}

func (h *Handlers) ListWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromRequest(r)
	writeJSON(w, map[string]any{"items": h.watchlistStore.List(userID)})
}

func (h *Handlers) CreateWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromRequest(r)
	var req struct {
		TargetType domain.WatchTargetType `json:"target_type"`
		TargetID   string                 `json:"target_id"`
		Label      string                 `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.TargetType == "" || strings.TrimSpace(req.TargetID) == "" {
		writeError(w, "target_type and target_id are required", http.StatusBadRequest)
		return
	}

	item := h.watchlistStore.Create(domain.WatchlistItem{
		ID:         newID(),
		UserID:     userID,
		TargetType: req.TargetType,
		TargetID:   strings.TrimSpace(req.TargetID),
		Label:      strings.TrimSpace(req.Label),
	})
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, item)
}

func (h *Handlers) DeleteWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromRequest(r)
	id := strings.TrimSpace(mux.Vars(r)["id"])
	if id == "" {
		writeError(w, "missing watchlist id", http.StatusBadRequest)
		return
	}
	if err := h.watchlistStore.Delete(userID, id); err != nil {
		if err == store.ErrWatchlistNotFound {
			writeError(w, "watchlist item not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to delete watchlist item", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) ListJobs(w http.ResponseWriter, r *http.Request) {
	if h.jobService == nil {
		writeError(w, "job service not configured", http.StatusServiceUnavailable)
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	jobs := h.jobService.List(domain.JobFilter{
		Type:  domain.JobType(strings.TrimSpace(r.URL.Query().Get("type"))),
		State: domain.JobState(strings.TrimSpace(r.URL.Query().Get("state"))),
		Limit: limit,
	})
	writeJSON(w, map[string]any{"items": jobs})
}

func (h *Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	if h.jobService == nil {
		writeError(w, "job service not configured", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(mux.Vars(r)["id"])
	if id == "" {
		writeError(w, "missing job id", http.StatusBadRequest)
		return
	}
	job, err := h.jobService.Get(id)
	if err != nil {
		if err == store.ErrJobNotFound {
			writeError(w, "job not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to read job", http.StatusInternalServerError)
		return
	}
	writeJSON(w, job)
}

func (h *Handlers) EnqueueJob(w http.ResponseWriter, r *http.Request) {
	if h.jobService == nil {
		writeError(w, "job service not configured", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Type        domain.JobType `json:"type"`
		Payload     map[string]any `json:"payload"`
		RunAt       *time.Time     `json:"run_at,omitempty"`
		MaxAttempts int            `json:"max_attempts,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		writeError(w, "type is required", http.StatusBadRequest)
		return
	}
	runAt := time.Now().UTC()
	if req.RunAt != nil {
		runAt = req.RunAt.UTC()
	}
	job := h.jobService.Enqueue(req.Type, req.Payload, runAt, req.MaxAttempts)
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, job)
}

func (h *Handlers) RetryJob(w http.ResponseWriter, r *http.Request) {
	if h.jobService == nil {
		writeError(w, "job service not configured", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(mux.Vars(r)["id"])
	job, err := h.jobService.Retry(id)
	if err != nil {
		if err == store.ErrJobNotFound {
			writeError(w, "job not found", http.StatusNotFound)
			return
		}
		if err == store.ErrJobNotRunnable {
			writeError(w, "job not retryable", http.StatusConflict)
			return
		}
		writeError(w, "failed to retry job", http.StatusInternalServerError)
		return
	}
	writeJSON(w, job)
}

func (h *Handlers) CancelJob(w http.ResponseWriter, r *http.Request) {
	if h.jobService == nil {
		writeError(w, "job service not configured", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(mux.Vars(r)["id"])
	job, err := h.jobService.Cancel(id)
	if err != nil {
		if err == store.ErrJobNotFound {
			writeError(w, "job not found", http.StatusNotFound)
			return
		}
		if err == store.ErrJobNotRunnable {
			writeError(w, "job not cancelable", http.StatusConflict)
			return
		}
		writeError(w, "failed to cancel job", http.StatusInternalServerError)
		return
	}
	writeJSON(w, job)
}

func (h *Handlers) ListDownloadJobs(w http.ResponseWriter, r *http.Request) {
	if h.downloadMgr == nil {
		writeError(w, "download manager not configured", http.StatusServiceUnavailable)
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	filter := downloadqueue.JobFilter{
		Status: downloadqueue.JobStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		Limit:  limit,
	}
	if rawImported := strings.TrimSpace(r.URL.Query().Get("imported")); rawImported != "" {
		imported := strings.EqualFold(rawImported, "true") || rawImported == "1"
		filter.Imported = &imported
	}
	items := h.downloadMgr.ListJobs(filter)
	writeJSON(w, map[string]any{"items": items})
}

func (h *Handlers) GetDownloadJob(w http.ResponseWriter, r *http.Request) {
	if h.downloadMgr == nil {
		writeError(w, "download manager not configured", http.StatusServiceUnavailable)
		return
	}
	jobID, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["id"]), 10, 64)
	if err != nil || jobID <= 0 {
		writeError(w, "invalid download job id", http.StatusBadRequest)
		return
	}
	job, err := h.downloadMgr.GetJob(jobID)
	if err != nil {
		if err == downloadqueue.ErrNotFound {
			writeError(w, "download job not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to load download job", http.StatusInternalServerError)
		return
	}
	writeJSON(w, job)
}

func (h *Handlers) CreateDownloadFromGrab(w http.ResponseWriter, r *http.Request) {
	if h.downloadMgr == nil {
		writeError(w, "download manager not configured", http.StatusServiceUnavailable)
		return
	}
	grabID, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["grabID"]), 10, 64)
	if err != nil || grabID <= 0 {
		writeError(w, "invalid grab id", http.StatusBadRequest)
		return
	}
	var body struct {
		Client string `json:"client"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	job, err := h.downloadMgr.EnqueueFromGrab(r.Context(), grabID, strings.TrimSpace(body.Client))
	if err != nil {
		writeError(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{"job": job})
}

func (h *Handlers) CancelDownloadJob(w http.ResponseWriter, r *http.Request) {
	if h.downloadMgr == nil {
		writeError(w, "download manager not configured", http.StatusServiceUnavailable)
		return
	}
	jobID, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["id"]), 10, 64)
	if err != nil || jobID <= 0 {
		writeError(w, "invalid download job id", http.StatusBadRequest)
		return
	}
	if err := h.downloadMgr.CancelJob(jobID); err != nil {
		if err == downloadqueue.ErrNotFound {
			writeError(w, "download job not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to cancel download job", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) RetryDownloadJob(w http.ResponseWriter, r *http.Request) {
	if h.downloadMgr == nil {
		writeError(w, "download manager not configured", http.StatusServiceUnavailable)
		return
	}
	jobID, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["id"]), 10, 64)
	if err != nil || jobID <= 0 {
		writeError(w, "invalid download job id", http.StatusBadRequest)
		return
	}
	if err := h.downloadMgr.RetryJob(jobID); err != nil {
		if err == downloadqueue.ErrNotFound {
			writeError(w, "download job not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to retry download job", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) ListImportJobs(w http.ResponseWriter, r *http.Request) {
	if h.importStore == nil {
		writeError(w, "import store not configured", http.StatusServiceUnavailable)
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	filter := importer.JobFilter{
		Status: importer.JobStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		Limit:  limit,
	}
	writeJSON(w, map[string]any{"items": h.importStore.ListJobs(filter)})
}

func (h *Handlers) GetImportJob(w http.ResponseWriter, r *http.Request) {
	if h.importStore == nil {
		writeError(w, "import store not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["id"]), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "invalid import job id", http.StatusBadRequest)
		return
	}
	job, err := h.importStore.GetJob(id)
	if err != nil {
		if err == importer.ErrNotFound {
			writeError(w, "import job not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to load import job", http.StatusInternalServerError)
		return
	}
	events := h.importStore.ListEvents(id)
	writeJSON(w, map[string]any{"job": job, "events": events})
}

func (h *Handlers) ApproveImportJob(w http.ResponseWriter, r *http.Request) {
	if h.importStore == nil {
		writeError(w, "import store not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["id"]), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "invalid import job id", http.StatusBadRequest)
		return
	}
	var body struct {
		WorkID           string `json:"work_id"`
		EditionID        string `json:"edition_id"`
		TemplateOverride string `json:"template_override"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.WorkID) == "" {
		writeError(w, "work_id is required", http.StatusBadRequest)
		return
	}
	if err := h.importStore.Approve(id, strings.TrimSpace(body.WorkID), strings.TrimSpace(body.EditionID), strings.TrimSpace(body.TemplateOverride)); err != nil {
		if err == importer.ErrNotFound {
			writeError(w, "import job not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to approve import job", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) RetryImportJob(w http.ResponseWriter, r *http.Request) {
	if h.importStore == nil {
		writeError(w, "import store not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["id"]), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "invalid import job id", http.StatusBadRequest)
		return
	}
	if err := h.importStore.Retry(id); err != nil {
		if err == importer.ErrNotFound {
			writeError(w, "import job not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to retry import job", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) SkipImportJob(w http.ResponseWriter, r *http.Request) {
	if h.importStore == nil {
		writeError(w, "import store not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["id"]), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "invalid import job id", http.StatusBadRequest)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.importStore.Skip(id, strings.TrimSpace(body.Reason)); err != nil {
		if err == importer.ErrNotFound {
			writeError(w, "import job not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to skip import job", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) ListLibraryItems(w http.ResponseWriter, r *http.Request) {
	if h.importStore == nil {
		writeError(w, "import store not configured", http.StatusServiceUnavailable)
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	workID := strings.TrimSpace(r.URL.Query().Get("work_id"))
	items := h.importStore.ListLibraryItems(workID, limit)
	writeJSON(w, map[string]any{"items": items})
}

func (h *Handlers) GetImportStats(w http.ResponseWriter, _ *http.Request) {
	if h.importStore == nil {
		writeError(w, "import store not configured", http.StatusServiceUnavailable)
		return
	}
	counts := h.importStore.CountJobsByStatus()
	writeJSON(w, map[string]any{
		"counts": map[string]int{
			"queued":       counts[importer.JobStatusQueued],
			"running":      counts[importer.JobStatusRunning],
			"needs_review": counts[importer.JobStatusNeedsReview],
			"imported":     counts[importer.JobStatusImported],
			"failed":       counts[importer.JobStatusFailed],
			"skipped":      counts[importer.JobStatusSkipped],
		},
		"next_runnable_at": nil,
		"keep_incoming": map[string]any{
			"value":  h.importConfig.KeepIncoming,
			"source": fallbackString(h.importConfig.Source, "default"),
		},
	})
}

func userIDFromRequest(r *http.Request) string {
	if id := strings.TrimSpace(r.Header.Get("X-User-ID")); id != "" {
		return id
	}
	return "local-user"
}

func newID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "watch-unknown"
	}
	return "watch-" + hex.EncodeToString(buf)
}

func extractMap(value map[string]any, key string) map[string]any {
	if m, ok := value[key].(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func extractSliceMap(value map[string]any, key string) []map[string]any {
	raw, ok := value[key].([]any)
	if !ok {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func fallbackString(v string, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
