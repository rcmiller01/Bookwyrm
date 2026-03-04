package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"metadata-service/internal/api"
	"metadata-service/internal/cache"
	"metadata-service/internal/config"
	"metadata-service/internal/provider"
	"metadata-service/internal/provider/googlebooks"
	"metadata-service/internal/provider/hardcover"
	"metadata-service/internal/provider/openlibrary"
	"metadata-service/internal/resolver"
	"metadata-service/internal/store"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	cfgPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// database
	pool, err := store.NewPool(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()
	log.Info().Str("host", cfg.Database.Host).Msg("connected to database")

	// stores
	providerCfgStore := store.NewProviderConfigStore(pool)
	providerStatusStore := store.NewProviderStatusStore(pool)

	stores := resolver.Stores{
		Works:    store.NewWorkStore(pool),
		Authors:  store.NewAuthorStore(pool),
		Editions: store.NewEditionStore(pool),
		IDs:      store.NewIdentifierStore(pool),
		Mappings: store.NewProviderMappingStore(pool),
		Status:   providerStatusStore,
	}

	// cache
	c, err := cache.NewRistrettoCache()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create cache")
	}

	// rate limiter
	rl := provider.NewRateLimiter()

	// provider registry
	registry := provider.NewRegistry()

	// Build provider configs: YAML (with env var overrides already applied) supplies
	// defaults and API keys; DB is authoritative for operational fields
	// (enabled, priority, rate_limit, timeout) so operators can change them without
	// restarting the service via the provider management API.
	type buildCfg struct {
		timeout, rate, priority int
		enabled                 bool
		apiKey                  string
	}
	builds := make(map[string]buildCfg)
	for name, pc := range cfg.Providers {
		t := pc.TimeoutSeconds
		if t == 0 {
			t = 10
		}
		r := pc.RateLimit
		if r == 0 {
			r = 60
		}
		p := pc.Priority
		if p == 0 {
			p = 100
		}
		builds[name] = buildCfg{timeout: t, rate: r, priority: p, enabled: pc.Enabled, apiKey: pc.APIKey}
	}

	dbCfgs, dbErr := providerCfgStore.GetAll(ctx)
	if dbErr != nil {
		log.Warn().Err(dbErr).Msg("could not load provider config from DB, using config.yaml defaults")
	} else {
		for _, dc := range dbCfgs {
			bc := builds[dc.Name] // inherit YAML defaults (api_key, timeout) if present
			bc.enabled = dc.Enabled
			bc.priority = dc.Priority
			if dc.RateLimit > 0 {
				bc.rate = dc.RateLimit
			}
			if dc.TimeoutSec > 0 {
				bc.timeout = dc.TimeoutSec
			}
			// API key: prefer YAML/env (already applied by config.Load) over DB
			if bc.apiKey == "" && dc.APIKey != "" {
				bc.apiKey = dc.APIKey
			}
			builds[dc.Name] = bc
		}
		log.Info().Int("count", len(dbCfgs)).Msg("loaded provider config from database")
	}

	for name, bc := range builds {
		var p provider.Provider
		switch name {
		case "openlibrary":
			p = openlibrary.New(bc.timeout)
		case "googlebooks":
			p = googlebooks.New(bc.timeout, bc.apiKey)
		case "hardcover":
			p = hardcover.New(bc.timeout, bc.apiKey)
		default:
			log.Warn().Str("provider", name).Msg("unknown provider in config, skipping")
			continue
		}
		registry.RegisterWithConfig(p, bc.priority, bc.enabled)
		rl.Configure(name, bc.rate)
		log.Info().Str("provider", name).Bool("enabled", bc.enabled).Int("priority", bc.priority).Msg("registered provider")
	}

	// resolver
	res := resolver.New(registry, rl, stores, c)

	// health monitor
	if cfg.HealthMonitor.Enabled {
		interval := time.Duration(cfg.HealthMonitor.IntervalMinutes) * time.Minute
		if interval == 0 {
			interval = 5 * time.Minute
		}
		monitor := provider.NewHealthMonitor(registry, providerStatusStore, interval)
		go monitor.Start(ctx)
	}

	// API
	handlers := api.NewHandlers(res, registry, rl, providerCfgStore, providerStatusStore)
	router := api.NewRouter(handlers)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("starting metadata service")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down gracefully")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}


