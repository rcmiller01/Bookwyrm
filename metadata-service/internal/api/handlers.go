package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"metadata-service/internal/model"
	"metadata-service/internal/provider"
	"metadata-service/internal/resolver"
	"metadata-service/internal/store"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type Handlers struct {
	resolver          resolver.Resolver
	registry          *provider.Registry
	rateLimiter       *provider.RateLimiter
	cfgStore          store.ProviderConfigStore
	statusStore       store.ProviderStatusStore
	reliabilityStore  store.ReliabilityStore
	enrichmentStore   store.EnrichmentJobStore
	enrichmentEnabled bool
	enrichmentWorkers int
	policySource      string
	policyMode        string
}

func NewHandlers(
	res resolver.Resolver,
	registry *provider.Registry,
	rl *provider.RateLimiter,
	cfgStore store.ProviderConfigStore,
	statusStore store.ProviderStatusStore,
	reliabilityStore store.ReliabilityStore,
	enrichmentStore store.EnrichmentJobStore,
	enrichmentEnabled bool,
	enrichmentWorkers int,
	policySource string,
	policyMode string,
) *Handlers {
	return &Handlers{
		resolver:          res,
		registry:          registry,
		rateLimiter:       rl,
		cfgStore:          cfgStore,
		statusStore:       statusStore,
		reliabilityStore:  reliabilityStore,
		enrichmentStore:   enrichmentStore,
		enrichmentEnabled: enrichmentEnabled,
		enrichmentWorkers: enrichmentWorkers,
		policySource:      policySource,
		policyMode:        policyMode,
	}
}

// --- Metadata endpoints ---

func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, "missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	works, err := h.resolver.SearchWorks(r.Context(), q)
	if err != nil {
		log.Error().Err(err).Str("query", q).Msg("search failed")
		writeError(w, "search failed", http.StatusInternalServerError)
		return
	}

	if works == nil {
		works = []model.Work{}
	}

	writeJSON(w, SearchResponse{Works: works})
}

func (h *Handlers) Resolve(w http.ResponseWriter, r *http.Request) {
	isbn := r.URL.Query().Get("isbn")
	if isbn == "" {
		writeError(w, "missing 'isbn' parameter", http.StatusBadRequest)
		return
	}

	idType := "ISBN_13"
	if len(isbn) == 10 {
		idType = "ISBN_10"
	}

	edition, err := h.resolver.ResolveIdentifier(r.Context(), idType, isbn)
	if err != nil {
		writeError(w, "identifier not found", http.StatusNotFound)
		return
	}

	writeJSON(w, EditionResponse{Edition: *edition})
}

func (h *Handlers) GetWork(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	work, err := h.resolver.GetWork(r.Context(), id)
	if err != nil {
		writeError(w, "work not found", http.StatusNotFound)
		return
	}

	writeJSON(w, WorkResponse{Work: *work})
}

// --- Provider management endpoints ---

func (h *Handlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	cfgs, err := h.cfgStore.GetAll(r.Context())
	if err != nil {
		writeError(w, "failed to load provider configs", http.StatusInternalServerError)
		return
	}
	statuses, _ := h.statusStore.GetAll(r.Context())
	writeJSON(w, ProvidersResponse{Providers: mergeProviderInfo(cfgs, statuses)})
}

func (h *Handlers) UpsertProvider(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	if name == "" {
		writeError(w, "missing provider name", http.StatusBadRequest)
		return
	}

	var req UpsertProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cfg, err := h.cfgStore.GetByName(r.Context(), name)
	if err != nil {
		// new provider — create with defaults
		cfg = &store.ProviderConfig{Name: name, Enabled: true, Priority: 100, TimeoutSec: 10, RateLimit: 60}
	}

	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
		h.registry.SetEnabled(name, *req.Enabled)
	}
	if req.Priority != nil {
		cfg.Priority = *req.Priority
		h.registry.SetPriority(name, *req.Priority)
	}
	if req.TimeoutSec != nil {
		cfg.TimeoutSec = *req.TimeoutSec
	}
	if req.RateLimit != nil {
		cfg.RateLimit = *req.RateLimit
		h.rateLimiter.Configure(name, *req.RateLimit)
	}
	if req.APIKey != nil {
		cfg.APIKey = *req.APIKey
	}

	if err := h.cfgStore.Upsert(r.Context(), *cfg); err != nil {
		writeError(w, "failed to save provider config", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) TestProvider(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	p, ok := h.registry.Get(name)
	if !ok {
		writeError(w, "provider not found", http.StatusNotFound)
		return
	}

	works, err := p.SearchWorks(r.Context(), "test")
	if err != nil {
		writeJSON(w, ProviderTestResponse{Provider: name, Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, ProviderTestResponse{Provider: name, Success: true, Works: works})
}

// GetProviderPolicy handles GET /v1/providers/policy.
func (h *Handlers) GetProviderPolicy(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, ProviderPolicyResponse{
		QuarantineDisableDispatch: h.registry.QuarantineDisables(),
		Source:                    h.policySource,
		Mode:                      h.policyMode,
	})
}

// ListEnrichmentJobs handles GET /v1/enrichment/jobs.
func (h *Handlers) ListEnrichmentJobs(w http.ResponseWriter, r *http.Request) {
	if h.enrichmentStore == nil {
		writeError(w, "enrichment store not configured", http.StatusServiceUnavailable)
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, "invalid limit", http.StatusBadRequest)
			return
		}
		if parsed > 200 {
			parsed = 200
		}
		limit = parsed
	}

	jobs, err := h.enrichmentStore.ListJobs(r.Context(), model.EnrichmentJobFilters{
		Status: r.URL.Query().Get("status"),
		Limit:  limit,
	})
	if err != nil {
		writeError(w, "failed to list enrichment jobs", http.StatusInternalServerError)
		return
	}
	writeJSON(w, EnrichmentJobsResponse{Jobs: jobs})
}

