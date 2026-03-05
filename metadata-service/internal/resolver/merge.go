package resolver

import (
	"strings"

	"metadata-service/internal/model"
)

// ProviderResult bundles a provider name with the works it returned.
// Used by the resolver to carry provenance information into the merge stage.
type ProviderResult struct {
	Provider string
	Works    []model.Work
}

// Merger merges work results from multiple providers into a deduplicated slice.
type Merger interface {
	// MergeWorks merges raw batches without reliability context (legacy / fallback).
	MergeWorks(results [][]model.Work) ([]model.Work, error)

	// MergeWorksWeighted merges provider-labelled results, using scores to resolve
	// field-level conflicts in favour of the more reliable provider.
	MergeWorksWeighted(results []ProviderResult, scores map[string]float64) ([]model.Work, error)
}

type defaultMerger struct{}

func NewMerger() Merger {
	return &defaultMerger{}
}

// MergeWorks is the legacy path — all batches treated with equal weight.
func (m *defaultMerger) MergeWorks(results [][]model.Work) ([]model.Work, error) {
	var provResults []ProviderResult
	for _, batch := range results {
		provResults = append(provResults, ProviderResult{Works: batch})
	}
	return m.MergeWorksWeighted(provResults, nil)
}

// MergeWorksWeighted deduplicates works by fingerprint and resolves field
// conflicts by preferring data from the provider with the higher reliability
// score. When scores are equal or unavailable the existing (first-seen) value
// is kept. Editions are always unioned regardless of source.
func (m *defaultMerger) MergeWorksWeighted(results []ProviderResult, scores map[string]float64) ([]model.Work, error) {
	// track the index in merged[] and the score of the provider that last wrote each fingerprint
	type meta struct {
		idx   int
		score float64
	}
	seen := make(map[string]meta)
	var merged []model.Work

	scoreFor := func(provider string) float64 {
		if scores != nil {
			if s, ok := scores[provider]; ok {
				return s
			}
		}
		return 0 // unknowns treated as equal to existing
	}

	for _, pr := range results {
		provScore := scoreFor(pr.Provider)
		for _, w := range pr.Works {
			fp := w.Fingerprint
			if fp == "" {
				authorName := ""
				if len(w.Authors) > 0 {
					authorName = w.Authors[0].Name
				}
				fp = GenerateFingerprint(w.Title, authorName, w.FirstPubYear)
				w.Fingerprint = fp
			}
			w.NormalizedTitle = NormalizeQuery(w.Title)

			if m, exists := seen[fp]; exists {
				existing := &merged[m.idx]

				// Always union editions — every source contributes.
				existing.Editions = append(existing.Editions, w.Editions...)

				// Field-level conflict resolution: higher-reliability provider wins.
				if provScore > m.score {
					if strings.TrimSpace(w.Title) != "" {
						existing.Title = w.Title
						existing.NormalizedTitle = w.NormalizedTitle
					}
					if w.FirstPubYear > 0 {
						existing.FirstPubYear = w.FirstPubYear
					}
					if len(w.Authors) > 0 {
						existing.Authors = w.Authors
					}
					// update the winning score so a subsequent even-better provider can override again
					seen[fp] = meta{idx: m.idx, score: provScore}
				}

				if existing.Confidence < w.Confidence {
					existing.Confidence = w.Confidence
				}
			} else {
				seen[fp] = meta{idx: len(merged), score: provScore}
				merged = append(merged, w)
			}
		}
	}

	// recompute confidence based on final completeness
	for i := range merged {
		merged[i].Confidence = scoreWork(merged[i])
	}

	return merged, nil
}

func scoreWork(w model.Work) float64 {
	score := 0.5
	if len(w.Authors) > 0 {
		score += 0.2
	}
	if w.FirstPubYear > 0 {
		score += 0.1
	}
	if len(w.Editions) > 0 {
		score += 0.1
	}
	if strings.TrimSpace(w.Title) != "" {
		score += 0.1
	}
	return score
}
