package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"app-backend/internal/domain"
	"app-backend/internal/domain/contract"
	"app-backend/internal/domain/factory"
	"app-backend/internal/downloadqueue"
	"app-backend/internal/importer"
	"app-backend/internal/integration/download"
	"app-backend/internal/integration/indexer"
	"app-backend/internal/integration/metadata"
	"app-backend/internal/jobs"
	"app-backend/internal/pipeline"
	"app-backend/internal/store"
	"app-backend/internal/version"

	"github.com/gorilla/mux"
)

type Handlers struct {
	metaClient        *metadata.Client
	indexerClient     *indexer.Client
	watchlistStore    store.WatchlistStore
	jobService        *jobs.Service
	downloadMgr       *downloadqueue.Manager
	downloadService   *download.Service
	importStore       importer.Store
	importEngine      *importer.Engine
	importConfig      ImportConfig
	domainPack        contract.Domain
	importer          *pipeline.ImporterPipeline
	renamer           *pipeline.RenamerPipeline
	metadataBaseURL   string
	indexerBaseURL    string
	metadataHealthURL string
	indexerHealthURL  string
	startupTime       time.Time
	libraryRoot       string
}

type ImportConfig struct {
	KeepIncoming bool
	Source       string
	LibraryRoot  string
}

func NewHandlers(metaClient *metadata.Client, indexerClient *indexer.Client, watchlistStore store.WatchlistStore) *Handlers {
	domainPack, err := factory.Resolve("books")
	if err != nil {
		panic(err)
	}
	return NewHandlersWithDomain(metaClient, indexerClient, watchlistStore, domainPack)
}

func NewHandlersWithDomain(
	metaClient *metadata.Client,
	indexerClient *indexer.Client,
	watchlistStore store.WatchlistStore,
	domainPack contract.Domain,
) *Handlers {
	if domainPack == nil {
		resolved, err := factory.Resolve("books")
		if err != nil {
			panic(err)
		}
		domainPack = resolved
	}

	return &Handlers{
		metaClient:     metaClient,
		indexerClient:  indexerClient,
		watchlistStore: watchlistStore,
		domainPack:     domainPack,
		importer:       pipeline.NewImporterPipeline(domainPack),
		renamer:        pipeline.NewRenamerPipeline(domainPack),
	}
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

func (h *Handlers) SetImportEngine(engine *importer.Engine) {
	h.importEngine = engine
}

func (h *Handlers) SetImportConfig(cfg ImportConfig) {
	h.importConfig = cfg
}

func (h *Handlers) SetDownloadService(svc *download.Service) {
	h.downloadService = svc
}

func (h *Handlers) SetUpstreamURLs(metadataBaseURL, indexerBaseURL string) {
	h.metadataBaseURL = strings.TrimRight(strings.TrimSpace(metadataBaseURL), "/")
	h.indexerBaseURL = strings.TrimRight(strings.TrimSpace(indexerBaseURL), "/")
	h.metadataHealthURL = strings.TrimRight(metadataBaseURL, "/") + "/health"
	h.indexerHealthURL = strings.TrimRight(indexerBaseURL, "/") + "/v1/indexer/health"
}

func (h *Handlers) SetStartupTime(t time.Time) {
	h.startupTime = t
}

func (h *Handlers) SetLibraryRoot(root string) {
	h.libraryRoot = strings.TrimSpace(root)
}

func (h *Handlers) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"status":  "ok",
		"version": version.Version,
		"commit":  version.Commit,
		"built":   version.BuildDate,
	})
}