// EnqueueEnrichmentJob handles POST /v1/enrichment/jobs.
func (h *Handlers) EnqueueEnrichmentJob(w http.ResponseWriter, r *http.Request) {
	if h.enrichmentStore == nil {
		writeError(w, "enrichment store not configured", http.StatusServiceUnavailable)
		return
	}

	var req EnqueueEnrichmentJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.JobType = strings.TrimSpace(req.JobType)
	req.EntityType = strings.TrimSpace(req.EntityType)
	req.EntityID = strings.TrimSpace(req.EntityID)

	if req.JobType == "" || req.EntityType == "" || req.EntityID == "" {
		writeError(w, "job_type, entity_type, and entity_id are required", http.StatusBadRequest)
		return
	}
	if req.Priority < 0 {
		writeError(w, "priority must be >= 0", http.StatusBadRequest)
		return
	}

	jobID, err := h.enrichmentStore.EnqueueJob(r.Context(), model.EnrichmentJob{
		JobType:    req.JobType,
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		Priority:   req.Priority,
	})
	if err != nil {
		writeError(w, "failed to enqueue enrichment job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(EnqueueEnrichmentJobResponse{JobID: jobID})
}

// GetEnrichmentJob handles GET /v1/enrichment/jobs/{id}.
func (h *Handlers) GetEnrichmentJob(w http.ResponseWriter, r *http.Request) {
	if h.enrichmentStore == nil {
		writeError(w, "enrichment store not configured", http.StatusServiceUnavailable)
		return
	}
	rawID := mux.Vars(r)["id"]
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "invalid job id", http.StatusBadRequest)
		return
	}

	job, err := h.enrichmentStore.GetJobByID(r.Context(), id)
	if err != nil {
		writeError(w, "enrichment job not found", http.StatusNotFound)
		return
	}
	writeJSON(w, EnrichmentJobResponse{Job: *job})
}

// GetEnrichmentStats handles GET /v1/enrichment/stats.
func (h *Handlers) GetEnrichmentStats(w http.ResponseWriter, r *http.Request) {
	queueDepth := map[string]int64{}
	var nextRunnableAt *time.Time
	if h.enrichmentStore != nil {
		depth, err := h.enrichmentStore.CountJobsByStatus(r.Context())
		if err != nil {
			writeError(w, "failed to read enrichment stats", http.StatusInternalServerError)
			return
		}
		queueDepth = depth

		nextAt, err := h.enrichmentStore.NextRunnableAt(r.Context())
		if err != nil {
			writeError(w, "failed to read enrichment stats", http.StatusInternalServerError)
			return
		}
		nextRunnableAt = nextAt
	}

	writeJSON(w, EnrichmentStatsResponse{
		Enabled:        h.enrichmentEnabled,
		WorkerCount:    h.enrichmentWorkers,
		QueueDepth:     queueDepth,
		NextRunnableAt: nextRunnableAt,
	})
}

// --- Reliability endpoints ---

// ListReliabilityScores handles GET /v1/providers/reliability.
// Returns the current reliability score for every provider that has one.
func (h *Handlers) ListReliabilityScores(w http.ResponseWriter, r *http.Request) {
	if h.reliabilityStore == nil {
		writeError(w, "reliability store not configured", http.StatusServiceUnavailable)
		return
	}
	scores, err := h.reliabilityStore.GetAllScores(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("failed to load reliability scores")
		writeError(w, "failed to load reliability scores", http.StatusInternalServerError)
		return
	}
	out := make([]ReliabilityInfo, 0, len(scores))
	for _, s := range scores {
		out = append(out, reliabilityInfoFromScore(s))
	}
	writeJSON(w, ReliabilityListResponse{Providers: out})
}

// GetProviderReliability handles GET /v1/providers/{name}/reliability.
func (h *Handlers) GetProviderReliability(w http.ResponseWriter, r *http.Request) {
	if h.reliabilityStore == nil {
		writeError(w, "reliability store not configured", http.StatusServiceUnavailable)
		return
	}
	name := mux.Vars(r)["name"]
	score, err := h.reliabilityStore.GetScore(r.Context(), name)
	if err != nil {
		writeError(w, "provider reliability not found", http.StatusNotFound)
		return
	}
	writeJSON(w, ReliabilityDetailResponse{Provider: reliabilityInfoFromScore(*score)})
}

func reliabilityInfoFromScore(s store.ReliabilityScore) ReliabilityInfo {
	return ReliabilityInfo{
		Name:              s.Provider,
		Score:             s.CompositeScore,
		Availability:      s.Availability,
		LatencyScore:      s.LatencyScore,
		AgreementScore:    s.AgreementScore,
		IdentifierQuality: s.IdentifierQuality,
		Status:            healthStatusFromScore(s.CompositeScore),
	}
}

func healthStatusFromScore(score float64) string {
	switch {
	case score > 0.80:
		return "healthy"
	case score >= 0.60:
		return "degraded"
	case score >= 0.40:
		return "unreliable"
	default:
		return "quarantine"
	}
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}
