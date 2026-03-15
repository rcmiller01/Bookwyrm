package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"indexer-service/internal/api"
	"indexer-service/internal/indexer"
	mcpbackend "indexer-service/internal/indexer/backends/mcp"
	prowlarrbackend "indexer-service/internal/indexer/backends/prowlarr"
	"indexer-service/internal/mcp"
	"indexer-service/internal/version"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log.Printf("bookwyrm indexer-service version=%s commit=%s built=%s", version.Version, version.Commit, version.BuildDate)

	listenAddr := envOrDefault("INDEXER_SERVICE_ADDR", ":8091")
	prowlarrURL := os.Getenv("PROWLARR_BASE_URL")
	prowlarrAPIKey := os.Getenv("PROWLARR_API_KEY")
	metadataServiceURL := os.Getenv("METADATA_SERVICE_URL")
	databaseDSN := strings.TrimSpace(os.Getenv("DATABASE_DSN"))
	candidateRetention := atoiOrDefault(envOrDefault("INDEXER_CANDIDATE_RETENTION", "50"), 50)

	svc := indexer.NewService()
	store, cleanup := initStorage(databaseDSN)
	defer cleanup()
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()
	mcpRegistry := mcp.NewRegistry(store)
	mcpRuntime := mcp.NewRuntime()
	orchestrator := indexer.NewOrchestrator(store, strings.TrimSpace(envOrDefault("INDEXER_QUARANTINE_MODE", "last_resort")))
	orchestrator.SetMetadataClient(indexer.NewMetadataClient(metadataServiceURL, 10*time.Second))
	orchestrator.SetCandidateRetention(candidateRetention)
	orchestrator.Start(rootCtx, 2)
	reliabilityWorker := indexer.NewReliabilityWorker(store, 2*time.Minute)
	go reliabilityWorker.Start(rootCtx)
	if prowlarrURL != "" {
		adapter := indexer.NewProwlarrAdapter("prowlarr-primary", prowlarrURL, prowlarrAPIKey, 10*time.Second)
		svc.Register("prowlarr", adapter)
		orchestrator.RegisterBackend(
			prowlarrbackend.NewBackend("prowlarr:primary", "prowlarr-primary", adapter),
			indexer.BackendRecord{
				ID:               "prowlarr:primary",
				Name:             "prowlarr-primary",
				BackendType:      indexer.BackendTypeProwlarr,
				Enabled:          true,
				Tier:             indexer.TierPrimary,
				ReliabilityScore: 0.85,
				Priority:         100,
				Config:           map[string]any{"base_url": prowlarrURL},
			},
		)
	} else {
		svc.Register("prowlarr", indexer.NewMockAdapter("prowlarr-primary", "prowlarr", []string{"availability", "files"}, true, 75*time.Millisecond))
		orchestrator.RegisterBackend(
			indexer.NewMockSearchBackend("prowlarr:mock", "prowlarr-mock", "prowlarr"),
			indexer.BackendRecord{
				ID:               "prowlarr:mock",
				Name:             "prowlarr-mock",
				BackendType:      indexer.BackendTypeProwlarr,
				Enabled:          true,
				Tier:             indexer.TierPrimary,
				ReliabilityScore: 0.80,
				Priority:         100,
			},
		)
	}
	svc.Register("non_prowlarr", indexer.NewMockAdapter("nonprowlarr-archive", "non_prowlarr", []string{"availability", "news"}, true, 90*time.Millisecond))

	for _, server := range mcpRegistry.ListServers() {
		if !server.Enabled {
			continue
		}
		orchestrator.RegisterBackend(
			mcpbackend.NewBackend(server, mcpRuntime),
			indexer.BackendRecord{
				ID:               "mcp:" + server.ID,
				Name:             server.Name,
				BackendType:      indexer.BackendTypeMCP,
				Enabled:          true,
				Tier:             indexer.TierSecondary,
				ReliabilityScore: 0.70,
				Priority:         200,
				Config:           map[string]any{"server_id": server.ID},
			},
		)
	}

	h := api.NewHandlers(svc, store, orchestrator, mcpRegistry, mcpRuntime)
	router := api.NewRouter(h)

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("indexer-service listening on %s", listenAddr)
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

func initStorage(databaseDSN string) (indexer.Storage, func()) {
	if databaseDSN == "" {
		log.Printf("indexer storage: using in-memory store")
		return indexer.NewStore(), func() {}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseDSN)
	if err != nil {
		log.Fatalf("failed to connect postgres: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		log.Fatalf("failed to ping postgres: %v", err)
	}
	if err := indexer.RunMigrations(ctx, pool); err != nil {
		pool.Close()
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Printf("indexer storage: using postgres")
	return indexer.NewPGStore(pool), func() { pool.Close() }
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func atoiOrDefault(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value := 0
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil || value <= 0 {
		return fallback
	}
	return value
}