func (h *Handlers) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]string{}
	allOK := true

	var mu sync.Mutex
	var wg sync.WaitGroup

	checkUpstream := func(name, healthURL string) {
		defer wg.Done()
		status := "ok"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if err != nil {
			status = fmt.Sprintf("error: %v", err)
		} else {
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				status = fmt.Sprintf("unreachable: %v", err)
			} else {
				resp.Body.Close()
				if resp.StatusCode >= 300 {
					status = fmt.Sprintf("status %d", resp.StatusCode)
				}
			}
		}
		mu.Lock()
		checks[name] = status
		if status != "ok" {
			allOK = false
		}
		mu.Unlock()
	}

	if h.metadataHealthURL != "" {
		wg.Add(1)
		go checkUpstream("metadata_service", h.metadataHealthURL)
	}
	if h.indexerHealthURL != "" {
		wg.Add(1)
		go checkUpstream("indexer_service", h.indexerHealthURL)
	}
	wg.Wait()

	status := "ok"
	code := http.StatusOK
	if !allOK {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  status,
		"version": version.Version,
		"commit":  version.Commit,
		"built":   version.BuildDate,
		"checks":  checks,
	})
}

// Health preserves backward compatibility with the original /api/v1/health endpoint.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	h.Healthz(w, r)
}

func (h *Handlers) HealthDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	type SubsystemCheck struct {
		Name     string `json:"name"`
		Status   string `json:"status"`
		Error    string `json:"error,omitempty"`
		Guidance string `json:"guidance,omitempty"`
	}

	checks := make([]SubsystemCheck, 0, 8)
	var mu sync.Mutex
	var wg sync.WaitGroup

	checkHTTP := func(name, healthURL, envVar string) {
		defer wg.Done()
		check := SubsystemCheck{Name: name, Status: "ok"}
		if strings.TrimSpace(healthURL) == "" {
			check.Status = "unconfigured"
			check.Guidance = "Set " + envVar + " to enable this service"
		} else {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL+"z", nil)
			if err != nil {
				check.Status = "error"
				check.Error = err.Error()
				check.Guidance = "Check " + envVar + " value"
			} else {
				resp, doErr := http.DefaultClient.Do(req)
				if doErr != nil {
					check.Status = "unreachable"
					check.Error = doErr.Error()
					check.Guidance = "Verify " + envVar + " is correct and the service is running"
				} else {
					resp.Body.Close()
					if resp.StatusCode >= 300 {
						check.Status = "degraded"
						check.Error = fmt.Sprintf("status %d", resp.StatusCode)
						check.Guidance = "Service responded with non-OK status; check its logs"
					}
				}
			}
		}
		mu.Lock()
		checks = append(checks, check)
		mu.Unlock()
	}

	wg.Add(2)
	go checkHTTP("metadata_service", h.metadataHealthURL, "METADATA_SERVICE_URL")
	go checkHTTP("indexer_service", h.indexerHealthURL, "INDEXER_SERVICE_URL")

	// Check download clients
	if h.downloadMgr != nil {
		for _, client := range h.downloadMgr.ListClients() {
			if !client.Enabled {
				continue
			}
			wg.Add(1)
			go func(clientID, clientType string) {
				defer wg.Done()
				check := SubsystemCheck{Name: "download_client:" + clientID, Status: "ok"}
				if h.downloadService != nil && !h.downloadService.HasClient(clientID) {
					check.Status = "disconnected"
					check.Error = "client not registered in download service"
					check.Guidance = "Check " + strings.ToUpper(clientType) + " configuration"
				}
				mu.Lock()
				checks = append(checks, check)
				mu.Unlock()
			}(client.ID, client.ClientType)
		}
	}

	wg.Wait()

	allOK := true
	for _, c := range checks {
		if c.Status != "ok" {
			allOK = false
			break
		}
	}
	overall := "healthy"
	if !allOK {
		overall = "degraded"
	}
	writeJSON(w, map[string]any{
		"status": overall,
		"checks": checks,
	})
}

