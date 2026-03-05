package indexer

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

type groupedAdapter struct {
	group   string
	adapter Adapter
}

type Service struct {
	groups map[string][]Adapter
}

func NewService() *Service {
	return &Service{groups: map[string][]Adapter{}}
}

func (s *Service) Register(group string, adapter Adapter) {
	key := normalizeGroup(group)
	s.groups[key] = append(s.groups[key], adapter)
}

func (s *Service) ListProviders(ctx context.Context) []AdapterStatus {
	statuses := []AdapterStatus{}
	for group, adapters := range s.groups {
		for _, adapter := range adapters {
			err := adapter.HealthCheck(ctx)
			statuses = append(statuses, AdapterStatus{
				Name:         adapter.Name(),
				Group:        group,
				Capabilities: adapter.Capabilities(),
				Healthy:      err == nil,
			})
		}
	}
	sort.Slice(statuses, func(i, j int) bool {
		if statuses[i].Group == statuses[j].Group {
			return statuses[i].Name < statuses[j].Name
		}
		return statuses[i].Group < statuses[j].Group
	})
	return statuses
}

func (s *Service) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	groups := req.BackendGroups
	if len(groups) == 0 {
		groups = []string{"prowlarr", "non_prowlarr"}
	}

	type resultItem struct {
		result SearchResult
		err    error
	}
	outCh := make(chan resultItem, len(groups)*2)
	var wg sync.WaitGroup

	dispatched := 0
	for _, rawGroup := range groups {
		group := normalizeGroup(rawGroup)
		adapters := s.groups[group]
		for _, adapter := range adapters {
			wg.Add(1)
			dispatched++
			go func(g string, ad Adapter) {
				defer wg.Done()
				res, err := ad.Search(ctx, req)
				if err == nil {
					res.Source = g + ":" + ad.Name()
				}
				outCh <- resultItem{result: res, err: err}
			}(group, adapter)
		}
	}

	if dispatched == 0 {
		return SearchResult{}, errors.New("no adapters configured for requested backend groups")
	}

	go func() {
		wg.Wait()
		close(outCh)
	}()

	merged := SearchResult{
		WorkID:     req.Metadata.WorkID,
		Source:     "multi-indexer",
		Found:      false,
		Candidates: []Candidate{},
		SearchedAt: time.Now().UTC(),
		Trace:      []AdapterTrace{},
	}

	for item := range outCh {
		if item.err != nil {
			merged.Trace = append(merged.Trace, AdapterTrace{Adapter: "unknown", Status: "error", Error: item.err.Error()})
			continue
		}
		merged.Candidates = append(merged.Candidates, item.result.Candidates...)
		merged.Trace = append(merged.Trace, item.result.Trace...)
	}

	sort.Slice(merged.Candidates, func(i, j int) bool {
		if merged.Candidates[i].MatchConfidence == merged.Candidates[j].MatchConfidence {
			return merged.Candidates[i].CandidateID < merged.Candidates[j].CandidateID
		}
		return merged.Candidates[i].MatchConfidence > merged.Candidates[j].MatchConfidence
	})
	if len(merged.Candidates) > 50 {
		merged.Candidates = merged.Candidates[:50]
	}
	merged.Found = len(merged.Candidates) > 0
	return merged, nil
}

func normalizeGroup(group string) string {
	value := strings.ToLower(strings.TrimSpace(group))
	if value == "" {
		return "non_prowlarr"
	}
	return value
}
