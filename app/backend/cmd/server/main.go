package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"app-backend/internal/api"
	"app-backend/internal/downloadqueue"
	"app-backend/internal/importer"
	"app-backend/internal/integration/download"
	"app-backend/internal/integration/indexer"
	"app-backend/internal/integration/metadata"
	"app-backend/internal/jobs"
	"app-backend/internal/store"

	_ "github.com/lib/pq"
)

func main() {
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
	libraryRoot := envOrDefault("LIBRARY_ROOT", "./library")
	allowCrossDeviceMove := strings.EqualFold(envOrDefault("LIBRARY_ALLOW_CROSS_DEVICE_MOVE", "true"), "true")
	databaseDSN := os.Getenv("DATABASE_DSN")
	listenAddr := envOrDefault("APP_BACKEND_ADDR", ":8090")

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
	downloadStore, closeDownloadStore := initDownloadStore(databaseDSN)
	defer closeDownloadStore()
	seedDownloadClients(downloadStore, qbitURL != "", sabURL != "", nzbgetURL != "")
	downloadManager := downloadqueue.NewManager(downloadStore, downloadService, indexerClient, downloadQuarantineMode)
	downloadManager.Start(context.Background())
	importStore := initImporterStore(databaseDSN)
	defer closeImporterStore(importStore)
	importEngine := importer.NewEngine(importer.Config{
		LibraryRoot:          libraryRoot,
		AllowCrossDeviceMove: allowCrossDeviceMove,
		MaxScanFiles:         5000,
	}, importStore, downloadStore)
	importEngine.Start(context.Background())

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
	go jobService.Start(context.Background())

	h := api.NewHandlers(metaClient, indexerClient, store.NewInMemoryWatchlistStore())
	h.SetJobService(jobService)
	h.SetDownloadManager(downloadManager)
	router := api.NewRouter(h)

	log.Printf("app backend listening on %s, metadata-service=%s", listenAddr, metadataURL)
	if err := http.ListenAndServe(listenAddr, router); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
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
