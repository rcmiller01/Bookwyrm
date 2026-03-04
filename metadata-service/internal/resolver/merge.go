package resolver

import (
	"strings"

	"metadata-service/internal/model"
)

type Merger interface {
	MergeWorks(results [][]model.Work) ([]model.Work, error)
}

type defaultMerger struct{}

func NewMerger() Merger {
	return &defaultMerger{}
}

func (m *defaultMerger) MergeWorks(results [][]model.Work) ([]model.Work, error) {
	seen := make(map[string]int)
	var merged []model.Work

	for _, batch := range results {
		for _, w := range batch {
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

			if idx, exists := seen[fp]; exists {
				// merge editions into existing work
				merged[idx].Editions = append(merged[idx].Editions, w.Editions...)
				if merged[idx].Confidence < w.Confidence {
					merged[idx].Confidence = w.Confidence
				}
			} else {
				seen[fp] = len(merged)
				merged = append(merged, w)
			}
		}
	}

	// score confidence based on completeness
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
