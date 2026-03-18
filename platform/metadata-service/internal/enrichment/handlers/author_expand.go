package handlers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"metadata-service/internal/model"
	"metadata-service/internal/provider"
	"metadata-service/internal/resolver"
	"metadata-service/internal/store"
)

// AuthorExpandHandler discovers additional works for an author-like query seed
// and schedules follow-up work_editions enrichment jobs.
type AuthorExpandHandler struct {
	registry         *provider.Registry
	rateLimiter      *provider.RateLimiter
	works            store.WorkStore
	authors          store.AuthorStore
	mappings         store.ProviderMappingStore
	jobStore         store.EnrichmentJobStore
	maxAuthorWorks   int
	maxJobsPerExpand int
}

func NewAuthorExpandHandler(
	registry *provider.Registry,
	rateLimiter *provider.RateLimiter,
	works store.WorkStore,
	authors store.AuthorStore,
	mappings store.ProviderMappingStore,
	jobStore store.EnrichmentJobStore,
	maxAuthorWorks int,
	maxJobsPerExpand int,
) *AuthorExpandHandler {
	if maxAuthorWorks <= 0 {
		maxAuthorWorks = 50
	}
	if maxJobsPerExpand <= 0 {
		maxJobsPerExpand = 5
	}
	return &AuthorExpandHandler{
		registry:         registry,
		rateLimiter:      rateLimiter,
		works:            works,
		authors:          authors,
		mappings:         mappings,
		jobStore:         jobStore,
		maxAuthorWorks:   maxAuthorWorks,
		maxJobsPerExpand: maxJobsPerExpand,
	}
}

func (h *AuthorExpandHandler) Type() string {
	return model.EnrichmentJobTypeAuthorExpand
}

func (h *AuthorExpandHandler) Handle(ctx context.Context, job model.EnrichmentJob) error {
	if job.EntityType != "author" {
		return fmt.Errorf("author_expand expects entity_type=author, got %q", job.EntityType)
	}

	authorQuery := strings.TrimSpace(job.EntityID)
	if authorQuery == "" {
		return fmt.Errorf("author_expand job missing entity_id")
	}
	normalizedQuery := resolver.NormalizeQuery(authorQuery)

	seen := map[string]struct{}{}
	var discovered []model.Work

	for _, p := range h.registry.EnabledProviders() {
		if len(discovered) >= h.maxAuthorWorks {
			break
		}
		if !provider.CapabilitiesFor(p).SupportsAuthorSearch {
			continue
		}
		if h.rateLimiter != nil && !h.rateLimiter.Allow(p.Name()) {
			continue
		}

		timeout := h.registry.TimeoutFor(p.Name())
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		pCtx, cancel := context.WithTimeout(ctx, timeout)
		results, err := p.SearchWorks(pCtx, authorQuery)
		cancel()
		if err != nil {
			continue
		}

		for _, work := range results {
			if len(discovered) >= h.maxAuthorWorks {
				break
			}
			if !matchesAuthor(work, normalizedQuery) {
				continue
			}
			if _, ok := seen[work.ID]; ok {
				continue
			}
			if strings.TrimSpace(work.ID) == "" {
				continue
			}
			work.NormalizedTitle = resolver.NormalizeQuery(work.Title)
			if err := h.works.InsertWork(ctx, work); err != nil {
				continue
			}
			for _, a := range work.Authors {
				if strings.TrimSpace(a.ID) == "" {
					a.ID = "author:" + resolver.NormalizeQuery(a.Name)
				}
				_ = h.authors.InsertAuthor(ctx, a)
				_ = h.authors.LinkWorkAuthor(ctx, work.ID, a.ID)
			}
			_ = h.mappings.InsertMapping(ctx, p.Name(), work.ID, "work", work.ID)
			seen[work.ID] = struct{}{}
			discovered = append(discovered, work)
		}
	}

	if len(discovered) == 0 {
		return fmt.Errorf("author_expand found no related works for %q", authorQuery)
	}

	// Stable ordering keeps scheduling deterministic.
	sort.SliceStable(discovered, func(i, j int) bool {
		return discovered[i].Confidence > discovered[j].Confidence
	})

	enqueued := 0
	graphEnqueued := 0
	for _, work := range discovered {
		if enqueued >= h.maxJobsPerExpand {
			break
		}
		_, err := h.jobStore.EnqueueJob(ctx, model.EnrichmentJob{
			JobType:    model.EnrichmentJobTypeWorkEditions,
			EntityType: "work",
			EntityID:   work.ID,
			Priority:   55,
		})
		if err == nil {
			enqueued++
		}

		if graphEnqueued < h.maxJobsPerExpand {
			_, graphErr := h.jobStore.EnqueueJob(ctx, model.EnrichmentJob{
				JobType:    model.EnrichmentJobTypeGraphUpdate,
				EntityType: "work",
				EntityID:   work.ID,
				Priority:   52,
			})
			if graphErr == nil {
				graphEnqueued++
			}
		}
	}

	return nil
}

func matchesAuthor(work model.Work, normalizedQuery string) bool {
	for _, author := range work.Authors {
		name := resolver.NormalizeQuery(author.Name)
		if name == normalizedQuery || strings.Contains(name, normalizedQuery) || strings.Contains(normalizedQuery, name) {
			return true
		}
	}
	return false
}
