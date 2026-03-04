package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"metadata-service/internal/model"
	"metadata-service/internal/provider"
	"metadata-service/internal/resolver"
	"metadata-service/internal/store"
)

type Handlers struct {
	resolver    resolver.Resolver
	registry    *provider.Registry
	rateLimiter *provider.RateLimiter
	cfgStore    store.ProviderConfigStore
	statusStore store.ProviderStatusStore
}

func NewHandlers(res resolver.Resolver, registry *provider.Registry, rl *provider.RateLimiter, cfgStore store.ProviderConfigStore, statusStore store.ProviderStatusStore) *Handlers {
	return &Handlers{
		resolver:    res,
		registry:    registry,
		rateLimiter: rl,
		cfgStore:    cfgStore,
		statusStore: statusStore,
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
