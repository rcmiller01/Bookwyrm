package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"indexer-service/internal/api"
	"indexer-service/internal/indexer"
)

func main() {
	listenAddr := envOrDefault("INDEXER_SERVICE_ADDR", ":8091")
	prowlarrURL := os.Getenv("PROWLARR_BASE_URL")
	prowlarrAPIKey := os.Getenv("PROWLARR_API_KEY")

	svc := indexer.NewService()
	if prowlarrURL != "" {
		svc.Register("prowlarr", indexer.NewProwlarrAdapter("prowlarr-primary", prowlarrURL, prowlarrAPIKey, 10*time.Second))
	} else {
		svc.Register("prowlarr", indexer.NewMockAdapter("prowlarr-primary", "prowlarr", []string{"availability", "files"}, true, 75*time.Millisecond))
	}
	svc.Register("non_prowlarr", indexer.NewMockAdapter("nonprowlarr-archive", "non_prowlarr", []string{"availability", "news"}, true, 90*time.Millisecond))

	h := api.NewHandlers(svc)
	router := api.NewRouter(h)

	log.Printf("indexer-service listening on %s", listenAddr)
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
