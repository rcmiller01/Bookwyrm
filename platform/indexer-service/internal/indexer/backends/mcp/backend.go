package mcp

import (
	"context"
	"time"

	"indexer-service/internal/indexer"
	mcpruntime "indexer-service/internal/mcp"
)

type Backend struct {
	server  indexer.MCPServerRecord
	runtime *mcpruntime.Runtime
	client  *mcpruntime.Client
}

func NewBackend(server indexer.MCPServerRecord, runtime *mcpruntime.Runtime) *Backend {
	return &Backend{
		server:  server,
		runtime: runtime,
		client:  mcpruntime.NewClient(server.BaseURL, 10*time.Second),
	}
}

func (b *Backend) ID() string       { return "mcp:" + b.server.ID }
func (b *Backend) Name() string     { return b.server.Name }
func (b *Backend) Pipeline() string { return "mcp" }
func (b *Backend) Capabilities() indexer.BackendCapabilities {
	return indexer.BackendCapabilities{
		Protocols: []string{"http"},
		Supports:  []string{"availability", "search"},
	}
}

func (b *Backend) Search(ctx context.Context, q indexer.QuerySpec) ([]indexer.Candidate, error) {
	headers := b.runtime.HeadersFor(b.server)
	results, err := b.client.Search(ctx, q, headers)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].SourcePipeline = "mcp"
		results[i].SourceBackendID = b.ID()
	}
	return results, nil
}

func (b *Backend) HealthCheck(ctx context.Context) error {
	headers := b.runtime.HeadersFor(b.server)
	return b.client.Health(ctx, headers)
}
