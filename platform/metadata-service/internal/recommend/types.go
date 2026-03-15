package recommend

import "metadata-service/internal/model"

type RecommendationPreferences struct {
	Formats   []string `json:"formats,omitempty"`
	Languages []string `json:"languages,omitempty"`
}

type RecommendationRequest struct {
	SeedWorkIDs   []string                  `json:"seed_work_ids"`
	Limit         int                       `json:"limit"`
	IncludeTypes  []string                  `json:"include_types,omitempty"`
	ExcludeIDs    []string                  `json:"exclude_ids,omitempty"`
	Preferences   RecommendationPreferences `json:"preferences,omitempty"`
	MaxDepth      int                       `json:"max_depth,omitempty"`
	MaxCandidates int                       `json:"max_candidates,omitempty"`
}

type Reason struct {
	Type     string         `json:"type"`
	Weight   float64        `json:"weight"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

type RecommendationResult struct {
	Work    model.Work `json:"work"`
	Score   float64    `json:"score"`
	Reasons []Reason   `json:"reasons"`
}
