package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"indexer-service/internal/indexer"
	"indexer-service/internal/mcp"

	"github.com/gorilla/mux"
)

type Handlers struct {
	service      *indexer.Service
	store        indexer.Storage
	orchestrator *indexer.Orchestrator
	mcpRegistry  *mcp.Registry
	mcpRuntime   *mcp.Runtime
}

func NewHandlers(service *indexer.Service, store indexer.Storage, orchestrator *indexer.Orchestrator, mcpRegistry *mcp.Registry, mcpRuntime *mcp.Runtime) *Handlers {
	return &Handlers{
		service:      service,
		store:        store,
		orchestrator: orchestrator,
		mcpRegistry:  mcpRegistry,
		mcpRuntime:   mcpRuntime,
	}
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"status": "ok"})
}

func (h *Handlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"providers": h.service.ListProviders(r.Context())})
}

func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	var req indexer.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Metadata.WorkID) == "" && strings.TrimSpace(req.Metadata.EntityID) == "" {
		writeError(w, "metadata.work_id or metadata.entity_id is required", http.StatusBadRequest)
		return
	}
	result, err := h.service.Search(r.Context(), req)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, result)
}

func (h *Handlers) EnqueueWorkSearch(w http.ResponseWriter, r *http.Request) {
	workID := strings.TrimSpace(mux.Vars(r)["workID"])
	if workID == "" {
		writeError(w, "missing work id", http.StatusBadRequest)
		return
	}
	var body struct {
		Title      string   `json:"title"`
		Author     string   `json:"author"`
		ISBN       string   `json:"isbn"`
		DOI        string   `json:"doi"`
		Formats    []string `json:"formats"`
		Languages  []string `json:"languages"`
		Limit      int      `json:"limit"`
		TimeoutSec int      `json:"timeout_sec"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	spec := indexer.QuerySpec{
		EntityType: "work",
		EntityID:   workID,
		Title:      body.Title,
		Author:     body.Author,
		ISBN:       body.ISBN,
		DOI:        body.DOI,
	}
	spec.Preferences.Formats = body.Formats
	spec.Preferences.Languages = body.Languages
	spec.Limits.MaxCandidates = body.Limit
	spec.Limits.TimeoutSec = body.TimeoutSec
	if spec.Title == "" {
		spec.Title = workID
	}
	req := h.orchestrator.Enqueue(spec)
	writeJSON(w, map[string]any{
		"search_request_id": req.ID,
		"status":            req.Status,
	})
}

func (h *Handlers) BulkSearch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Items []struct {
			EntityType string `json:"entity_type"`
			EntityID   string `json:"entity_id"`
			Title      string `json:"title"`
			Author     string `json:"author"`
			ISBN       string `json:"isbn"`
			DOI        string `json:"doi"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	requests := make([]map[string]any, 0, len(body.Items))
	for _, item := range body.Items {
		entityType := strings.TrimSpace(item.EntityType)
		if entityType == "" {
			entityType = "work"
		}
		entityID := strings.TrimSpace(item.EntityID)
		if entityID == "" {
			continue
		}
		spec := indexer.QuerySpec{
			EntityType: entityType,
			EntityID:   entityID,
			Title:      strings.TrimSpace(item.Title),
			Author:     strings.TrimSpace(item.Author),
			ISBN:       strings.TrimSpace(item.ISBN),
			DOI:        strings.TrimSpace(item.DOI),
		}
		req := h.orchestrator.Enqueue(spec)
		requests = append(requests, map[string]any{
			"search_request_id": req.ID,
			"status":            req.Status,
			"entity_type":       entityType,
			"entity_id":         entityID,
		})
	}
	writeJSON(w, map[string]any{"items": requests})
}

func (h *Handlers) SetWantedWork(w http.ResponseWriter, r *http.Request) {
	workID := strings.TrimSpace(mux.Vars(r)["workID"])
	if workID == "" {
		writeError(w, "missing work id", http.StatusBadRequest)
		return
	}
	var body struct {
		Enabled        bool     `json:"enabled"`
		Priority       int      `json:"priority"`
		CadenceMinutes int      `json:"cadence_minutes"`
		ProfileID      string   `json:"profile_id"`
		IgnoreUpgrades bool     `json:"ignore_upgrades"`
		Formats        []string `json:"formats"`
		Languages      []string `json:"languages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.CadenceMinutes <= 0 {
		body.CadenceMinutes = 60
	}
	rec, err := h.store.SetWantedWork(indexer.WantedWorkRecord{
		WorkID:         workID,
		Enabled:        body.Enabled,
		Priority:       body.Priority,
		CadenceMinutes: body.CadenceMinutes,
		ProfileID:      strings.TrimSpace(body.ProfileID),
		IgnoreUpgrades: body.IgnoreUpgrades,
		Formats:        body.Formats,
		Languages:      body.Languages,
	})
	if err != nil {
		writeError(w, "failed to set wanted work", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"item": rec})
}

func (h *Handlers) ListWantedWorks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"items": h.store.ListWantedWorks()})
}

func (h *Handlers) DeleteWantedWork(w http.ResponseWriter, r *http.Request) {
	workID := strings.TrimSpace(mux.Vars(r)["workID"])
	if workID == "" {
		writeError(w, "missing work id", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteWantedWork(workID); err != nil {
		writeError(w, "wanted work not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) SetWantedAuthor(w http.ResponseWriter, r *http.Request) {
	authorID := strings.TrimSpace(mux.Vars(r)["authorID"])
	if authorID == "" {
		writeError(w, "missing author id", http.StatusBadRequest)
		return
	}
	var body struct {
		Enabled        bool     `json:"enabled"`
		Priority       int      `json:"priority"`
		CadenceMinutes int      `json:"cadence_minutes"`
		ProfileID      string   `json:"profile_id"`
		Formats        []string `json:"formats"`
		Languages      []string `json:"languages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.CadenceMinutes <= 0 {
		body.CadenceMinutes = 60
	}
	rec, err := h.store.SetWantedAuthor(indexer.WantedAuthorRecord{
		AuthorID:       authorID,
		Enabled:        body.Enabled,
		Priority:       body.Priority,
		CadenceMinutes: body.CadenceMinutes,
		ProfileID:      strings.TrimSpace(body.ProfileID),
		Formats:        body.Formats,
		Languages:      body.Languages,
	})
	if err != nil {
		writeError(w, "failed to set wanted author", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"item": rec})
}

func (h *Handlers) ListWantedAuthors(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"items": h.store.ListWantedAuthors()})
}

