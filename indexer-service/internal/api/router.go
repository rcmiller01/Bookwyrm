package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

func NewRouter(h *Handlers) http.Handler {
	r := mux.NewRouter()
	v1 := r.PathPrefix("/v1/indexer").Subrouter()
	v1.HandleFunc("/health", h.Health).Methods(http.MethodGet)
	v1.HandleFunc("/providers", h.ListProviders).Methods(http.MethodGet)
	v1.HandleFunc("/search", h.Search).Methods(http.MethodPost)
	return r
}
