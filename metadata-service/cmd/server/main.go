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

	// register providers from config
	for name, pc := range cfg.Providers {
		timeout := pc.TimeoutSeconds
		if timeout == 0 {
			timeout = 10
		}
		priority := pc.Priority
		if priority == 0 {
			priority = 100
		}
		rateLimit := pc.RateLimit
		if rateLimit == 0 {
			rateLimit = 60
		}

		var p provider.Provider
		switch name {
		case "openlibrary":
			p = openlibrary.New(timeout)
		case "googlebooks":
			p = googlebooks.New(timeout, pc.APIKey)
		case "hardcover":
			p = hardcover.New(timeout, pc.APIKey)
		default:
			log.Warn().Str("provider", name).Msg("unknown provider in config, skipping")
			continue
		}

		registry.RegisterWithConfig(p, priority, pc.Enabled)
		rl.Configure(name, rateLimit)
		log.Info().Str("provider", name).Bool("enabled", pc.Enabled).Int("priority", priority).Msg("registered provider")
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


