package resolver

import (
	"sort"
	"strings"

	"metadata-service/internal/model"
	"metadata-service/internal/normalize"
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
		idx          int
		score        float64
		seriesScore  float64
		yearVotes    map[int]int
		yearMaxScore map[int]float64
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
					m.score = provScore
				}
				if shouldUpdateSeries(*existing, w, m.seriesScore, provScore) {
					existing.SeriesName = w.SeriesName
					existing.SeriesIndex = w.SeriesIndex
					m.seriesScore = provScore
				}
				existing.Subjects = mergeSubjects(existing.Subjects, w.Subjects, 25)
				existing.Editions = mergeEditionIdentifiers(existing.Editions)
				if w.FirstPubYear > 0 {
					if m.yearVotes == nil {
						m.yearVotes = map[int]int{}
					}
					if m.yearMaxScore == nil {
						m.yearMaxScore = map[int]float64{}
					}
					m.yearVotes[w.FirstPubYear]++
					if provScore > m.yearMaxScore[w.FirstPubYear] {
						m.yearMaxScore[w.FirstPubYear] = provScore
					}
					existing.FirstPubYear = pickBestYear(m.yearVotes, m.yearMaxScore, existing.FirstPubYear)
				}
				seen[fp] = m

				if existing.Confidence < w.Confidence {
					existing.Confidence = w.Confidence
				}
			} else {
				yearVotes := map[int]int{}
				yearMaxScore := map[int]float64{}
				if w.FirstPubYear > 0 {
					yearVotes[w.FirstPubYear] = 1
					yearMaxScore[w.FirstPubYear] = provScore
				}
				w.Subjects = mergeSubjects(nil, w.Subjects, 25)
				w.Editions = mergeEditionIdentifiers(w.Editions)
				seen[fp] = meta{
					idx:          len(merged),
					score:        provScore,
					seriesScore:  provScore,
					yearVotes:    yearVotes,
					yearMaxScore: yearMaxScore,
				}
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

func shouldUpdateSeries(existing model.Work, incoming model.Work, existingScore float64, incomingScore float64) bool {
	if incoming.SeriesName == nil || strings.TrimSpace(*incoming.SeriesName) == "" {
		return false
	}
	if existing.SeriesName == nil || strings.TrimSpace(*existing.SeriesName) == "" {
		return true
	}
	return incomingScore > existingScore
}

func mergeSubjects(existing []string, incoming []string, max int) []string {
	if max <= 0 {
		max = 25
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, max)
	add := func(subject string) {
		if len(out) >= max {
			return
		}
		trimmed := strings.TrimSpace(subject)
		if trimmed == "" {
			return
		}
		key := normalize.NormalizeSubject(trimmed)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	for _, s := range existing {
		add(s)
	}
	for _, s := range incoming {
		add(s)
	}
	return out
}

func mergeEditionIdentifiers(editions []model.Edition) []model.Edition {
	for i := range editions {
		seen := map[string]struct{}{}
		merged := make([]model.Identifier, 0, len(editions[i].Identifiers))
		for _, id := range editions[i].Identifiers {
			t := strings.ToUpper(strings.TrimSpace(id.Type))
			v := strings.TrimSpace(id.Value)
			if t == "" || v == "" {
				continue
			}
			key := t + "|" + strings.ToUpper(v)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, model.Identifier{Type: t, Value: v})
		}
		editions[i].Identifiers = merged
	}
	return editions
}

func pickBestYear(votes map[int]int, maxScore map[int]float64, current int) int {
	type candidate struct {
		year     int
		count    int
		maxScore float64
	}
	all := make([]candidate, 0, len(votes))
	for year, count := range votes {
		all = append(all, candidate{year: year, count: count, maxScore: maxScore[year]})
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].count == all[j].count {
			if all[i].maxScore == all[j].maxScore {
				return all[i].year < all[j].year
			}
			return all[i].maxScore > all[j].maxScore
		}
		return all[i].count > all[j].count
	})
	if len(all) == 0 {
		return current
	}
	return all[0].year
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
