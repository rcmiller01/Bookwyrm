package indexer

import (
	"context"
	"time"
)

type MockSearchBackend struct {
	id       string
	name     string
	pipeline string
}

func NewMockSearchBackend(id string, name string, pipeline string) *MockSearchBackend {
	return &MockSearchBackend{id: id, name: name, pipeline: pipeline}
}

func (b *MockSearchBackend) ID() string       { return b.id }
func (b *MockSearchBackend) Name() string     { return b.name }
func (b *MockSearchBackend) Pipeline() string { return b.pipeline }
func (b *MockSearchBackend) Capabilities() BackendCapabilities {
	return BackendCapabilities{Protocols: []string{"http"}, Supports: []string{"search"}}
}
func (b *MockSearchBackend) Search(_ context.Context, q QuerySpec) ([]Candidate, error) {
	now := time.Now().UTC()
	seeders := 10
	c := Candidate{
		CandidateID:     b.id + ":" + q.EntityID,
		Title:           q.Title,
		NormalizedTitle: normalizeText(q.Title),
		Protocol:        "http",
		MatchConfidence: 0.65,
		Score:           0.65,
		ReasonCodes:     []string{"mock_backend"},
		Reasons:         []Reason{{Code: "mock_backend", Weight: 0.65}},
		SourcePipeline:  b.pipeline,
		SourceBackendID: b.id,
		Seeders:         &seeders,
		PublishedAt:     &now,
		GrabPayload: map[string]any{
			"protocol":    "http",
			"downloadUrl": "https://example.invalid/mock/" + q.EntityID,
		},
	}
	return []Candidate{c}, nil
}
func (b *MockSearchBackend) HealthCheck(_ context.Context) error { return nil }
