package indexer

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type MockAdapter struct {
	name         string
	group        string
	capabilities []string
	healthy      bool
	networkDelay time.Duration
}

func NewMockAdapter(name string, group string, capabilities []string, healthy bool, delay time.Duration) Adapter {
	return &MockAdapter{
		name:         name,
		group:        group,
		capabilities: capabilities,
		healthy:      healthy,
		networkDelay: delay,
	}
}

func (a *MockAdapter) Name() string { return a.name }

func (a *MockAdapter) Capabilities() []string { return a.capabilities }

func (a *MockAdapter) HealthCheck(_ context.Context) error {
	if a.healthy {
		return nil
	}
	return fmt.Errorf("adapter %s unhealthy", a.name)
}

func (a *MockAdapter) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	if err := a.HealthCheck(ctx); err != nil {
		return SearchResult{}, err
	}
	if a.networkDelay > 0 {
		select {
		case <-time.After(a.networkDelay):
		case <-ctx.Done():
			return SearchResult{}, ctx.Err()
		}
	}

	title := strings.TrimSpace(req.Metadata.Title)
	if title == "" {
		title = req.Metadata.WorkID
	}

	base := Candidate{
		CandidateID:     fmt.Sprintf("%s-%s", a.group, req.Metadata.WorkID),
		Title:           title,
		Format:          "epub",
		MatchConfidence: 0.78,
		ProviderLink:    fmt.Sprintf("https://example.invalid/%s/%s", a.group, req.Metadata.WorkID),
		Provenance:      a.name,
		ReasonCodes:     []string{"title_fuzzy", "author_overlap"},
	}
	if req.Metadata.ISBN13 != "" || req.Metadata.ISBN10 != "" {
		base.MatchConfidence = 0.92
		base.ReasonCodes = append([]string{"identifier_exact"}, base.ReasonCodes...)
	}
	if a.group == "non_prowlarr" {
		base.Format = "pdf"
		base.MatchConfidence -= 0.05
	}

	return SearchResult{
		WorkID:     req.Metadata.WorkID,
		Source:     a.name,
		Found:      true,
		Candidates: []Candidate{base},
		SearchedAt: time.Now().UTC(),
		Trace: []AdapterTrace{{
			Adapter: a.name,
			Status:  "ok",
		}},
	}, nil
}
