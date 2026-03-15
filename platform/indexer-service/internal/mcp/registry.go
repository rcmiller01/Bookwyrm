package mcp

import "indexer-service/internal/indexer"

type Registry struct {
	store indexer.Storage
}

func NewRegistry(store indexer.Storage) *Registry {
	return &Registry{store: store}
}

func (r *Registry) ListServers() []indexer.MCPServerRecord {
	return r.store.ListMCPServers()
}

func (r *Registry) UpsertServer(rec indexer.MCPServerRecord) indexer.MCPServerRecord {
	return r.store.UpsertMCPServer(rec)
}

func (r *Registry) SetEnabled(id string, enabled bool) error {
	return r.store.SetMCPEnabled(id, enabled)
}

func (r *Registry) SetEnvMapping(id string, mapping map[string]string) error {
	return r.store.SetMCPEnvMapping(id, mapping)
}

func (r *Registry) Get(id string) (indexer.MCPServerRecord, error) {
	return r.store.GetMCPServer(id)
}
