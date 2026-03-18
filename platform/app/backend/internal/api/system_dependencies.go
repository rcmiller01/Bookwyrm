package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (h *Handlers) SystemDependencies(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	writeJSON(w, h.computeDependencySummary(ctx))
}

func (h *Handlers) computeDependencySummary(ctx context.Context) map[string]any {
	checks := make([]map[string]any, 0, 8)
	addCheck := func(key string, ready bool, blocking bool, detail string, guidance string, route string) {
		status := "ok"
		if !ready && blocking {
			status = "blocking"
		} else if !ready {
			status = "warning"
		}
		checks = append(checks, map[string]any{
			"key":      key,
			"ready":    ready,
			"blocking": blocking,
			"status":   status,
			"detail":   detail,
			"guidance": guidance,
			"route":    route,
		})
	}

	addCheck("backend_reachable", true, true, "backend API process is serving requests", "", "/system/status")

	metadataCheck := checkURLHealth(ctx, h.metadataHealthURL+"z", os.Getenv("METADATA_SERVICE_API_KEY"))
	metadataReady := metadataCheck["status"] == "ok"
	addCheck(
		"metadata_reachable",
		metadataReady,
		true,
		fmt.Sprintf("%v", metadataCheck["status"]),
		"Verify METADATA_SERVICE_URL and metadata-service health.",
		"/system/status",
	)

	indexerCheck := checkURLHealth(ctx, h.indexerHealthURL, os.Getenv("INDEXER_SERVICE_API_KEY"))
	indexerReady := indexerCheck["status"] == "ok"
	addCheck(
		"indexer_reachable",
		indexerReady,
		true,
		fmt.Sprintf("%v", indexerCheck["status"]),
		"Verify INDEXER_SERVICE_URL and indexer-service health.",
		"/system/status",
	)

	dsn := strings.TrimSpace(os.Getenv("DATABASE_DSN"))
	dbReady := false
	dbDetail := "DATABASE_DSN is not configured"
	if dsn != "" {
		if err := pingDatabaseDSN(ctx, dsn); err != nil {
			dbDetail = err.Error()
		} else {
			dbReady = true
			dbDetail = "postgres reachable"
		}
	}
	addCheck(
		"database_ready",
		dbReady,
		true,
		dbDetail,
		"Configure DATABASE_DSN to a reachable Postgres instance.",
		"/system/status",
	)

	libraryRoot := strings.TrimSpace(h.importConfig.LibraryRoot)
	libraryReady := false
	libraryDetail := "LIBRARY_ROOT is not configured"
	if libraryRoot != "" {
		if info, err := os.Stat(libraryRoot); err == nil && info.IsDir() {
			tmpPath := filepath.Join(libraryRoot, ".bookwyrm-write-check")
			writeErr := os.WriteFile(tmpPath, []byte("ok"), 0o644)
			if writeErr == nil {
				_ = os.Remove(tmpPath)
				libraryReady = true
				libraryDetail = "path exists and writable"
			} else {
				libraryDetail = "path exists but is not writable"
			}
		} else {
			libraryDetail = "path does not exist"
		}
	}
	addCheck(
		"library_root_ready",
		libraryReady,
		true,
		libraryDetail,
		"Set LIBRARY_ROOT to an existing writable library path.",
		"/settings/media-management",
	)

	enabledBackends := 0
	if indexerReady && strings.TrimSpace(h.indexerBaseURL) != "" {
		if backends, err := h.fetchJSONObject(ctx, h.indexerBaseURL+"/v1/indexer/backends", os.Getenv("INDEXER_SERVICE_API_KEY")); err == nil {
			if list, ok := backends["backends"].([]any); ok {
				for _, raw := range list {
					item, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					enabled, _ := item["enabled"].(bool)
					if enabled {
						enabledBackends++
					}
				}
			}
		}
	}
	addCheck(
		"search_backend_enabled",
		enabledBackends > 0,
		true,
		fmt.Sprintf("%d enabled backend(s)", enabledBackends),
		"Enable at least one indexer backend in Settings -> Indexers.",
		"/settings/indexers",
	)

	enabledClients := 0
	if h.downloadMgr != nil {
		for _, c := range h.downloadMgr.ListClients() {
			if c.Enabled {
				enabledClients++
			}
		}
	}
	addCheck(
		"download_client_enabled",
		enabledClients > 0,
		true,
		fmt.Sprintf("%d enabled client(s)", enabledClients),
		"Enable at least one download client in Settings -> Download Clients.",
		"/settings/download-clients",
	)

	blockingCount := 0
	warningCount := 0
	for _, check := range checks {
		ready, _ := check["ready"].(bool)
		if ready {
			continue
		}
		blocking, _ := check["blocking"].(bool)
		if blocking {
			blockingCount++
		} else {
			warningCount++
		}
	}
	status := "ready"
	if blockingCount > 0 {
		status = "degraded"
	}
	return map[string]any{
		"status":           status,
		"can_function_now": blockingCount == 0,
		"blocking_count":   blockingCount,
		"warning_count":    warningCount,
		"checks":           checks,
	}
}
