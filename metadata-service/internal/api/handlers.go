package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"metadata-service/internal/model"
	"metadata-service/internal/resolver"
)

type Handlers struct {
	resolver resolver.Resolver
}

func NewHandlers(res resolver.Resolver) *Handlers {
	return &Handlers{resolver: res}
}

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

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}
