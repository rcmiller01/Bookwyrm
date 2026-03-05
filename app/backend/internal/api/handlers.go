package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"app-backend/internal/domain"
	"app-backend/internal/integration/indexer"
	"app-backend/internal/integration/metadata"
	"app-backend/internal/store"

	"github.com/gorilla/mux"
)

type Handlers struct {
	metaClient     *metadata.Client
	indexerClient  *indexer.Client
	watchlistStore store.WatchlistStore
}

func NewHandlers(metaClient *metadata.Client, indexerClient *indexer.Client, watchlistStore store.WatchlistStore) *Handlers {
	return &Handlers{metaClient: metaClient, indexerClient: indexerClient, watchlistStore: watchlistStore}
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

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
