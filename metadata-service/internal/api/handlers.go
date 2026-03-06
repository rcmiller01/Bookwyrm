package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"metadata-service/internal/model"
	"metadata-service/internal/provider"
	"metadata-service/internal/quality"
	"metadata-service/internal/recommend"
	"metadata-service/internal/resolver"
	"metadata-service/internal/store"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type Handlers struct {
	resolver          resolver.Resolver
	recommender       recommendationService
	qualityEngine     qualityEngineService
	registry          *provider.Registry
	rateLimiter       *provider.RateLimiter
	cfgStore          store.ProviderConfigStore
	statusStore       store.ProviderStatusStore
	reliabilityStore  store.ReliabilityStore
	enrichmentStore   store.EnrichmentJobStore
	workStore         store.WorkStore
	seriesStore       store.SeriesStore
	subjectStore      store.SubjectStore
	workRelStore      store.WorkRelationshipStore
	enrichmentEnabled bool
	enrichmentWorkers int
	policySource      string
	policyMode        string
}

type recommendationService interface {
	Recommend(ctx context.Context, req recommend.RecommendationRequest) ([]recommend.RecommendationResult, error)
	RecommendNextInSeries(ctx context.Context, workID string) (*recommend.RecommendationResult, error)
	RecommendSimilar(ctx context.Context, workID string, limit int, preferences recommend.RecommendationPreferences) ([]recommend.RecommendationResult, error)
}

type qualityEngineService interface {
	Audit(ctx context.Context, limit int) (*quality.AuditReport, error)
	Repair(ctx context.Context, req quality.RepairRequest) (*quality.RepairResult, error)
}

func NewHandlers(
	res resolver.Resolver,
	recommender recommendationService,
	qualityEngine qualityEngineService,
	registry *provider.Registry,
	rl *provider.RateLimiter,
	cfgStore store.ProviderConfigStore,
	statusStore store.ProviderStatusStore,
	reliabilityStore store.ReliabilityStore,
	enrichmentStore store.EnrichmentJobStore,
	workStore store.WorkStore,
	seriesStore store.SeriesStore,
	subjectStore store.SubjectStore,
	workRelStore store.WorkRelationshipStore,
	enrichmentEnabled bool,
	enrichmentWorkers int,
	policySource string,
	policyMode string,
) *Handlers {
	return &Handlers{
		resolver:          res,
		recommender:       recommender,
		qualityEngine:     qualityEngine,
		registry:          registry,
		rateLimiter:       rl,
		cfgStore:          cfgStore,
		statusStore:       statusStore,
		reliabilityStore:  reliabilityStore,
		enrichmentStore:   enrichmentStore,
		workStore:         workStore,
		seriesStore:       seriesStore,
		subjectStore:      subjectStore,
		workRelStore:      workRelStore,
		enrichmentEnabled: enrichmentEnabled,
		enrichmentWorkers: enrichmentWorkers,
		policySource:      policySource,
		policyMode:        policyMode,
	}
}

