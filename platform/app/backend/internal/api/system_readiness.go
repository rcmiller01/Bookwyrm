package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var pingDatabaseDSN = func(ctx context.Context, dsn string) error {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return db.PingContext(pingCtx)
}

type readinessItem struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Ready    bool   `json:"ready"`
	Blocking bool   `json:"blocking"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Guidance string `json:"guidance,omitempty"`
	Route    string `json:"route,omitempty"`
}

func (h *Handlers) SystemReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()

	items := make([]readinessItem, 0, 12)
	addItem := func(item readinessItem) {
		if item.Status == "" {
			if item.Ready {
				item.Status = "ok"
			} else if item.Blocking {
				item.Status = "blocking"
			} else {
				item.Status = "warning"
			}
		}
		items = append(items, item)
	}

	libraryRoot := strings.TrimSpace(h.importConfig.LibraryRoot)
	libraryConfigured := libraryRoot != ""
	addItem(readinessItem{
		Key:      "library_root_configured",
		Label:    "Library root configured",
		Ready:    libraryConfigured,
		Blocking: true,
		Detail:   fallbackString(libraryRoot, "LIBRARY_ROOT is not set"),
		Guidance: "Set LIBRARY_ROOT to your books library directory.",
		Route:    "/settings/media-management",
	})

	if libraryConfigured {
		info, err := os.Stat(libraryRoot)
		existsDir := err == nil && info.IsDir()
		addItem(readinessItem{
			Key:      "library_root_exists",
			Label:    "Library root exists",
			Ready:    existsDir,
			Blocking: true,
			Detail:   libraryRoot,
			Guidance: "Create the configured library directory or fix LIBRARY_ROOT.",
			Route:    "/settings/media-management",
		})
		if existsDir {
			tmpPath := filepath.Join(libraryRoot, ".bookwyrm-write-check")
			writeErr := os.WriteFile(tmpPath, []byte("ok"), 0o644)
			if writeErr == nil {
				_ = os.Remove(tmpPath)
			}
			addItem(readinessItem{
				Key:      "library_root_writable",
				Label:    "Library root writable",
				Ready:    writeErr == nil,
				Blocking: true,
				Detail:   libraryRoot,
				Guidance: "Grant write permissions to the Bookwyrm process account.",
				Route:    "/settings/media-management",
			})
		}
	}

	metadataCheck := checkURLHealth(ctx, h.metadataHealthURL+"z", os.Getenv("METADATA_SERVICE_API_KEY"))
	metadataReady := metadataCheck["status"] == "ok"
	addItem(readinessItem{
		Key:      "metadata_service_reachable",
		Label:    "Metadata service reachable",
		Ready:    metadataReady,
		Blocking: true,
		Detail:   fmt.Sprintf("%v", metadataCheck["url"]),
		Guidance: "Verify METADATA_SERVICE_URL and metadata-service health.",
		Route:    "/system/status",
	})

	indexerCheck := checkURLHealth(ctx, h.indexerHealthURL, os.Getenv("INDEXER_SERVICE_API_KEY"))
	indexerReady := indexerCheck["status"] == "ok"
	addItem(readinessItem{
		Key:      "indexer_service_reachable",
		Label:    "Indexer service reachable",
		Ready:    indexerReady,
		Blocking: true,
		Detail:   fmt.Sprintf("%v", indexerCheck["url"]),
		Guidance: "Verify INDEXER_SERVICE_URL and indexer-service health.",
		Route:    "/system/status",
	})

	enabledProviders := 0
	providerDetail := "metadata service unavailable"
	if metadataReady && strings.TrimSpace(h.metadataBaseURL) != "" {
		if providers, err := h.fetchJSONObject(ctx, h.metadataBaseURL+"/v1/providers", os.Getenv("METADATA_SERVICE_API_KEY")); err == nil {
			if list, ok := providers["providers"].([]any); ok {
				for _, raw := range list {
					item, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					if enabled, ok := item["enabled"].(bool); ok && enabled {
						enabledProviders++
					}
				}
				providerDetail = fmt.Sprintf("%d enabled provider(s)", enabledProviders)
			}
		} else {
			providerDetail = err.Error()
		}
	}
	addItem(readinessItem{
		Key:      "metadata_provider_enabled",
		Label:    "At least one metadata provider enabled",
		Ready:    enabledProviders > 0,
		Blocking: true,
		Detail:   providerDetail,
		Guidance: "Enable at least one metadata provider and test connectivity.",
		Route:    "/settings/metadata",
	})

	enabledBackends := 0
	prowlarrEnabled := false
	backendDetail := "indexer service unavailable"
	if indexerReady && strings.TrimSpace(h.indexerBaseURL) != "" {
		if backends, err := h.fetchJSONObject(ctx, h.indexerBaseURL+"/v1/indexer/backends", os.Getenv("INDEXER_SERVICE_API_KEY")); err == nil {
			if list, ok := backends["backends"].([]any); ok {
				for _, raw := range list {
					item, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					enabled, _ := item["enabled"].(bool)
					if !enabled {
						continue
					}
					enabledBackends++
					bt, _ := item["backend_type"].(string)
					if strings.EqualFold(strings.TrimSpace(bt), "prowlarr") {
						prowlarrEnabled = true
					}
				}
				backendDetail = fmt.Sprintf("%d enabled backend(s)", enabledBackends)
			}
		} else {
			backendDetail = err.Error()
		}
	}
	addItem(readinessItem{
		Key:      "search_backend_enabled",
		Label:    "At least one search backend enabled",
		Ready:    enabledBackends > 0,
		Blocking: true,
		Detail:   backendDetail,
		Guidance: "Enable at least one indexer backend in Settings -> Indexers.",
		Route:    "/settings/indexers",
	})

	prowlarrRequired := strings.EqualFold(fallbackString(os.Getenv("BOOKWYRM_PIPELINE_A_ENABLED"), "true"), "true")
	addItem(readinessItem{
		Key:      "prowlarr_if_pipeline_a",
		Label:    "Prowlarr configured (Pipeline A)",
		Ready:    !prowlarrRequired || prowlarrEnabled,
		Blocking: prowlarrRequired,
		Detail:   map[bool]string{true: "enabled", false: "not detected"}[prowlarrEnabled],
		Guidance: "Configure Prowlarr backend if Pipeline A is enabled.",
		Route:    "/settings/indexers",
	})

	enabledClients := 0
	if h.downloadMgr != nil {
		for _, c := range h.downloadMgr.ListClients() {
			if c.Enabled {
				enabledClients++
			}
		}
	}
	addItem(readinessItem{
		Key:      "download_client_enabled",
		Label:    "At least one download client enabled",
		Ready:    enabledClients > 0,
		Blocking: true,
		Detail:   fmt.Sprintf("%d enabled client(s)", enabledClients),
		Guidance: "Enable at least one download client and test connection.",
		Route:    "/settings/download-clients",
	})

	databaseDSN := strings.TrimSpace(os.Getenv("DATABASE_DSN"))
	databaseConfigured := databaseDSN != ""
	databaseReady := false
	databaseDetail := "DATABASE_DSN is not configured"
	if databaseConfigured {
		if err := pingDatabaseDSN(ctx, databaseDSN); err != nil {
			databaseDetail = err.Error()
		} else {
			databaseReady = true
			databaseDetail = "postgres reachable"
		}
	}
	addItem(readinessItem{
		Key:      "database_ready",
		Label:    "Database ready",
		Ready:    databaseReady,
		Blocking: true,
		Detail:   databaseDetail,
		Guidance: "Configure DATABASE_DSN to a reachable Postgres instance.",
		Route:    "/system/status",
	})

	migrationStatus := h.computeMigrationStatus(ctx)
	migrationState := strings.TrimSpace(fmt.Sprintf("%v", migrationStatus["status"]))
	migrationReady, _ := migrationStatus["ready"].(bool)
	migrationItemStatus := "warning"
	switch migrationState {
	case "ok":
		migrationItemStatus = "ok"
	case "failed":
		migrationItemStatus = "blocking"
	}
	addItem(readinessItem{
		Key:      "migrations_applied",
		Label:    "Database migrations applied",
		Ready:    migrationReady,
		Blocking: migrationState == "failed",
		Status:   migrationItemStatus,
		Detail:   fmt.Sprintf("%v", migrationStatus["detail"]),
		Guidance: "Apply pending migrations before upgrading further. Check docs/migrations.md and support bundle migration-status.",
		Route:    "/system/status",
	})

	blockingCount := 0
	warningCount := 0
	for _, item := range items {
		if item.Ready {
			continue
		}
		if item.Blocking {
			blockingCount++
		} else {
			warningCount++
		}
	}

	status := "ready"
	if blockingCount > 0 {
		status = "setup_required"
	} else if warningCount > 0 {
		status = "degraded"
	}

	writeJSON(w, map[string]any{
		"status":           status,
		"ready":            blockingCount == 0,
		"can_function_now": blockingCount == 0,
		"blocking_count":   blockingCount,
		"warning_count":    warningCount,
		"items":            items,
		"generated_at":     time.Now().UTC().Format(time.RFC3339),
	})
}
