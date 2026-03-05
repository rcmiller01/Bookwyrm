package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"app-backend/internal/api"
	"app-backend/internal/integration/indexer"
	"app-backend/internal/integration/metadata"
	"app-backend/internal/store"
)

func main() {
	metadataURL := envOrDefault("METADATA_SERVICE_URL", "http://localhost:8080")
	metadataAPIKey := os.Getenv("METADATA_SERVICE_API_KEY")
	indexerURL := envOrDefault("INDEXER_SERVICE_URL", "http://localhost:8091")
	indexerAPIKey := os.Getenv("INDEXER_SERVICE_API_KEY")
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

	h := api.NewHandlers(metaClient, indexerClient, store.NewInMemoryWatchlistStore())
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
