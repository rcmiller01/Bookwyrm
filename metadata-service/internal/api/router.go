package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(h *Handlers) http.Handler {
	r := mux.NewRouter()

	v1 := r.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/search", h.Search).Methods(http.MethodGet)
	v1.HandleFunc("/resolve", h.Resolve).Methods(http.MethodGet)
	v1.HandleFunc("/work/{id}", h.GetWork).Methods(http.MethodGet)

	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}).Methods(http.MethodGet)

	return r
}