func (h *Handlers) SystemStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	services := map[string]any{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	checkService := func(name, healthURL string) {
		defer wg.Done()
		info := map[string]any{"status": "ok"}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL+"z", nil)
		if err != nil {
			info["status"] = "unreachable"
			info["error"] = err.Error()
		} else {
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				info["status"] = "unreachable"
				info["error"] = err.Error()
			} else {
				defer resp.Body.Close()
				var body map[string]any
				if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
					if v, ok := body["version"]; ok {
						info["version"] = v
					}
					if c, ok := body["commit"]; ok {
						info["commit"] = c
					}
				}
				if resp.StatusCode >= 300 {
					info["status"] = fmt.Sprintf("status %d", resp.StatusCode)
				}
			}
		}
		mu.Lock()
		services[name] = info
		mu.Unlock()
	}

	if h.metadataHealthURL != "" {
		wg.Add(1)
		go checkService("metadata_service", h.metadataHealthURL)
	}
	if h.indexerHealthURL != "" {
		wg.Add(1)
		go checkService("indexer_service", h.indexerHealthURL)
	}
	wg.Wait()

	libraryExists := false
	if h.libraryRoot != "" {
		if info, err := os.Stat(h.libraryRoot); err == nil && info.IsDir() {
			libraryExists = true
		}
	}

	var downloadClients []string
	if h.downloadService != nil {
		downloadClients = h.downloadService.ListClientNames()
	}
	if downloadClients == nil {
		downloadClients = []string{}
	}

	writeJSON(w, map[string]any{
		"version":          version.Version,
		"commit":           version.Commit,
		"built":            version.BuildDate,
		"go_version":       runtime.Version(),
		"startup_time":     h.startupTime.Format(time.RFC3339),
		"services":         services,
		"library_root":     h.libraryRoot,
		"library_exists":   libraryExists,
		"download_clients": downloadClients,
	})
}

