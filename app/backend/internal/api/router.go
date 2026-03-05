package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

func NewRouter(h *Handlers) http.Handler {
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/health", h.Health).Methods(http.MethodGet)
	api.HandleFunc("/search", h.Search).Methods(http.MethodGet)
	api.HandleFunc("/works/{id}/intelligence", h.GetWorkIntelligence).Methods(http.MethodGet)
	api.HandleFunc("/works/{id}/availability", h.GetAvailability).Methods(http.MethodGet)
	api.HandleFunc("/quality/report", h.GetQualityReport).Methods(http.MethodGet)
	api.HandleFunc("/quality/repair", h.RepairQuality).Methods(http.MethodPost)
	api.HandleFunc("/watchlists", h.ListWatchlist).Methods(http.MethodGet)
	api.HandleFunc("/watchlists", h.CreateWatchlist).Methods(http.MethodPost)
	api.HandleFunc("/watchlists/{id}", h.DeleteWatchlist).Methods(http.MethodDelete)
	return r
}
