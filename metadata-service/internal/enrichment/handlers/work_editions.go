package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"metadata-service/internal/model"
	"metadata-service/internal/provider"
	"metadata-service/internal/resolver"
	"metadata-service/internal/store"

	"github.com/rs/zerolog/log"
)

// WorkEditionsHandler expands editions for a canonical work by querying
// enabled providers in policy-aware order.
type WorkEditionsHandler struct {
	registry        *provider.Registry
	rateLimiter     *provider.RateLimiter
	workStore       store.WorkStore
	editionStore    store.EditionStore
	identifierStore store.IdentifierStore
	providerMetrics store.ProviderMetricsStore
	maxWorkEditions int
}

func NewWorkEditionsHandler(
	registry *provider.Registry,
	rateLimiter *provider.RateLimiter,
	workStore store.WorkStore,
	editionStore store.EditionStore,
	identifierStore store.IdentifierStore,
	providerMetrics store.ProviderMetricsStore,
	maxWorkEditions int,
) *WorkEditionsHandler {
	if maxWorkEditions <= 0 {
		maxWorkEditions = 100
	}
	return &WorkEditionsHandler{
		registry:        registry,
		rateLimiter:     rateLimiter,
		workStore:       workStore,
		editionStore:    editionStore,
		identifierStore: identifierStore,
		providerMetrics: providerMetrics,
		maxWorkEditions: maxWorkEditions,
	}
}

func (h *WorkEditionsHandler) Type() string {
	return model.EnrichmentJobTypeWorkEditions
}

func (h *WorkEditionsHandler) Handle(ctx context.Context, job model.EnrichmentJob) error {
	if job.EntityType != "work" {
		return fmt.Errorf("work_editions expects entity_type=work, got %q", job.EntityType)
	}

	work, err := h.workStore.GetWorkByID(ctx, job.EntityID)
	if err != nil {
		return fmt.Errorf("load work %s: %w", job.EntityID, err)
	}

	normalizedTitle := resolver.NormalizeQuery(work.Title)
	inserted := 0

	for _, p := range h.registry.EnabledProviders() {
		if inserted >= h.maxWorkEditions {
			break
		}
		if h.rateLimiter != nil && !h.rateLimiter.Allow(p.Name()) {
			continue
		}

		timeout := h.registry.TimeoutFor(p.Name())
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		pCtx, cancel := context.WithTimeout(ctx, timeout)
		providerWorkID, providerName := h.findProviderWorkID(pCtx, p, work.Title, normalizedTitle)
		cancel()
		if providerWorkID == "" {
			continue
		}

		pCtx, cancel = context.WithTimeout(ctx, timeout)
		editions, getErr := p.GetEditions(pCtx, providerWorkID)
		cancel()
		if getErr != nil {
			log.Debug().Err(getErr).Str("provider", p.Name()).Str("work_id", job.EntityID).
				Msg("work_editions: provider GetEditions failed")
			continue
		}

		for idx, edition := range editions {
			if inserted >= h.maxWorkEditions {
				break
			}
			if strings.TrimSpace(edition.ID) == "" {
				edition.ID = fmt.Sprintf("%s:%s:%d", job.EntityID, providerName, idx)
			}
			edition.WorkID = job.EntityID

			if err := h.editionStore.InsertEdition(ctx, edition); err != nil {
				continue
			}

			for _, id := range edition.Identifiers {
				_ = h.identifierStore.InsertIdentifier(ctx, edition.ID, id)
				if h.providerMetrics != nil {
					_ = h.providerMetrics.RecordIdentifierIntroduced(ctx, p.Name())
				}
			}
			inserted++
		}
	}

	if inserted == 0 {
		return fmt.Errorf("no editions discovered for work %s", job.EntityID)
	}
	return nil
}

func (h *WorkEditionsHandler) findProviderWorkID(
	ctx context.Context,
	p provider.Provider,
	workTitle string,
	normalizedTitle string,
) (string, string) {
	results, err := p.SearchWorks(ctx, workTitle)
	if err != nil || len(results) == 0 {
		return "", p.Name()
	}

	for _, result := range results {
		if resolver.NormalizeQuery(result.Title) == normalizedTitle {
			return result.ID, p.Name()
		}
	}
	return results[0].ID, p.Name()
}
