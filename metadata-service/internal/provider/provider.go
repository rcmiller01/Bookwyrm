package provider

import (
	"context"
	"metadata-service/internal/model"
)

type Provider interface {
	Name() string
	SearchWorks(ctx context.Context, query string) ([]model.Work, error)
	GetWork(ctx context.Context, providerID string) (*model.Work, error)
	GetEditions(ctx context.Context, providerWorkID string) ([]model.Edition, error)
	ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error)
}

// Capabilities describes optional behavior that can be used by request-scoped
// routing and enrichment heuristics.
type Capabilities struct {
	SupportsSearch       bool
	SupportsISBN         bool
	SupportsDOI          bool
	SupportsSeries       bool
	SupportsSubjects     bool
	SupportsAuthorSearch bool
}

// CapableProvider is an optional extension for providers that expose
// capability flags. Providers not implementing this interface get a default.
type CapableProvider interface {
	Capabilities() Capabilities
}

// CapabilitiesFor returns provider capabilities with safe defaults so existing
// providers continue to work when they do not yet implement CapableProvider.
func CapabilitiesFor(p Provider) Capabilities {
	if cp, ok := p.(CapableProvider); ok {
		return cp.Capabilities()
	}
	return Capabilities{
		SupportsSearch:       true,
		SupportsISBN:         true,
		SupportsDOI:          false,
		SupportsSeries:       false,
		SupportsSubjects:     false,
		SupportsAuthorSearch: true,
	}
}
