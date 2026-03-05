package resolver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"metadata-service/internal/cache"
	"metadata-service/internal/metrics"
	"metadata-service/internal/model"
	"metadata-service/internal/provider"
	"metadata-service/internal/store"

	"github.com/rs/zerolog/log"
)

type Resolver interface {
	SearchWorks(ctx context.Context, query string) ([]model.Work, error)
	ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error)
	GetWork(ctx context.Context, id string) (*model.Work, error)
}

// Stores bundles all store dependencies.
type Stores struct {
	Works       store.WorkStore
	Authors     store.AuthorStore
	Editions    store.EditionStore
	IDs         store.IdentifierStore
	Mappings    store.ProviderMappingStore
	Status      store.ProviderStatusStore
	ProvMetrics store.ProviderMetricsStore
	Reliability store.ReliabilityStore
	Enrichment  store.EnrichmentJobStore
}

type defaultResolver struct {
	registry    *provider.Registry
	rateLimiter *provider.RateLimiter
	works       store.WorkStore
	authors     store.AuthorStore
	editions    store.EditionStore
	ids         store.IdentifierStore
	mappings    store.ProviderMappingStore
	status      store.ProviderStatusStore
	provMetrics store.ProviderMetricsStore
	reliability store.ReliabilityStore
	enrichment  store.EnrichmentJobStore
	cache       cache.Cache
	merger      Merger
	identity    IdentityResolver
}

func New(registry *provider.Registry, rateLimiter *provider.RateLimiter, s Stores, c cache.Cache) Resolver {
	return &defaultResolver{
		registry:    registry,
		rateLimiter: rateLimiter,
		works:       s.Works,
		authors:     s.Authors,
		editions:    s.Editions,
		ids:         s.IDs,
		mappings:    s.Mappings,
		status:      s.Status,
		provMetrics: s.ProvMetrics,
		reliability: s.Reliability,
		enrichment:  s.Enrichment,
		cache:       c,
		merger:      NewMerger(),
		identity:    NewIdentityResolver(s.Works, s.Mappings),
	}
}

func (r *defaultResolver) SearchWorks(ctx context.Context, query string) ([]model.Work, error) {
	start := time.Now()
	metrics.ResolverRequestsTotal.Inc()
	defer func() {
		metrics.ResolverLatencyMs.Observe(float64(time.Since(start).Milliseconds()))
	}()

	cq := ClassifyQuery(query)

	// identifier shortcut
	if cq.Type != QueryTypeText {
		edition, err := r.ResolveIdentifier(ctx, cq.IdentifierType, cq.IdentifierValue)
		if err == nil && edition != nil {
			work, err := r.works.GetWorkByID(ctx, edition.WorkID)
			if err == nil {
				return []model.Work{*work}, nil
			}
		}
	}

	cacheKey := fmt.Sprintf("search:%s", cq.Normalized)
	if cached, ok := r.cache.Get(cacheKey); ok {
		metrics.CacheHitsTotal.Inc()
		log.Debug().Str("key", cacheKey).Msg("cache hit")
		return cached.([]model.Work), nil
	}
	metrics.CacheMissesTotal.Inc()

	// search canonical DB first
	dbResults, err := r.works.SearchWorks(ctx, cq.Normalized)
	if err == nil && len(dbResults) >= 3 {
		r.cache.Set(cacheKey, dbResults, time.Hour)
		return dbResults, nil
	}

	// Pre-gate: apply rate limiting before sizing the channel or launching goroutines.
	// This ensures resultsCh capacity exactly matches the number of active workers,
	// and no goroutine is launched for a provider that will be skipped.
	type activeProvider struct {
		p       provider.Provider
		timeout time.Duration
	}
	all := r.registry.EnabledProviders()
	scoreMap := r.loadReliabilityScores(context.Background())

	var active []activeProvider
	for _, p := range all {
		if !r.rateLimiter.Allow(p.Name()) {
			log.Warn().Str("provider", p.Name()).Msg("rate limited, skipping")
			continue
		}
		active = append(active, activeProvider{p: p, timeout: r.registry.TimeoutFor(p.Name())})
	}

	resultsCh := make(chan ProviderResult, len(active))

	var wg sync.WaitGroup
	for _, ap := range active {
		wg.Add(1)
		go func(ap activeProvider) {
			defer wg.Done()
			pStart := time.Now()
			metrics.ProviderRequestsTotal.WithLabelValues(ap.p.Name()).Inc()

			// Derive context from caller — if the request is abandoned the
			// parent ctx is cancelled, which propagates here immediately.
			// The per-provider deadline caps long-running upstreams independently.
			pCtx, cancel := context.WithTimeout(ctx, ap.timeout)
			defer cancel()

			results, err := ap.p.SearchWorks(pCtx, cq.Normalized)
			elapsed := time.Since(pStart)
			metrics.ProviderLatencyMs.WithLabelValues(ap.p.Name()).Observe(float64(elapsed.Milliseconds()))

			if err != nil {
				metrics.ProviderFailuresTotal.WithLabelValues(ap.p.Name()).Inc()
				log.Warn().Err(err).Str("provider", ap.p.Name()).Msg("provider search failed")
				r.recordFailure(context.Background(), ap.p.Name())
				return
			}
			metrics.ProviderSuccessTotal.WithLabelValues(ap.p.Name()).Inc()
			r.recordSuccess(context.Background(), ap.p.Name(), elapsed)
			resultsCh <- ProviderResult{Provider: ap.p.Name(), Works: results}
		}(ap)
	}

	// The resolver owns channel closure; workers only send.
	// The closer goroutine waits until every launched worker has called wg.Done()
	// before closing, so range-over-channel in the collector below terminates cleanly.
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var allResults []ProviderResult
	for pr := range resultsCh {
		allResults = append(allResults, pr)
	}

	// fallback: if all providers failed, return whatever DB had
	if len(allResults) == 0 && len(dbResults) > 0 {
		log.Warn().Str("query", query).Msg("all providers failed, returning DB results")
		return dbResults, nil
	}

	merged, err := r.merger.MergeWorksWeighted(allResults, scoreMap)
	if err != nil {
		return nil, err
	}

	// persist and canonicalize
	for i := range merged {
		canonicalID, err := r.identity.ResolveWork(ctx, merged[i])
		if err == nil {
			merged[i].ID = canonicalID
		}
		if err := r.persistWork(ctx, merged[i]); err != nil {
			log.Warn().Err(err).Str("work", merged[i].ID).Msg("failed to persist work")
		}
	}

	r.cache.Set(cacheKey, merged, time.Hour)
	r.scheduleSearchEnrichment(context.Background(), merged)
	return merged, nil
}

