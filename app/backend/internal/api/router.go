package api

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const proxyTimeout = 15 * time.Second

type RouterConfig struct {
	UIAssetsDir          string
	MetadataProxyBaseURL string
	IndexerProxyBaseURL  string
}

func NewRouter(h *Handlers) http.Handler {
	return NewRouterWithConfig(h, RouterConfig{})
}

func NewRouterWithConfig(h *Handlers, cfg RouterConfig) http.Handler {
	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler()).Methods(http.MethodGet)
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/health", h.Health).Methods(http.MethodGet)
	api.HandleFunc("/healthz", h.Healthz).Methods(http.MethodGet)
	api.HandleFunc("/readyz", h.Readyz).Methods(http.MethodGet)
	api.HandleFunc("/system/status", h.SystemStatus).Methods(http.MethodGet)
	api.HandleFunc("/system/dependencies", h.SystemDependencies).Methods(http.MethodGet)
	api.HandleFunc("/system/migration-status", h.SystemMigrationStatus).Methods(http.MethodGet)
	api.HandleFunc("/system/health-detail", h.HealthDetail).Methods(http.MethodGet)
	api.HandleFunc("/system/readiness", h.SystemReadiness).Methods(http.MethodGet)
	api.HandleFunc("/system/support-bundle", h.SupportBundle).Methods(http.MethodGet)
	api.HandleFunc("/system/actions/retry-failed-downloads", h.ActionRetryFailedDownloads).Methods(http.MethodPost)
	api.HandleFunc("/system/actions/retry-failed-imports", h.ActionRetryFailedImports).Methods(http.MethodPost)
	api.HandleFunc("/system/actions/test-connections", h.ActionTestConnections).Methods(http.MethodPost)
	api.HandleFunc("/system/actions/run-cleanup", h.ActionRunCleanup).Methods(http.MethodPost)
	api.HandleFunc("/system/actions/recompute-reliability", h.ActionRecomputeReliability).Methods(http.MethodPost)
	api.HandleFunc("/system/actions/rerun-wanted-searches", h.ActionRerunWantedSearches).Methods(http.MethodPost)
	api.HandleFunc("/system/actions/rerun-enrichment", h.ActionRerunEnrichment).Methods(http.MethodPost)
	api.HandleFunc("/search", h.Search).Methods(http.MethodGet)
	api.HandleFunc("/works/{id}/intelligence", h.GetWorkIntelligence).Methods(http.MethodGet)
	api.HandleFunc("/works/{id}/availability", h.GetAvailability).Methods(http.MethodGet)
	api.HandleFunc("/works/{id}/timeline", h.GetWorkTimeline).Methods(http.MethodGet)
	api.HandleFunc("/work/{id}/timeline", h.GetWorkTimeline).Methods(http.MethodGet)
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
	api.HandleFunc("/download/clients", h.ListDownloadClients).Methods(http.MethodGet)
	api.HandleFunc("/download/clients/{id}", h.UpdateDownloadClient).Methods(http.MethodPatch)
	api.HandleFunc("/download/jobs/{id}", h.GetDownloadJob).Methods(http.MethodGet)
	api.HandleFunc("/download/from-grab/{grabID}", h.CreateDownloadFromGrab).Methods(http.MethodPost)
	api.HandleFunc("/download/jobs/{id}/cancel", h.CancelDownloadJob).Methods(http.MethodPost)
	api.HandleFunc("/download/jobs/{id}/retry", h.RetryDownloadJob).Methods(http.MethodPost)
	api.HandleFunc("/import/jobs", h.ListImportJobs).Methods(http.MethodGet)
	api.HandleFunc("/import/jobs/{id}", h.GetImportJob).Methods(http.MethodGet)
	api.HandleFunc("/import/jobs/{id}/decide", h.DecideImportJob).Methods(http.MethodPost)
	api.HandleFunc("/import/jobs/{id}/approve", h.ApproveImportJob).Methods(http.MethodPost)
	api.HandleFunc("/import/jobs/{id}/retry", h.RetryImportJob).Methods(http.MethodPost)
	api.HandleFunc("/import/jobs/{id}/skip", h.SkipImportJob).Methods(http.MethodPost)
	api.HandleFunc("/import/stats", h.GetImportStats).Methods(http.MethodGet)
	api.HandleFunc("/library/items", h.ListLibraryItems).Methods(http.MethodGet)
	api.HandleFunc("/test-connection/download-client/{id}", h.TestDownloadClient).Methods(http.MethodPost)

	if metadataProxy := newPathProxy(cfg.MetadataProxyBaseURL, "/ui-api/metadata", "/v1"); metadataProxy != nil {
		r.Handle("/ui-api/metadata", metadataProxy)
		r.PathPrefix("/ui-api/metadata/").Handler(metadataProxy)
	}
	if indexerProxy := newPathProxy(cfg.IndexerProxyBaseURL, "/ui-api/indexer", "/v1/indexer"); indexerProxy != nil {
		r.Handle("/ui-api/indexer", indexerProxy)
		r.PathPrefix("/ui-api/indexer/").Handler(indexerProxy)
	}

	if strings.TrimSpace(cfg.UIAssetsDir) != "" {
		fs := http.FileServer(http.Dir(cfg.UIAssetsDir))
		r.PathPrefix("/assets/").Handler(fs)
		r.PathPrefix("/").Handler(spaFallbackHandler(cfg.UIAssetsDir))
	}

	return HTTPMetrics("backend", SecurityHeaders(r))
}

func newPathProxy(baseURL string, stripPrefix string, upstreamPrefix string) http.Handler {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil
	}
	target, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		ResponseHeaderTimeout: proxyTimeout,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   10,
	}

	serviceName := strings.Trim(stripPrefix, "/")
	envVarHint := ""
	if strings.Contains(serviceName, "metadata") {
		envVarHint = "METADATA_SERVICE_URL"
	} else if strings.Contains(serviceName, "indexer") {
		envVarHint = "INDEXER_SERVICE_URL"
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, proxyErr error) {
		log.Printf("proxy %s error: %v", serviceName, proxyErr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		msg := serviceName + " service unavailable"
		guidance := "Verify the service is running at " + baseURL
		if envVarHint != "" {
			guidance += " (configured via " + envVarHint + ")"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":    msg,
			"message":  msg + ": " + proxyErr.Error(),
			"guidance": guidance,
			"category": "connectivity",
		})
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		path := strings.TrimPrefix(req.URL.Path, stripPrefix)
		if path == "" {
			path = "/"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		req.URL.Path = joinURLPath(upstreamPrefix, path)
	}
	return proxy
}

func spaFallbackHandler(assetsDir string) http.Handler {
	indexPath := filepath.Join(assetsDir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ui-api/") || r.URL.Path == "/metrics" {
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(indexPath); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}

func joinURLPath(base string, suffix string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "/"
	}
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		suffix = "/"
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	if !strings.HasPrefix(suffix, "/") {
		suffix = "/" + suffix
	}
	if base == "/" {
		return suffix
	}
	if suffix == "/" {
		return base
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(suffix, "/")
}