func (h *Handlers) TestDownloadClient(w http.ResponseWriter, r *http.Request) {
	clientID := mux.Vars(r)["id"]
	if clientID == "" {
		writeError(w, "missing client id", http.StatusBadRequest)
		return
	}
	if h.downloadService == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "download service not configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.downloadService.TestConnection(ctx, clientID); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
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

	querySpec := h.importer.BuildSearchSpec(
		snapshot,
		splitCSV(r.URL.Query().Get("capabilities")),
		strings.TrimSpace(r.URL.Query().Get("priority")),
		strings.TrimSpace(r.URL.Query().Get("policy_profile")),
		groups,
	)

	result, err := h.indexerClient.Search(r.Context(), indexer.SearchRequest{
		Metadata:              querySpec.Metadata,
		RequestedCapabilities: querySpec.RequestedCapabilities,
		Priority:              querySpec.Priority,
		PolicyProfile:         querySpec.PolicyProfile,
		BackendGroups:         querySpec.BackendGroups,
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

func (h *Handlers) GetWorkTimeline(w http.ResponseWriter, r *http.Request) {
	if h.importStore == nil {
		writeError(w, "import store not configured", http.StatusServiceUnavailable)
		return
	}
	if h.downloadMgr == nil {
		writeError(w, "download manager not configured", http.StatusServiceUnavailable)
		return
	}
	workID := strings.TrimSpace(mux.Vars(r)["id"])
	if workID == "" {
		writeError(w, "missing work id", http.StatusBadRequest)
		return
	}

	allImports := h.importStore.ListJobs(importer.JobFilter{Limit: 500})
	imports := make([]map[string]any, 0)
	downloadIDs := make(map[int64]struct{})
	for _, job := range allImports {
		if strings.TrimSpace(job.WorkID) != workID {
			continue
		}
		events := h.importStore.ListEvents(job.ID)
		imports = append(imports, map[string]any{
			"job":    job,
			"events": events,
		})
		downloadIDs[job.DownloadJobID] = struct{}{}
	}

	downloads := make([]map[string]any, 0, len(downloadIDs))
	searches := make([]map[string]any, 0, len(downloadIDs))
	grabs := make([]map[string]any, 0, len(downloadIDs))
	for downloadID := range downloadIDs {
		job, err := h.downloadMgr.GetJob(downloadID)
		if err != nil {
			continue
		}
		downloads = append(downloads, map[string]any{
			"job":    job,
			"events": h.downloadMgr.ListEvents(downloadID),
		})
		searches = append(searches, map[string]any{
			"search_request_id": nil,
			"candidate_id":      job.CandidateID,
			"grab_id":           job.GrabID,
			"download_job_id":   job.ID,
			"status":            job.Status,
			"created_at":        job.CreatedAt,
			"updated_at":        job.UpdatedAt,
		})
		grabs = append(grabs, map[string]any{
			"grab_id":         job.GrabID,
			"candidate_id":    job.CandidateID,
			"download_job_id": job.ID,
			"protocol":        job.Protocol,
			"status":          job.Status,
		})
	}

	libraryItems := h.importStore.ListLibraryItems(workID, 200)
	writeJSON(w, map[string]any{
		"work_id": workID,
		"timeline": map[string]any{
			"searches":      searches,
			"grabs":         grabs,
			"downloads":     downloads,
			"imports":       imports,
			"library_items": libraryItems,
		},
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

func (h *Handlers) ListDownloadClients(w http.ResponseWriter, _ *http.Request) {
	if h.downloadMgr == nil {
		writeError(w, "download manager not configured", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]any{"items": h.downloadMgr.ListClients()})
}

func (h *Handlers) UpdateDownloadClient(w http.ResponseWriter, r *http.Request) {
	if h.downloadMgr == nil {
		writeError(w, "download manager not configured", http.StatusServiceUnavailable)
		return
	}
	id := strings.TrimSpace(mux.Vars(r)["id"])
	if id == "" {
		writeError(w, "invalid client id", http.StatusBadRequest)
		return
	}
	var body struct {
		Enabled  *bool `json:"enabled"`
		Priority *int  `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Enabled == nil && body.Priority == nil {
		writeError(w, "at least one field is required", http.StatusBadRequest)
		return
	}
	if body.Priority != nil && *body.Priority < 1 {
		writeError(w, "priority must be >= 1", http.StatusBadRequest)
		return
	}
	rec, err := h.downloadMgr.UpdateClient(id, body.Enabled, body.Priority)
	if err != nil {
		if err == downloadqueue.ErrNotFound {
			writeError(w, "download client not found", http.StatusNotFound)
			return
		}
		writeError(w, "failed to update download client", http.StatusInternalServerError)
		return
	}
	writeJSON(w, rec)
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
		Client        string `json:"client"`
		UpgradeAction string `json:"upgrade_action"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	job, err := h.downloadMgr.EnqueueFromGrab(r.Context(), grabID, strings.TrimSpace(body.Client), strings.TrimSpace(body.UpgradeAction))
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

func (h *Handlers) DecideImportJob(w http.ResponseWriter, r *http.Request) {
	if h.importStore == nil {
		writeError(w, "import store not configured", http.StatusServiceUnavailable)
		return
	}
	if h.importEngine == nil {
		writeError(w, "import decision engine not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["id"]), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "invalid import job id", http.StatusBadRequest)
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	action := importer.DecisionAction(strings.TrimSpace(body.Action))
	if !importer.IsValidDecisionAction(action) {
		writeError(w, "invalid action", http.StatusBadRequest)
		return
	}
	if err := h.importEngine.Decide(id, action); err != nil {
		if err == importer.ErrNotFound {
			writeError(w, "import job not found", http.StatusNotFound)
			return
		}
		if err == importer.ErrInvalidDecisionAction {
			writeError(w, "invalid action", http.StatusBadRequest)
			return
		}
		writeError(w, err.Error(), http.StatusConflict)
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
	nextRunnable := h.importStore.NextRunnableAt()
	writeJSON(w, map[string]any{
		"counts_by_status": map[string]int{
			"queued":       counts[importer.JobStatusQueued],
			"running":      counts[importer.JobStatusRunning],
			"needs_review": counts[importer.JobStatusNeedsReview],
			"imported":     counts[importer.JobStatusImported],
			"failed":       counts[importer.JobStatusFailed],
			"skipped":      counts[importer.JobStatusSkipped],
		},
		"next_runnable_at":        nextRunnable,
		"keep_incoming":           h.importConfig.KeepIncoming,
		"keep_incoming_source":    fallbackString(h.importConfig.Source, "default"),
		"library_root":            strings.TrimSpace(h.importConfig.LibraryRoot),
		"library_root_configured": strings.TrimSpace(h.importConfig.LibraryRoot) != "",
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

func writeStructuredError(w http.ResponseWriter, code string, message string, guidance string, category string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":    code,
		"message":  message,
		"guidance": guidance,
		"category": category,
	})
}
