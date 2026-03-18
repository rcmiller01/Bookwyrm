package prowlarr

import (
	"context"

	"indexer-service/internal/indexer"
)

type Backend struct {
	id      string
	name    string
	adapter *indexer.ProwlarrAdapter
}

func NewBackend(id string, name string, adapter *indexer.ProwlarrAdapter) *Backend {
	return &Backend{id: id, name: name, adapter: adapter}
}

func (b *Backend) ID() string       { return b.id }
func (b *Backend) Name() string     { return b.name }
func (b *Backend) Pipeline() string { return "prowlarr" }
func (b *Backend) Capabilities() indexer.BackendCapabilities {
	return indexer.BackendCapabilities{
		Protocols: []string{"usenet", "torrent"},
		Supports:  []string{"availability", "search", "grab"},
	}
}

func (b *Backend) Search(ctx context.Context, q indexer.QuerySpec) ([]indexer.Candidate, error) {
	req := indexer.SearchRequest{
		Metadata: indexer.MetadataSnapshot{
			WorkID: b.id + ":" + q.EntityID,
			Title:  q.Title,
		},
		PreferredFormats: append([]string(nil), q.Preferences.Formats...),
	}
	if q.Author != "" {
		req.Metadata.Authors = []string{q.Author}
	}
	req.Metadata.ISBN13 = q.ISBN
	res, err := b.adapter.Search(ctx, req)
	if err != nil {
		return nil, err
	}
	for i := range res.Candidates {
		res.Candidates[i].SourcePipeline = "prowlarr"
		res.Candidates[i].SourceBackendID = b.id
	}
	return res.Candidates, nil
}

func (b *Backend) HealthCheck(ctx context.Context) error {
	return b.adapter.HealthCheck(ctx)
}
