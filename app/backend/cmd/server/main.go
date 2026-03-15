package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"app-backend/internal/api"
	"app-backend/internal/autograb"
	"app-backend/internal/domain/factory"
	"app-backend/internal/downloadqueue"
	"app-backend/internal/importer"
	"app-backend/internal/integration/download"
	"app-backend/internal/integration/indexer"
	"app-backend/internal/integration/metadata"
	"app-backend/internal/jobs"
	"app-backend/internal/store"
	"app-backend/internal/version"

	_ "github.com/lib/pq"
)

func main() {
	log.Printf("bookwyrm app-backend version=%s commit=%s built=%s", version.Version, version.Commit, version.BuildDate)

	metadataURL := envOrDefault("METADATA_SERVICE_URL", "http://localhost:8080")
	metadataAPIKey := os.Getenv("METADATA_SERVICE_API_KEY")
	indexerURL := envOrDefault("INDEXER_SERVICE_URL", "http://localhost:8091")
	indexerAPIKey := os.Getenv("INDEXER_SERVICE_API_KEY")
	qbitURL := os.Getenv("QBITTORRENT_BASE_URL")
	qbitUser := os.Getenv("QBITTORRENT_USERNAME")
	qbitPass := os.Getenv("QBITTORRENT_PASSWORD")
	sabURL := os.Getenv("SABNZBD_BASE_URL")
	sabAPIKey := os.Getenv("SABNZBD_API_KEY")
	sabCategory := envOrDefault("SABNZBD_CATEGORY", "books")
	nzbgetURL := os.Getenv("NZBGET_BASE_URL")
	nzbgetUser := os.Getenv("NZBGET_USERNAME")
	nzbgetPass := os.Getenv("NZBGET_PASSWORD")
	nzbgetCategory := envOrDefault("NZBGET_CATEGORY", "books")
	downloadQuarantineMode := envOrDefault("DOWNLOAD_QUARANTINE_MODE", "last_resort")
	autoGrabPollInterval := time.Duration(atoiOrDefault(envOrDefault("AUTO_GRAB_POLL_INTERVAL_SEC", "15"), 15)) * time.Second
	autoGrabMinScore := atofOrDefault(envOrDefault("AUTO_GRAB_MIN_SCORE", "0.70"), 0.70)
	libraryRoot := envOrDefault("LIBRARY_ROOT", "./library")
	allowCrossDeviceMove := strings.EqualFold(envOrDefault("LIBRARY_ALLOW_CROSS_DEVICE_MOVE", "true"), "true")
	namingTemplateEbook := envOrDefault("NAMING_TEMPLATE_EBOOK", "{Author}/{Title} ({Year})/{Title} - {Author}.{Ext}")
	namingTemplateAudiobookSingle := envOrDefault("NAMING_TEMPLATE_AUDIOBOOK_SINGLE", "{Author}/{Title} ({Year})/{Title} - {Author}.{Ext}")
	namingTemplateAudiobookFolder := envOrDefault("NAMING_TEMPLATE_AUDIOBOOK_FOLDER", "{Author}/{Title} ({Year})")
	namingMaxPathLen := atoiOrDefault(envOrDefault("NAMING_MAX_PATH_LEN", "240"), 240)
	namingReplaceColon := strings.EqualFold(envOrDefault("NAMING_REPLACE_COLON", "true"), "true")
	keepIncomingRaw, keepIncomingSource := envOrDefaultWithSource("IMPORT_KEEP_INCOMING", "true")
	keepIncoming := strings.EqualFold(keepIncomingRaw, "true")
	keepIncomingDays := atoiOrDefault(envOrDefault("LIBRARY_KEEP_INCOMING_DAYS", "14"), 14)
	keepTrashDays := atoiOrDefault(envOrDefault("LIBRARY_KEEP_TRASH_DAYS", "30"), 30)
	uiAssetsDir := envOrDefault("UI_DIST_DIR", "./web/dist")
	databaseDSN := os.Getenv("DATABASE_DSN")
	listenAddr := envOrDefault("APP_BACKEND_ADDR", ":8090")
	domainName := envOrDefault("APP_DOMAIN", "books")

	metaClient := metadata.NewClient(metadata.Config{
		BaseURL: metadataURL,
		APIKey:  metadataAPIKey,
		Timeout: 10 * time.Second,
	})
	indexerClient := indexer.NewClient(indexer.Config{
		BaseURL: indexerURL,
		APIKey:  indexerAPIKey,
		Timeout: 10 * time.Second,
	})

	domainPack, err := factory.Resolve(domainName)
	if err != nil {
		log.Fatalf("failed to resolve app domain %q: %v", domainName, err)
	}

	downloadService := download.NewService()
	if qbitURL != "" {
		downloadService.Register(download.NewQBitTorrentClient(download.QBitTorrentConfig{
			BaseURL:  qbitURL,
			Username: qbitUser,
			Password: qbitPass,
			Timeout:  10 * time.Second,
		}))
	}
	if sabURL != "" {
		downloadService.Register(download.NewSABnzbdClient(download.SABnzbdConfig{
			BaseURL:  sabURL,
			APIKey:   sabAPIKey,
			Category: sabCategory,
			Timeout:  10 * time.Second,
		}))
	}
	if nzbgetURL != "" {
		downloadService.Register(download.NewNZBGetClient(download.NZBGetConfig{
			BaseURL:  nzbgetURL,
			Username: nzbgetUser,
			Password: nzbgetPass,
			Category: nzbgetCategory,
			Timeout:  10 * time.Second,
		}))
	}
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	downloadStore, closeDownloadStore := initDownloadStore(databaseDSN)
	defer closeDownloadStore()
	seedDownloadClients(downloadStore, qbitURL != "", sabURL != "", nzbgetURL != "")
	downloadManager := downloadqueue.NewManager(downloadStore, downloadService, indexerClient, downloadQuarantineMode)
	downloadManager.Start(rootCtx)
	importStore := initImporterStore(databaseDSN)
	defer closeImporterStore(importStore)
	importEngine := importer.NewEngine(importer.Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    allowCrossDeviceMove,
		MaxScanFiles:            5000,
		TemplateEbook:           namingTemplateEbook,
		TemplateAudiobookSingle: namingTemplateAudiobookSingle,
		TemplateAudiobookFolder: namingTemplateAudiobookFolder,
		MaxPathLen:              namingMaxPathLen,
		ReplaceColon:            namingReplaceColon,
		KeepIncoming:            keepIncoming,
		KeepIncomingDays:        keepIncomingDays,
		KeepTrashDays:           keepTrashDays,
	}, importStore, downloadStore, metaClient)
	importEngine.Start(rootCtx)
	autoGrabWorker := autograb.NewWorker(indexerClient, downloadManager, importStore, autoGrabPollInterval, autoGrabMinScore)
	go autoGrabWorker.Start(rootCtx)

	jobStore := store.NewInMemoryJobStore()
	jobService := jobs.NewService(
		jobStore,
		jobs.Options{WorkerCount: 2, PollInterval: 500 * time.Millisecond},
		jobs.NewIndexerSearchHandler(indexerClient),
		jobs.NewDownloadEnqueueHandler(downloadService),
		jobs.NewDownloadPollHandler(downloadService),
		jobs.NewNoopHandler("import_completed"),
		jobs.NewNoopHandler("rename_finalize"),
	)
	go jobService.Start(rootCtx)

	h := api.NewHandlersWithDomain(metaClient, indexerClient, store.NewInMemoryWatchlistStore(), domainPack)
	h.SetJobService(jobService)
	h.SetDownloadManager(downloadManager)
	h.SetDownloadService(downloadService)
	h.SetImportStore(importStore)
	h.SetImportEngine(importEngine)
	h.SetImportConfig(api.ImportConfig{
		KeepIncoming: keepIncoming,
		Source:       keepIncomingSource,
		LibraryRoot:  libraryRoot,
	})
	h.SetUpstreamURLs(metadataURL, indexerURL)
	h.SetStartupTime(time.Now())
	h.SetLibraryRoot(libraryRoot)
	emitStartupWarnings(metadataURL, metadataAPIKey, indexerURL, indexerAPIKey, databaseDSN, downloadService)
	router := api.NewRouterWithConfig(h, api.RouterConfig{
		UIAssetsDir:          uiAssetsDir,
		MetadataProxyBaseURL: metadataURL,
		IndexerProxyBaseURL:  indexerURL,
	})

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("app backend listening on %s, metadata-service=%s, domain=%s", listenAddr, metadataURL, domainPack.Name())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down gracefully")
	rootCancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envOrDefaultWithSource(name string, fallback string) (string, string) {
	if value := os.Getenv(name); value != "" {
		return value, "env"
	}
	return fallback, "default"
}