func (h *Handlers) DeleteWantedAuthor(w http.ResponseWriter, r *http.Request) {
	authorID := strings.TrimSpace(mux.Vars(r)["authorID"])
	if authorID == "" {
		writeError(w, "missing author id", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteWantedAuthor(authorID); err != nil {
		writeError(w, "wanted author not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) ListProfiles(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"items":              h.store.ListProfiles(),
		"default_profile_id": h.store.GetDefaultProfileID(),
	})
}

func (h *Handlers) UpsertProfile(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	var body struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		CutoffQuality  string `json:"cutoff_quality"`
		DefaultProfile bool   `json:"default_profile"`
		Qualities      []struct {
			Quality string `json:"quality"`
			Rank    int    `json:"rank"`
		} `json:"qualities"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if id == "" {
		id = strings.TrimSpace(body.ID)
	}
	if id == "" {
		writeError(w, "profile id is required", http.StatusBadRequest)
		return
	}
	profile := indexer.ProfileRecord{
		ID:             id,
		Name:           strings.TrimSpace(body.Name),
		CutoffQuality:  strings.TrimSpace(body.CutoffQuality),
		DefaultProfile: body.DefaultProfile,
	}
	qualities := make([]indexer.ProfileQualityRecord, 0, len(body.Qualities))
	for i, q := range body.Qualities {
		quality := strings.TrimSpace(q.Quality)
		if quality == "" {
			continue
		}
		rank := q.Rank
		if rank <= 0 {
			rank = i + 1
		}
		qualities = append(qualities, indexer.ProfileQualityRecord{
			ProfileID: id,
			Quality:   quality,
			Rank:      rank,
		})
	}
	result, err := h.store.UpsertProfile(profile, qualities)
	if err != nil {
		writeError(w, "failed to upsert profile", http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (h *Handlers) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	if id == "" {
		writeError(w, "missing profile id", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteProfile(id); err != nil {
		if err == indexer.ErrNotFound {
			writeError(w, "profile not found", http.StatusNotFound)
			return
		}
		writeError(w, err.Error(), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) GetSearchRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["requestID"]), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "invalid request id", http.StatusBadRequest)
		return
	}
	rec, err := h.store.GetSearchRequest(id)
	if err != nil {
		writeError(w, "search request not found", http.StatusNotFound)
		return
	}
	candidates, _ := h.store.ListCandidates(id, 10)
	writeJSON(w, map[string]any{
		"request":        rec,
		"top_candidates": candidates,
	})
}

func (h *Handlers) ListCandidates(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["requestID"]), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, "invalid request id", http.StatusBadRequest)
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}
	candidates, err := h.store.ListCandidates(id, limit)
	if err != nil {
		writeError(w, "candidates not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"items": candidates})
}

func (h *Handlers) GrabCandidate(w http.ResponseWriter, r *http.Request) {
	candidateID, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["candidateID"]), 10, 64)
	if err != nil || candidateID <= 0 {
		writeError(w, "invalid candidate id", http.StatusBadRequest)
		return
	}
	candidate, err := h.store.GetCandidateByID(candidateID)
	if err != nil {
		writeError(w, "candidate not found", http.StatusNotFound)
		return
	}
	rec, err := h.store.GetSearchRequest(candidate.SearchRequestID)
	if err != nil {
		writeError(w, "candidate search request missing", http.StatusNotFound)
		return
	}
	grab, err := h.store.CreateGrab(candidateID, rec.EntityType, rec.EntityID)
	if err != nil {
		writeError(w, "failed to create grab", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"grab": grab})
}

func (h *Handlers) GetCandidateByID(w http.ResponseWriter, r *http.Request) {
	candidateID, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["candidateID"]), 10, 64)
	if err != nil || candidateID <= 0 {
		writeError(w, "invalid candidate id", http.StatusBadRequest)
		return
	}
	candidate, err := h.store.GetCandidateByID(candidateID)
	if err != nil {
		writeError(w, "candidate not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"candidate": candidate})
}

func (h *Handlers) GetGrabByID(w http.ResponseWriter, r *http.Request) {
	grabID, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["grabID"]), 10, 64)
	if err != nil || grabID <= 0 {
		writeError(w, "invalid grab id", http.StatusBadRequest)
		return
	}
	grab, err := h.store.GetGrabByID(grabID)
	if err != nil {
		writeError(w, "grab not found", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{"grab": grab})
}

func (h *Handlers) ListBackends(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"backends": h.store.ListBackends()})
}

func (h *Handlers) EnableBackend(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	if err := h.store.SetBackendEnabled(id, true); err != nil {
		writeError(w, "backend not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) DisableBackend(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	if err := h.store.SetBackendEnabled(id, false); err != nil {
		writeError(w, "backend not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) SetBackendPriority(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	var body struct {
		Priority int `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.store.SetBackendPriority(id, body.Priority); err != nil {
		writeError(w, "backend not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) SetBackendPreferred(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	var body struct {
		Preferred bool `json:"preferred"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.store.SetBackendPreferred(id, body.Preferred); err != nil {
		writeError(w, "backend not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) ListBackendReliability(w http.ResponseWriter, _ *http.Request) {
	backends := h.store.ListBackends()
	writeJSON(w, map[string]any{"backends": backends})
}

func (h *Handlers) ListMCPServers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"servers": h.mcpRegistry.ListServers()})
}

func (h *Handlers) CreateMCPServer(w http.ResponseWriter, r *http.Request) {
	var rec indexer.MCPServerRecord
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(rec.ID) == "" || strings.TrimSpace(rec.Name) == "" || strings.TrimSpace(rec.Source) == "" || strings.TrimSpace(rec.SourceRef) == "" {
		writeError(w, "id, name, source, and source_ref are required", http.StatusBadRequest)
		return
	}
	if rec.EnvSchema == nil {
		rec.EnvSchema = map[string]string{}
	}
	if rec.EnvMapping == nil {
		rec.EnvMapping = map[string]string{}
	}
	rec.Enabled = true
	rec = h.mcpRegistry.UpsertServer(rec)
	backendID := "mcp:" + rec.ID
	h.store.UpsertBackend(indexer.BackendRecord{
		ID:               backendID,
		Name:             rec.Name,
		BackendType:      indexer.BackendTypeMCP,
		Enabled:          true,
		Tier:             indexer.TierUnclassified,
		ReliabilityScore: 0.70,
		Priority:         200,
		Config:           map[string]any{"server_id": rec.ID},
	})
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, rec)
}

func (h *Handlers) EnableMCPServer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	if err := h.mcpRegistry.SetEnabled(id, true); err != nil {
		writeError(w, "mcp server not found", http.StatusNotFound)
		return
	}
	_ = h.store.SetBackendEnabled("mcp:"+id, true)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) DisableMCPServer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	if err := h.mcpRegistry.SetEnabled(id, false); err != nil {
		writeError(w, "mcp server not found", http.StatusNotFound)
		return
	}
	_ = h.store.SetBackendEnabled("mcp:"+id, false)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) SetMCPEnvMapping(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.mcpRegistry.SetEnvMapping(id, body); err != nil {
		writeError(w, "mcp server not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) TestMCPServer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(mux.Vars(r)["id"])
	server, err := h.mcpRegistry.Get(id)
	if err != nil {
		writeError(w, "mcp server not found", http.StatusNotFound)
		return
	}
	client := mcp.NewClient(server.BaseURL, 8*time.Second)
	headers := h.mcpRuntime.HeadersFor(server)
	if err := client.Health(r.Context(), headers); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "server_id": id})
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
