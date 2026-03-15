package indexer

import "context"

type SearchBackend interface {
	ID() string
	Name() string
	Pipeline() string
	Capabilities() BackendCapabilities
	Search(ctx context.Context, q QuerySpec) ([]Candidate, error)
	HealthCheck(ctx context.Context) error
}