func (r *defaultResolver) persistWork(ctx context.Context, w model.Work) error {
	if err := r.works.InsertWork(ctx, w); err != nil {
		return err
	}
	for _, a := range w.Authors {
		if err := r.authors.InsertAuthor(ctx, a); err != nil {
			log.Warn().Err(err).Str("author", a.Name).Msg("failed to insert author")
			continue
		}
		_ = r.authors.LinkWorkAuthor(ctx, w.ID, a.ID)
	}
	for _, e := range w.Editions {
		e.WorkID = w.ID
		if err := r.editions.InsertEdition(ctx, e); err != nil {
			log.Warn().Err(err).Str("edition", e.ID).Msg("failed to insert edition")
			continue
		}
		for _, id := range e.Identifiers {
			_ = r.ids.InsertIdentifier(ctx, e.ID, id)
		}
	}
	r.scheduleGraphUpdate(ctx, w.ID)
	return nil
}

func (r *defaultResolver) ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error) {
	cacheKey := fmt.Sprintf("id:%s:%s", idType, value)
	if cached, ok := r.cache.Get(cacheKey); ok {
		metrics.CacheHitsTotal.Inc()
		e := cached.(model.Edition)
		return &e, nil
	}
	metrics.CacheMissesTotal.Inc()

	// DB first
	e, err := r.ids.FindEditionByIdentifier(ctx, idType, value)
	if err == nil {
		r.cache.Set(cacheKey, *e, time.Hour)
		return e, nil
	}

	// fall through to providers
	for _, p := range r.registry.EnabledProviders() {
		if !r.rateLimiter.Allow(p.Name()) {
			continue
		}
		metrics.ProviderRequestsTotal.WithLabelValues(p.Name()).Inc()
		pStart := time.Now()
		edition, err := p.ResolveIdentifier(ctx, idType, value)
		elapsed := time.Since(pStart)
		metrics.ProviderLatencyMs.WithLabelValues(p.Name()).Observe(float64(elapsed.Milliseconds()))
		if err == nil && edition != nil {
			metrics.ProviderSuccessTotal.WithLabelValues(p.Name()).Inc()
			r.recordSuccess(context.Background(), p.Name(), elapsed)
			r.cache.Set(cacheKey, *edition, time.Hour)
			r.scheduleIdentifierEnrichment(context.Background(), *edition)
			return edition, nil
		}
		metrics.ProviderFailuresTotal.WithLabelValues(p.Name()).Inc()
		r.recordFailure(context.Background(), p.Name())
	}

	return nil, fmt.Errorf("identifier not found: %s %s", idType, value)
}