func splitCSVParam(value string) []string {
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

// GetWorkGraph handles GET /v1/work/{id}/graph.
func (h *Handlers) GetWorkGraph(w http.ResponseWriter, r *http.Request) {
	if h.workStore == nil || h.seriesStore == nil || h.subjectStore == nil || h.workRelStore == nil {
		writeError(w, "graph stores not configured", http.StatusServiceUnavailable)
		return
	}
	workID := mux.Vars(r)["id"]
	if strings.TrimSpace(workID) == "" {
		writeError(w, "missing work id", http.StatusBadRequest)
		return
	}

	series, _ := h.seriesStore.GetSeriesForWork(r.Context(), workID)
	seriesItems := []model.SeriesEntry{}
	if series != nil {
		entries, err := h.seriesStore.GetSeriesEntries(r.Context(), series.ID)
		if err != nil {
			writeError(w, "failed to load work graph", http.StatusInternalServerError)
			return
		}
		seriesItems = entries
	}

	subjects, err := h.subjectStore.GetSubjectsForWork(r.Context(), workID)
	if err != nil {
		writeError(w, "failed to load work graph", http.StatusInternalServerError)
		return
	}

	relationships, err := h.workRelStore.GetRelatedWorks(r.Context(), workID, nil, 50)
	if err != nil {
		writeError(w, "failed to load work graph", http.StatusInternalServerError)
		return
	}
	related := make([]RelatedWork, 0, len(relationships))
	for _, rel := range relationships {
		target, targetErr := h.workStore.GetWorkByID(r.Context(), rel.TargetWorkID)
		if targetErr != nil {
			continue
		}
		related = append(related, RelatedWork{
			RelationshipType: rel.RelationshipType,
			Confidence:       rel.Confidence,
			Provider:         rel.Provider,
			Work:             *target,
		})
	}

	writeJSON(w, WorkGraphResponse{
		WorkID:      workID,
		Series:      series,
		SeriesItems: seriesItems,
		Subjects:    subjects,
		Related:     related,
	})
}

// GetSeries handles GET /v1/series/{id}.
func (h *Handlers) GetSeries(w http.ResponseWriter, r *http.Request) {
	if h.seriesStore == nil {
		writeError(w, "series store not configured", http.StatusServiceUnavailable)
		return
	}
	id := mux.Vars(r)["id"]
	if strings.TrimSpace(id) == "" {
		writeError(w, "missing series id", http.StatusBadRequest)
		return
	}

	series, err := h.seriesStore.GetSeriesByID(r.Context(), id)
	if err != nil {
		writeError(w, "series not found", http.StatusNotFound)
		return
	}
	entries, err := h.seriesStore.GetSeriesEntries(r.Context(), id)
	if err != nil {
		writeError(w, "failed to load series entries", http.StatusInternalServerError)
		return
	}
	works := make([]model.Work, 0, len(entries))
	if h.workStore != nil {
		for _, entry := range entries {
			work, workErr := h.workStore.GetWorkByID(r.Context(), entry.WorkID)
			if workErr != nil {
				continue
			}
			works = append(works, *work)
		}
	}

	writeJSON(w, SeriesResponse{Series: *series, Entries: entries, Works: works})
}

// GetSubjectWorks handles GET /v1/subjects/{id}/works.
func (h *Handlers) GetSubjectWorks(w http.ResponseWriter, r *http.Request) {
	if h.subjectStore == nil {
		writeError(w, "subject store not configured", http.StatusServiceUnavailable)
		return
	}
	id := mux.Vars(r)["id"]
	if strings.TrimSpace(id) == "" {
		writeError(w, "missing subject id", http.StatusBadRequest)
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

	subject, err := h.subjectStore.GetSubjectByID(r.Context(), id)
	if err != nil {
		writeError(w, "subject not found", http.StatusNotFound)
		return
	}
	works, err := h.subjectStore.GetWorksForSubject(r.Context(), id, limit, 0)
	if err != nil {
		writeError(w, "failed to load subject works", http.StatusInternalServerError)
		return
	}

	writeJSON(w, SubjectWorksResponse{Subject: *subject, Works: works})
}

// GetGraphStats handles GET /v1/graph/stats.
func (h *Handlers) GetGraphStats(w http.ResponseWriter, r *http.Request) {
	if h.seriesStore == nil || h.subjectStore == nil || h.workRelStore == nil {
		writeError(w, "graph stores not configured", http.StatusServiceUnavailable)
		return
	}

	seriesCount, err := h.seriesStore.CountSeries(r.Context())
	if err != nil {
		writeError(w, "failed to read graph stats", http.StatusInternalServerError)
		return
	}
	subjectCount, err := h.subjectStore.CountSubjects(r.Context())
	if err != nil {
		writeError(w, "failed to read graph stats", http.StatusInternalServerError)
		return
	}
	relByType, err := h.workRelStore.CountRelationshipsByType(r.Context())
	if err != nil {
		writeError(w, "failed to read graph stats", http.StatusInternalServerError)
		return
	}

	writeJSON(w, GraphStatsResponse{
		SeriesCount:             seriesCount,
		SubjectsCount:           subjectCount,
		RelationshipCountByType: relByType,
	})
}

func (h *Handlers) GetWorkRecommendations(w http.ResponseWriter, r *http.Request) {
	if h.recommender == nil {
		writeError(w, "recommendation engine not configured", http.StatusServiceUnavailable)
		return
	}
	workID := strings.TrimSpace(mux.Vars(r)["id"])
	if workID == "" {
		writeError(w, "missing work id", http.StatusBadRequest)
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, "invalid limit", http.StatusBadRequest)
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}

	req := recommend.RecommendationRequest{
		SeedWorkIDs:  []string{workID},
		Limit:        limit,
		IncludeTypes: splitCSVParam(r.URL.Query().Get("include")),
		Preferences: recommend.RecommendationPreferences{
			Formats:   splitCSVParam(r.URL.Query().Get("formats")),
			Languages: splitCSVParam(r.URL.Query().Get("languages")),
		},
	}

	results, err := h.recommender.Recommend(r.Context(), req)
	if err != nil {
		log.Error().Err(err).Str("work_id", workID).Msg("work recommendations failed")
		writeError(w, "failed to compute recommendations", http.StatusInternalServerError)
		return
	}

	writeJSON(w, RecommendationsResponse{SeedWorkID: workID, Recommendations: results})
}

func (h *Handlers) GetNextInSeries(w http.ResponseWriter, r *http.Request) {
	if h.recommender == nil {
		writeError(w, "recommendation engine not configured", http.StatusServiceUnavailable)
		return
	}
	workID := strings.TrimSpace(mux.Vars(r)["id"])
	if workID == "" {
		writeError(w, "missing work id", http.StatusBadRequest)
		return
	}

	next, err := h.recommender.RecommendNextInSeries(r.Context(), workID)
	if err != nil {
		log.Error().Err(err).Str("work_id", workID).Msg("next-in-series lookup failed")
		writeError(w, "failed to compute next in series", http.StatusInternalServerError)
		return
	}
	if next == nil {
		writeError(w, "next in series not found", http.StatusNotFound)
		return
	}

	writeJSON(w, NextRecommendationResponse{SeedWorkID: workID, Next: next})
}

func (h *Handlers) GetSimilarWorks(w http.ResponseWriter, r *http.Request) {
	if h.recommender == nil {
		writeError(w, "recommendation engine not configured", http.StatusServiceUnavailable)
		return
	}
	workID := strings.TrimSpace(mux.Vars(r)["id"])
	if workID == "" {
		writeError(w, "missing work id", http.StatusBadRequest)
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, "invalid limit", http.StatusBadRequest)
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}

	results, err := h.recommender.RecommendSimilar(r.Context(), workID, limit, recommend.RecommendationPreferences{
		Formats:   splitCSVParam(r.URL.Query().Get("formats")),
		Languages: splitCSVParam(r.URL.Query().Get("languages")),
	})
	if err != nil {
		log.Error().Err(err).Str("work_id", workID).Msg("similar works lookup failed")
		writeError(w, "failed to compute similar works", http.StatusInternalServerError)
		return
	}

	writeJSON(w, RecommendationsResponse{SeedWorkID: workID, Recommendations: results})
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
	quarantineDisableDispatch := false
	if h.registry != nil {
		quarantineDisableDispatch = h.registry.QuarantineDisables()
	}

	writeJSON(w, ProviderPolicyResponse{
		QuarantineDisableDispatch: quarantineDisableDispatch,
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

// --- Metadata quality endpoints ---

// GetQualityReport handles GET /v1/quality/report.
func (h *Handlers) GetQualityReport(w http.ResponseWriter, r *http.Request) {
	if h.qualityEngine == nil {
		writeError(w, "quality engine not configured", http.StatusServiceUnavailable)
		return
	}

	limit := 25
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
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

	report, err := h.qualityEngine.Audit(r.Context(), limit)
	if err != nil {
		log.Error().Err(err).Msg("quality report generation failed")
		writeError(w, "failed to generate quality report", http.StatusInternalServerError)
		return
	}

	writeJSON(w, QualityReportResponse{Report: *report})
}

// RepairQualityIssues handles POST /v1/quality/repair.
func (h *Handlers) RepairQualityIssues(w http.ResponseWriter, r *http.Request) {
	if h.qualityEngine == nil {
		writeError(w, "quality engine not configured", http.StatusServiceUnavailable)
		return
	}

	var req RepairQualityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}

	removeInvalid := true
	if req.RemoveInvalidIdentifiers != nil {
		removeInvalid = *req.RemoveInvalidIdentifiers
	}

	result, err := h.qualityEngine.Repair(r.Context(), quality.RepairRequest{
		Limit:                    limit,
		DryRun:                   req.DryRun,
		RemoveInvalidIdentifiers: removeInvalid,
	})
	if err != nil {
		log.Error().Err(err).Msg("metadata quality repair failed")
		writeError(w, "failed to apply quality repairs", http.StatusInternalServerError)
		return
	}

	writeJSON(w, QualityRepairResponse{Result: *result})
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