func initDownloadStore(databaseDSN string) (downloadqueue.Storage, func()) {
	if databaseDSN == "" {
		log.Printf("download queue storage: using in-memory store")
		return downloadqueue.NewStore(), func() {}
	}
	db, err := sql.Open("postgres", databaseDSN)
	if err != nil {
		log.Printf("download queue storage: postgres unavailable (%v), falling back to in-memory", err)
		return downloadqueue.NewStore(), func() {}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		log.Printf("download queue storage: postgres ping failed (%v), falling back to in-memory", err)
		return downloadqueue.NewStore(), func() {}
	}
	if err := downloadqueue.RunMigrations(ctx, db); err != nil {
		_ = db.Close()
		log.Printf("download queue storage: migration failed (%v), falling back to in-memory", err)
		return downloadqueue.NewStore(), func() {}
	}
	log.Printf("download queue storage: using postgres")
	return downloadqueue.NewPGStore(db), func() { _ = db.Close() }
}

func seedDownloadClients(store downloadqueue.Storage, hasQbit bool, hasSab bool, hasNZBGet bool) {
	if hasQbit {
		store.UpsertClient(downloadqueue.DownloadClientRecord{
			ID:               "qbittorrent",
			Name:             "qBittorrent",
			ClientType:       "qbittorrent",
			Enabled:          true,
			Tier:             "secondary",
			ReliabilityScore: 0.70,
			Priority:         200,
			Config:           map[string]any{},
		})
	}
	if hasSab {
		store.UpsertClient(downloadqueue.DownloadClientRecord{
			ID:               "sabnzbd",
			Name:             "SABnzbd",
			ClientType:       "sabnzbd",
			Enabled:          true,
			Tier:             "secondary",
			ReliabilityScore: 0.70,
			Priority:         150,
			Config:           map[string]any{},
		})
	}
	if hasNZBGet {
		store.UpsertClient(downloadqueue.DownloadClientRecord{
			ID:               "nzbget",
			Name:             "NZBGet",
			ClientType:       "nzbget",
			Enabled:          true,
			Tier:             "primary",
			ReliabilityScore: 0.80,
			Priority:         100,
			Config:           map[string]any{},
		})
	}
}

