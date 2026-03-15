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
	"metadata-service/internal/enrichment"
	"metadata-service/internal/enrichment/handlers"
	"metadata-service/internal/graph"
	"metadata-service/internal/provider"
	"metadata-service/internal/provider/annasarchive"
	"metadata-service/internal/provider/crossref"
	"metadata-service/internal/provider/googlebooks"
	"metadata-service/internal/provider/hardcover"
	"metadata-service/internal/provider/librarything"
	"metadata-service/internal/provider/openlibrary"
	"metadata-service/internal/provider/worldcat"
	"metadata-service/internal/quality"
	"metadata-service/internal/recommend"
	"metadata-service/internal/resolver"
	"metadata-service/internal/store"
	"metadata-service/internal/version"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	log.Info().
		Str("version", version.Version).
		Str("commit", version.Commit).
		Str("built", version.BuildDate).
		Msg("starting bookwyrm metadata-service")

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

	if err := store.RunMigrations(ctx, pool); err != nil {
		log.Fatal().Err(err).Msg("failed to run metadata migrations")
	}

	// stores
	providerCfgStore := store.NewProviderConfigStore(pool)
	providerStatusStore := store.NewProviderStatusStore(pool)
	providerMetricsStore := store.NewProviderMetricsStore(pool)
	reliabilityStore := store.NewReliabilityStore(pool)
	enrichmentStore := store.NewEnrichmentJobStore(pool)
	seriesStore := store.NewSeriesStore(pool)
	subjectStore := store.NewSubjectStore(pool)
	workRelStore := store.NewWorkRelationshipStore(pool)
	recommendReadStore := store.NewRecommendReadStore(pool)

	stores := resolver.Stores{
		Works:       store.NewWorkStore(pool),
		Authors:     store.NewAuthorStore(pool),
		Editions:    store.NewEditionStore(pool),
		IDs:         store.NewIdentifierStore(pool),
		Mappings:    store.NewProviderMappingStore(pool),
		Status:      providerStatusStore,
		ProvMetrics: providerMetricsStore,
		Reliability: reliabilityStore,
		Enrichment:  enrichmentStore,
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
	quarantineMode := cfg.ProviderDispatchPolicy.QuarantineMode
	switch quarantineMode {
	case "", "last_resort":
		registry.SetQuarantineDisables(false)
		log.Info().Str("quarantine_mode", "last_resort").Msg("provider dispatch policy configured")
	case "disabled":
		registry.SetQuarantineDisables(true)
		log.Info().Str("quarantine_mode", "disabled").Msg("provider dispatch policy configured")
	default:
		registry.SetQuarantineDisables(false)
		log.Warn().Str("quarantine_mode", quarantineMode).Msg("unknown quarantine_mode; defaulting to last_resort")
	}

	// Build provider configs: YAML (with env var overrides already applied) supplies
	// defaults and API keys; DB is authoritative for operational fields
	// (enabled, priority, rate_limit, timeout) so operators can change them without
	// restarting the service via the provider management API.
	type buildCfg struct {
		timeout, rate, priority int
		enabled                 bool
		apiKey                  string
		baseURL                 string
		mailTo                  string
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
		builds[name] = buildCfg{
			timeout:  t,
			rate:     r,
			priority: p,
			enabled:  pc.Enabled,
			apiKey:   pc.APIKey,
			baseURL:  pc.BaseURL,
			mailTo:   pc.MailTo,
		}
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
		case "annasarchive":
			p = annasarchive.New(bc.timeout, bc.baseURL)
		case "librarything":
			p = librarything.New(bc.timeout, bc.baseURL)
		case "worldcat":
			p = worldcat.New(bc.timeout, bc.baseURL)
		case "crossref":
			p = crossref.New(bc.timeout, bc.mailTo, bc.baseURL)
		default:
			log.Warn().Str("provider", name).Msg("unknown provider in config, skipping")
			continue
		}
		registry.RegisterWithConfig(p, bc.priority, bc.enabled)
		registry.SetTimeout(name, time.Duration(bc.timeout)*time.Second)
		rl.Configure(name, bc.rate)
		log.Info().Str("provider", name).Bool("enabled", bc.enabled).Int("priority", bc.priority).Int("timeout_sec", bc.timeout).Msg("registered provider")
	}

	// resolver
	res := resolver.New(registry, rl, stores, c)
	graphBuilder := graph.NewBuilder(pool, stores.Works, seriesStore, subjectStore, workRelStore)
	recommendOptions := recommend.Options{
		Weights: recommend.ScoringWeights{
			SeriesNeighbor:  cfg.Recommendation.Weights.SeriesNeighbor,
			SameSeries:      cfg.Recommendation.Weights.SameSeries,
			SameAuthor:      cfg.Recommendation.Weights.SameAuthor,
			SharedSubject:   cfg.Recommendation.Weights.SharedSubject,
			ExplicitRelated: cfg.Recommendation.Weights.ExplicitRelated,
			PreferenceBoost: cfg.Recommendation.Weights.PreferenceBoost,
		},
		MaxDepth:           cfg.Recommendation.MaxDepth,
		MaxCandidatePool:   cfg.Recommendation.MaxCandidatePool,
		SeriesLimit:        cfg.Recommendation.SeriesLimit,
		AuthorLimit:        cfg.Recommendation.AuthorLimit,
		MaxSubjects:        cfg.Recommendation.MaxSubjects,
		MaxWorksPerSubject: cfg.Recommendation.MaxWorksPerSubject,
		RelationshipLimit:  cfg.Recommendation.RelationshipLimit,
		CacheTTL:           time.Duration(cfg.Recommendation.CacheTTLHours) * time.Hour,
	}
	recommendEngine := recommend.NewEngine(recommendReadStore, c, recommendOptions)
	qualityEngine := quality.NewEngine(quality.NewPGRepository(pool))

	// health monitor
	if cfg.HealthMonitor.Enabled {
		interval := time.Duration(cfg.HealthMonitor.IntervalMinutes) * time.Minute
		if interval == 0 {
			interval = 5 * time.Minute
		}
		monitor := provider.NewHealthMonitor(registry, providerStatusStore, interval).
			WithReliabilityStore(reliabilityStore)
		go monitor.Start(ctx)
	}

	// reliability worker — recomputes scores every 5 minutes
	reliabilityWorker := provider.NewReliabilityWorker(providerMetricsStore, reliabilityStore, registry, 5*time.Minute)
	go reliabilityWorker.Start(ctx)
	go graph.StartMetricsUpdater(ctx, seriesStore, subjectStore, workRelStore, 45*time.Second)

	if cfg.Enrichment.Enabled {
		handlerRegistry := handlers.NewRegistry()
		handlerRegistry.Register(handlers.NewWorkEditionsHandler(
			registry,
			rl,
			stores.Works,
			stores.Editions,
			stores.IDs,
			stores.ProvMetrics,
			cfg.Enrichment.Limits.MaxWorkEditions,
		))
		handlerRegistry.Register(handlers.NewAuthorExpandHandler(
			registry,
			rl,
			stores.Works,
			stores.Authors,
			stores.Mappings,
			enrichmentStore,
			cfg.Enrichment.Limits.MaxAuthorWorks,
			cfg.Enrichment.MaxJobsPerRequest,
		))
		handlerRegistry.Register(handlers.NewGraphUpdateWorkHandler(graphBuilder))

		enrichmentEngine := enrichment.NewEngine(cfg.Enrichment.WorkerCount, enrichmentStore, handlerRegistry)
		go enrichmentEngine.Start(ctx)
		log.Info().Int("workers", cfg.Enrichment.WorkerCount).Msg("enrichment engine started")
	}

	// API
	handlers := api.NewHandlers(
		res,
		recommendEngine,
		qualityEngine,
		registry,
		rl,
		providerCfgStore,
		providerStatusStore,
		reliabilityStore,
		enrichmentStore,
		stores.Works,
		seriesStore,
		subjectStore,
		workRelStore,
		cfg.Enrichment.Enabled,
		cfg.Enrichment.WorkerCount,
		cfg.ProviderDispatchPolicy.Source,
		cfg.ProviderDispatchPolicy.QuarantineMode,
	)
	handlers.SetDBPing(pool.Ping)
	router := api.NewRouter(handlers, api.RouterOptions{
		AuthEnabled:        cfg.API.Auth.Enabled,
		APIKeys:            cfg.API.Auth.Keys,
		RateLimitEnabled:   cfg.API.RateLimit.Enabled,
		RateLimitPerMinute: cfg.API.RateLimit.RequestsPerMinute,
		RateLimitBurst:     cfg.API.RateLimit.Burst,
	})

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
