package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"app-backend/internal/api"
	"app-backend/internal/integration/download"
	"app-backend/internal/integration/indexer"
	"app-backend/internal/integration/metadata"
	"app-backend/internal/jobs"
	"app-backend/internal/store"
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
