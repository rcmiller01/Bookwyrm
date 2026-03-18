package resolver

import (
	"context"

	"metadata-service/internal/model"
	"metadata-service/internal/store"
)

type IdentityResolver interface {
	ResolveWork(ctx context.Context, work model.Work) (string, error)
}

type pgIdentityResolver struct {
	works    store.WorkStore
	mappings store.ProviderMappingStore
}

func NewIdentityResolver(works store.WorkStore, mappings store.ProviderMappingStore) IdentityResolver {
	return &pgIdentityResolver{works: works, mappings: mappings}
}

// ResolveWork returns the canonical work ID — existing if a fingerprint match is found, or the work's own ID if new.
func (r *pgIdentityResolver) ResolveWork(ctx context.Context, work model.Work) (string, error) {
	if work.Fingerprint != "" {
		existing, err := r.works.GetWorkByFingerprint(ctx, work.Fingerprint)
		if err == nil && existing != nil {
			return existing.ID, nil
		}
	}
	return work.ID, nil
}
