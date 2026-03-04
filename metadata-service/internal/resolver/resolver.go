package resolver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"metadata-service/internal/cache"
	"metadata-service/internal/model"
	"metadata-service/internal/provider"
	"metadata-service/internal/store"
)

type Resolver interface {
	SearchWorks(ctx context.Context, query string) ([]model.Work, error)
	ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error)
	GetWork(ctx context.Context, id string) (*model.Work, error)
}

// Stores bundles all store dependencies.
type Stores struct {
	Works    store.WorkStore
	Authors  store.AuthorStore
	Editions store.EditionStore
	IDs      store.IdentifierStore
	Mappings store.ProviderMappingStore
}

type defaultResolver struct {
	registry *provider.Registry
	works    store.WorkStore
	authors  store.AuthorStore
	editions store.EditionStore
	ids      store.IdentifierStore
	mappings store.ProviderMappingStore
	cache    cache.Cache
	merger   Merger
	identity IdentityResolver
}

func New(registry *provider.Registry, s Stores, c cache.Cache) Resolver {
	return &defaultResolver{
		registry: registry,
		works:    s.Works,
		authors:  s.Authors,
		editions: s.Editions,
		ids:      s.IDs,
		mappings: s.Mappings,
		cache:    c,
		merger:   NewMerger(),
		identity: NewIdentityResolver(s.Works, s.Mappings),
	}
}

func (r *defaultResolver) SearchWorks(ctx context.Context, query string) ([]model.Work, error) {
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
		log.Debug().Str("key", cacheKey).Msg("cache hit")
		return cached.([]model.Work), nil
	}

	// search canonical DB first
	dbResults, err := r.works.SearchWorks(ctx, cq.Normalized)
	if err == nil && len(dbResults) >= 3 {
		r.cache.Set(cacheKey, dbResults, time.Hour)
		return dbResults, nil
	}

	// query providers concurrently
	providers := r.registry.EnabledProviders()
	resultsCh := make(chan []model.Work, len(providers))

	var wg sync.WaitGroup
	for _, p := range providers {
		wg.Add(1)
		go func(p provider.Provider) {
			defer wg.Done()
			results, err := p.SearchWorks(ctx, cq.Normalized)
			if err != nil {
				log.Warn().Err(err).Str("provider", p.Name()).Msg("provider search failed")
				return
			}
			resultsCh <- results
		}(p)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var allResults [][]model.Work
	for batch := range resultsCh {
		allResults = append(allResults, batch)
	}

	merged, err := r.merger.MergeWorks(allResults)
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
	return nil
}

func (r *defaultResolver) ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error) {
	cacheKey := fmt.Sprintf("id:%s:%s", idType, value)
	if cached, ok := r.cache.Get(cacheKey); ok {
		e := cached.(model.Edition)
		return &e, nil
	}

	// DB first
	e, err := r.ids.FindEditionByIdentifier(ctx, idType, value)
	if err == nil {
		r.cache.Set(cacheKey, *e, time.Hour)
		return e, nil
	}

	// fall through to providers
	for _, p := range r.registry.EnabledProviders() {
		edition, err := p.ResolveIdentifier(ctx, idType, value)
		if err == nil && edition != nil {
			r.cache.Set(cacheKey, *edition, time.Hour)
			return edition, nil
		}
	}

	return nil, fmt.Errorf("identifier not found: %s %s", idType, value)
}

func (r *defaultResolver) GetWork(ctx context.Context, id string) (*model.Work, error) {
	cacheKey := fmt.Sprintf("work:%s", id)
	if cached, ok := r.cache.Get(cacheKey); ok {
		w := cached.(model.Work)
		return &w, nil
	}

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