func (r *defaultResolver) GetWork(ctx context.Context, id string) (*model.Work, error) {
	cacheKey := fmt.Sprintf("work:%s", id)
	if cached, ok := r.cache.Get(cacheKey); ok {
		metrics.CacheHitsTotal.Inc()
		w := cached.(model.Work)
		return &w, nil
	}
	metrics.CacheMissesTotal.Inc()

	w, err := r.works.GetWorkByID(ctx, id)
	if err != nil {
		return nil, err
	}

	editions, _ := r.editions.GetEditionsByWork(ctx, id)
	for i := range editions {
		identifiers, _ := r.ids.GetIdentifiersByEdition(ctx, editions[i].ID)
		editions[i].Identifiers = identifiers
	}
	w.Editions = editions

	r.cache.Set(cacheKey, *w, time.Hour)
	return w, nil
}

// --- reliability helpers ---

// loadReliabilityScores returns a map of provider name → composite score.
// If the reliability store is unavailable, an empty map is returned so the
// resolver falls back to the configured priority order.
func (r *defaultResolver) loadReliabilityScores(ctx context.Context) map[string]float64 {
	out := make(map[string]float64)
	if r.reliability == nil {
		return out
	}
	scores, err := r.reliability.GetAllScores(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("resolver: failed to load reliability scores, using priority order")
		return out
	}
	for _, s := range scores {
		out[s.Provider] = s.CompositeScore
	}
	return out
}

// recordSuccess records a successful provider call to both the legacy status
// store and the new provider_metrics table.
func (r *defaultResolver) recordSuccess(ctx context.Context, providerName string, elapsed time.Duration) {
	if r.status != nil {
		_ = r.status.RecordSuccess(ctx, providerName, elapsed.Milliseconds())
	}
	if r.provMetrics != nil {
		_ = r.provMetrics.RecordSuccess(ctx, providerName, elapsed)
	}
}

// recordFailure records a failed provider call to both the legacy status
// store and the new provider_metrics table.
func (r *defaultResolver) recordFailure(ctx context.Context, providerName string) {
	if r.status != nil {
		_ = r.status.RecordFailure(ctx, providerName)
	}
	if r.provMetrics != nil {
		_ = r.provMetrics.RecordFailure(ctx, providerName)
	}
}

// scheduleSearchEnrichment enqueues best-effort enrichment jobs without blocking
// the interactive request path.
func (r *defaultResolver) scheduleSearchEnrichment(ctx context.Context, works []model.Work) {
	if r.enrichment == nil || len(works) == 0 {
		return
	}
	go func() {
		const maxJobs = 3
		const minConfidence = 0.85

		enqueued := 0
		for _, work := range works {
			if enqueued >= maxJobs {
				break
			}
			if work.ID == "" || work.Confidence < minConfidence {
				continue
			}
			_, err := r.enrichment.EnqueueJob(ctx, model.EnrichmentJob{
				JobType:    model.EnrichmentJobTypeWorkEditions,
				EntityType: "work",
				EntityID:   work.ID,
				Priority:   50,
			})
			if err == nil {
				enqueued++
			}
		}
	}()
}

// scheduleIdentifierEnrichment enqueues work_editions for resolved identifier hits.
func (r *defaultResolver) scheduleIdentifierEnrichment(ctx context.Context, edition model.Edition) {
	if r.enrichment == nil || edition.WorkID == "" {
		return
	}
	go func() {
		_, _ = r.enrichment.EnqueueJob(ctx, model.EnrichmentJob{
			JobType:    model.EnrichmentJobTypeWorkEditions,
			EntityType: "work",
			EntityID:   edition.WorkID,
			Priority:   50,
		})
		_, _ = r.enrichment.EnqueueJob(ctx, model.EnrichmentJob{
			JobType:    model.EnrichmentJobTypeGraphUpdate,
			EntityType: "work",
			EntityID:   edition.WorkID,
			Priority:   52,
		})
	}()
}

func (r *defaultResolver) scheduleGraphUpdate(ctx context.Context, workID string) {
	if r.enrichment == nil || workID == "" {
		return
	}
	go func() {
		_, _ = r.enrichment.EnqueueJob(ctx, model.EnrichmentJob{
			JobType:    model.EnrichmentJobTypeGraphUpdate,
			EntityType: "work",
			EntityID:   workID,
			Priority:   52,
		})
	}()
}
