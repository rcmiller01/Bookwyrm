package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(h *Handlers) http.Handler {
	r := mux.NewRouter()

	v1 := r.PathPrefix("/v1").Subrouter()

	// Metadata endpoints
	v1.HandleFunc("/search", h.Search).Methods(http.MethodGet)
	v1.HandleFunc("/resolve", h.Resolve).Methods(http.MethodGet)
	v1.HandleFunc("/work/{id}", h.GetWork).Methods(http.MethodGet)
	v1.HandleFunc("/work/{id}/recommendations", h.GetWorkRecommendations).Methods(http.MethodGet)
	v1.HandleFunc("/work/{id}/next", h.GetNextInSeries).Methods(http.MethodGet)
	v1.HandleFunc("/work/{id}/similar", h.GetSimilarWorks).Methods(http.MethodGet)
	v1.HandleFunc("/work/{id}/graph", h.GetWorkGraph).Methods(http.MethodGet)
	v1.HandleFunc("/series/{id}", h.GetSeries).Methods(http.MethodGet)
	v1.HandleFunc("/subjects/{id}/works", h.GetSubjectWorks).Methods(http.MethodGet)
	v1.HandleFunc("/graph/stats", h.GetGraphStats).Methods(http.MethodGet)

	// Provider management endpoints
	v1.HandleFunc("/providers", h.ListProviders).Methods(http.MethodGet)
	v1.HandleFunc("/providers/policy", h.GetProviderPolicy).Methods(http.MethodGet)
	v1.HandleFunc("/providers/reliability", h.ListReliabilityScores).Methods(http.MethodGet)
	v1.HandleFunc("/providers/{name}", h.UpsertProvider).Methods(http.MethodPost, http.MethodPut)
	v1.HandleFunc("/providers/{name}/test", h.TestProvider).Methods(http.MethodPost)
	v1.HandleFunc("/providers/{name}/reliability", h.GetProviderReliability).Methods(http.MethodGet)

	// Enrichment endpoints
	v1.HandleFunc("/enrichment/jobs", h.ListEnrichmentJobs).Methods(http.MethodGet)
	v1.HandleFunc("/enrichment/jobs", h.EnqueueEnrichmentJob).Methods(http.MethodPost)
	v1.HandleFunc("/enrichment/jobs/{id}", h.GetEnrichmentJob).Methods(http.MethodGet)
	v1.HandleFunc("/enrichment/stats", h.GetEnrichmentStats).Methods(http.MethodGet)

	// Quality endpoints
	v1.HandleFunc("/quality/report", h.GetQualityReport).Methods(http.MethodGet)
	v1.HandleFunc("/quality/repair", h.RepairQualityIssues).Methods(http.MethodPost)

	// Observability
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}).Methods(http.MethodGet)

	return r
}
