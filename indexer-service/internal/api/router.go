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
	v1.HandleFunc("/search/work/{workID}", h.EnqueueWorkSearch).Methods(http.MethodPost)
	v1.HandleFunc("/wanted/works", h.ListWantedWorks).Methods(http.MethodGet)
	v1.HandleFunc("/wanted/works/{workID}", h.SetWantedWork).Methods(http.MethodPost, http.MethodPut)
	v1.HandleFunc("/wanted/works/{workID}", h.DeleteWantedWork).Methods(http.MethodDelete)
	v1.HandleFunc("/wanted/authors", h.ListWantedAuthors).Methods(http.MethodGet)
	v1.HandleFunc("/wanted/authors/{authorID}", h.SetWantedAuthor).Methods(http.MethodPost, http.MethodPut)
	v1.HandleFunc("/wanted/authors/{authorID}", h.DeleteWantedAuthor).Methods(http.MethodDelete)
	v1.HandleFunc("/search/{requestID}", h.GetSearchRequest).Methods(http.MethodGet)
	v1.HandleFunc("/candidates/{requestID}", h.ListCandidates).Methods(http.MethodGet)
	v1.HandleFunc("/candidates/id/{candidateID}", h.GetCandidateByID).Methods(http.MethodGet)
	v1.HandleFunc("/grab/{candidateID}", h.GrabCandidate).Methods(http.MethodPost)
	v1.HandleFunc("/grabs/{grabID}", h.GetGrabByID).Methods(http.MethodGet)
	v1.HandleFunc("/backends", h.ListBackends).Methods(http.MethodGet)
	v1.HandleFunc("/backends/{id}/enable", h.EnableBackend).Methods(http.MethodPost)
	v1.HandleFunc("/backends/{id}/disable", h.DisableBackend).Methods(http.MethodPost)
	v1.HandleFunc("/backends/{id}/priority", h.SetBackendPriority).Methods(http.MethodPost)
	v1.HandleFunc("/backends/reliability", h.ListBackendReliability).Methods(http.MethodGet)

	mcp := r.PathPrefix("/v1/mcp").Subrouter()
	mcp.HandleFunc("/servers", h.ListMCPServers).Methods(http.MethodGet)
	mcp.HandleFunc("/servers", h.CreateMCPServer).Methods(http.MethodPost)
	mcp.HandleFunc("/servers/{id}/enable", h.EnableMCPServer).Methods(http.MethodPost)
	mcp.HandleFunc("/servers/{id}/disable", h.DisableMCPServer).Methods(http.MethodPost)
	mcp.HandleFunc("/servers/{id}/env", h.SetMCPEnvMapping).Methods(http.MethodPost)
	mcp.HandleFunc("/servers/{id}/test", h.TestMCPServer).Methods(http.MethodPost)
	return r
}