func initImporterStore(databaseDSN string) importer.Store {
	if databaseDSN == "" {
		return importer.NewMemoryStore()
	}
	db, err := sql.Open("postgres", databaseDSN)
	if err != nil {
		return importer.NewMemoryStore()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return importer.NewMemoryStore()
	}
	return importer.NewPGStore(db)
}

func closeImporterStore(store importer.Store) {
	pg, ok := store.(*importer.PGStore)
	if !ok {
		return
	}
	_ = pg.Close()
}

func atoiOrDefault(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	var out int
	if _, err := fmt.Sscanf(raw, "%d", &out); err != nil || out <= 0 {
		return fallback
	}
	return out
}

func emitStartupWarnings(metadataURL, metadataAPIKey, indexerURL, indexerAPIKey, databaseDSN string, downloadService *download.Service) {
	if strings.TrimSpace(databaseDSN) == "" {
		log.Printf("startup warning: DATABASE_DSN is not configured; backend state will be in-memory only")
	} else if err := pingDatabase(databaseDSN); err != nil {
		log.Printf("startup warning: database unavailable (%v)", err)
	}

	if err := checkHTTPHealth(joinURL(metadataURL, "/healthz"), metadataAPIKey); err != nil {
		log.Printf("startup warning: metadata-service unavailable (%v)", err)
	}
	if err := checkHTTPHealth(joinURL(indexerURL, "/v1/indexer/health"), indexerAPIKey); err != nil {
		log.Printf("startup warning: indexer-service unavailable (%v)", err)
	}

	if enabledBackends, err := fetchEnabledIndexerBackends(indexerURL, indexerAPIKey); err != nil {
		log.Printf("startup warning: could not verify enabled indexer backends (%v)", err)
	} else if enabledBackends == 0 {
		log.Printf("startup warning: no enabled indexer backends detected")
	}

	if downloadService == nil || len(downloadService.ListClientNames()) == 0 {
		log.Printf("startup warning: no download clients configured")
	}
}

func pingDatabase(databaseDSN string) error {
	db, err := sql.Open("postgres", strings.TrimSpace(databaseDSN))
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

func checkHTTPHealth(healthURL string, apiKey string) error {
	if strings.TrimSpace(healthURL) == "" {
		return fmt.Errorf("health URL not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-API-Key", strings.TrimSpace(apiKey))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func fetchEnabledIndexerBackends(indexerURL string, apiKey string) (int, error) {
	if strings.TrimSpace(indexerURL) == "" {
		return 0, fmt.Errorf("indexer URL not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, joinURL(indexerURL, "/v1/indexer/backends"), nil)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-API-Key", strings.TrimSpace(apiKey))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	rawList, _ := body["backends"].([]any)
	enabled := 0
	for _, raw := range rawList {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		v, _ := item["enabled"].(bool)
		if v {
			enabled++
		}
	}
	return enabled, nil
}

func joinURL(base string, path string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	joinPath := "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	u.Path = strings.TrimRight(u.Path, "/") + joinPath
	return u.String()
}

func atofOrDefault(raw string, fallback float64) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	var out float64
	if _, err := fmt.Sscanf(raw, "%f", &out); err != nil || out <= 0 {
		return fallback
	}
	return out
}
