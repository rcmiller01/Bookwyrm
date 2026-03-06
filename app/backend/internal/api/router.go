package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(h *Handlers) http.Handler {
	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler()).Methods(http.MethodGet)
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
	api.HandleFunc("/jobs", h.ListJobs).Methods(http.MethodGet)
	api.HandleFunc("/jobs", h.EnqueueJob).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}", h.GetJob).Methods(http.MethodGet)
	api.HandleFunc("/jobs/{id}/retry", h.RetryJob).Methods(http.MethodPost)
	api.HandleFunc("/jobs/{id}/cancel", h.CancelJob).Methods(http.MethodPost)
	api.HandleFunc("/download/jobs", h.ListDownloadJobs).Methods(http.MethodGet)
	api.HandleFunc("/download/jobs/{id}", h.GetDownloadJob).Methods(http.MethodGet)
	api.HandleFunc("/download/from-grab/{grabID}", h.CreateDownloadFromGrab).Methods(http.MethodPost)
	api.HandleFunc("/download/jobs/{id}/cancel", h.CancelDownloadJob).Methods(http.MethodPost)
	api.HandleFunc("/download/jobs/{id}/retry", h.RetryDownloadJob).Methods(http.MethodPost)
	api.HandleFunc("/import/jobs", h.ListImportJobs).Methods(http.MethodGet)
	api.HandleFunc("/import/jobs/{id}", h.GetImportJob).Methods(http.MethodGet)
	api.HandleFunc("/import/jobs/{id}/approve", h.ApproveImportJob).Methods(http.MethodPost)
	api.HandleFunc("/import/jobs/{id}/retry", h.RetryImportJob).Methods(http.MethodPost)
	api.HandleFunc("/import/jobs/{id}/skip", h.SkipImportJob).Methods(http.MethodPost)
	api.HandleFunc("/import/stats", h.GetImportStats).Methods(http.MethodGet)
	api.HandleFunc("/library/items", h.ListLibraryItems).Methods(http.MethodGet)
	return r
}
