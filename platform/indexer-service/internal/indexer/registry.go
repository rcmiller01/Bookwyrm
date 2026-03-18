package indexer

import (
	"context"
	"sort"
)

type Adapter interface {
	Name() string
	Capabilities() []string
	Search(ctx context.Context, req SearchRequest) (SearchResult, error)
	HealthCheck(ctx context.Context) error
}

type Registry struct {
	adapters []Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: []Adapter{}}
}

func (r *Registry) Register(adapter Adapter) {
	r.adapters = append(r.adapters, adapter)
}

func (r *Registry) ListStatus(ctx context.Context) []AdapterStatus {
	statuses := make([]AdapterStatus, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		err := adapter.HealthCheck(ctx)
		statuses = append(statuses, AdapterStatus{
			Name:         adapter.Name(),
			Capabilities: adapter.Capabilities(),
			Healthy:      err == nil,
		})
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })
	return statuses
}
