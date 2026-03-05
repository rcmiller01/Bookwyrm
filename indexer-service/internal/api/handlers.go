package api

import (
	"encoding/json"
	"net/http"

	"indexer-service/internal/indexer"
)

type Handlers struct {
	service *indexer.Service
}

func NewHandlers(service *indexer.Service) *Handlers {
	return &Handlers{service: service}
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
	if req.Metadata.WorkID == "" {
		writeError(w, "metadata.work_id is required", http.StatusBadRequest)
		return
	}
	result, err := h.service.Search(r.Context(), req)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, result)
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
