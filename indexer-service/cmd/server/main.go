package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"indexer-service/internal/api"
	"indexer-service/internal/indexer"
	mcpbackend "indexer-service/internal/indexer/backends/mcp"
	prowlarrbackend "indexer-service/internal/indexer/backends/prowlarr"
	"indexer-service/internal/mcp"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	listenAddr := envOrDefault("INDEXER_SERVICE_ADDR", ":8091")
	prowlarrURL := os.Getenv("PROWLARR_BASE_URL")
	prowlarrAPIKey := os.Getenv("PROWLARR_API_KEY")
	databaseDSN := strings.TrimSpace(os.Getenv("DATABASE_DSN"))
	candidateRetention := atoiOrDefault(envOrDefault("INDEXER_CANDIDATE_RETENTION", "50"), 50)

	svc := indexer.NewService()
	store, cleanup := initStorage(databaseDSN)
	defer cleanup()
	rootCtx := context.Background()
	mcpRegistry := mcp.NewRegistry(store)
	mcpRuntime := mcp.NewRuntime()
	orchestrator := indexer.NewOrchestrator(store, strings.TrimSpace(envOrDefault("INDEXER_QUARANTINE_MODE", "last_resort")))
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

	log.Printf("indexer-service listening on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, router); err != nil {
		log.Fatal(err)
	}
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
