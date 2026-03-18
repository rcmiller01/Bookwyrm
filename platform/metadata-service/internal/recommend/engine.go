package recommend

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"metadata-service/internal/cache"
	"metadata-service/internal/metrics"
	"metadata-service/internal/model"
	"metadata-service/internal/store"

	"github.com/rs/zerolog/log"
)

type Options struct {
	Weights            ScoringWeights
	MaxDepth           int
	MaxCandidatePool   int
	SeriesLimit        int
	AuthorLimit        int
	MaxSubjects        int
	MaxWorksPerSubject int
	RelationshipLimit  int
	CacheTTL           time.Duration
}

func DefaultOptions() Options {
	return Options{
		Weights:            DefaultWeights(),
		MaxDepth:           2,
		MaxCandidatePool:   250,
		SeriesLimit:        10,
		AuthorLimit:        25,
		MaxSubjects:        10,
		MaxWorksPerSubject: 10,
		RelationshipLimit:  100,
		CacheTTL:           2 * time.Hour,
	}
}

type Engine struct {
	store store.RecommendReadStore
	cache cache.Cache
	opts  Options
}

func NewEngine(readStore store.RecommendReadStore, cacheStore cache.Cache, opts Options) *Engine {
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 2
	}
	if opts.MaxCandidatePool <= 0 {
		opts.MaxCandidatePool = 250
	}
	if opts.SeriesLimit <= 0 {
		opts.SeriesLimit = 10
	}
	if opts.AuthorLimit <= 0 {
		opts.AuthorLimit = 25
	}
	if opts.MaxSubjects <= 0 {
		opts.MaxSubjects = 10
	}
	if opts.MaxWorksPerSubject <= 0 {
		opts.MaxWorksPerSubject = 10
	}
	if opts.RelationshipLimit <= 0 {
		opts.RelationshipLimit = 100
	}
	if opts.CacheTTL <= 0 {
		opts.CacheTTL = 2 * time.Hour
	}
	if opts.Weights == (ScoringWeights{}) {
		opts.Weights = DefaultWeights()
	}
	return &Engine{store: readStore, cache: cacheStore, opts: opts}
}

type candidateScore struct {
	workID  string
	score   float64
	reasons []Reason
}

func (e *Engine) Recommend(ctx context.Context, req RecommendationRequest) ([]RecommendationResult, error) {
	started := time.Now()
	metrics.RecommendRequestsTotal.Inc()
	defer func() {
		metrics.RecommendLatencySeconds.Observe(time.Since(started).Seconds())
	}()

	if len(req.SeedWorkIDs) == 0 {
		return []RecommendationResult{}, nil
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	if req.MaxCandidates <= 0 {
		req.MaxCandidates = e.opts.MaxCandidatePool
	}

	cacheKey := recommendationCacheKey(req)
	if cached, ok := e.getFromCache(cacheKey); ok {
		metrics.RecommendCacheHitsTotal.Inc()
		log.Info().
			Strs("seed_work_ids", req.SeedWorkIDs).
			Int("limit", req.Limit).
			Bool("cache_hit", true).
			Dur("duration", time.Since(started)).
			Msg("recommendation request served")
		return cached, nil
	}
	metrics.RecommendCacheMissesTotal.Inc()

	include := includeSet(req.IncludeTypes)
	exclude := excludeSet(req.ExcludeIDs, req.SeedWorkIDs)
	candidateMap := map[string]*candidateScore{}

	for _, seedID := range req.SeedWorkIDs {
		if strings.TrimSpace(seedID) == "" {
			continue
		}

		if include["relationships"] {
			relationships, err := e.store.GetRelationshipCandidatesForWork(ctx, seedID, e.opts.RelationshipLimit)
			if err != nil {
				return nil, err
			}
			for _, rel := range relationships {
				if exclude[rel.WorkID] {
					continue
				}
				base := e.opts.Weights.ExplicitRelated
				if rel.Confidence > 0 {
					base = base * rel.Confidence
				}
				e.addCandidate(candidateMap, rel.WorkID, base, Reason{
					Type:   "explicit_related",
					Weight: normalizeScore(base),
					Evidence: map[string]any{
						"relationship_type": rel.RelationshipType,
					},
				})
			}
		}

		if include["series"] {
			prev, next, err := e.store.GetSeriesNeighborsForWork(ctx, seedID)
			if err != nil {
				return nil, err
			}
			if prev != nil && !exclude[prev.WorkID] {
				weight := e.opts.Weights.SeriesNeighbor - distancePenalty(prev.Delta)
				e.addCandidate(candidateMap, prev.WorkID, weight, Reason{
					Type:   "series_neighbor",
					Weight: normalizeScore(weight),
					Evidence: map[string]any{
						"series": prev.SeriesName,
						"delta":  prev.Delta,
					},
				})
			}
			if next != nil && !exclude[next.WorkID] {
				weight := e.opts.Weights.SeriesNeighbor - distancePenalty(next.Delta)
				e.addCandidate(candidateMap, next.WorkID, weight, Reason{
					Type:   "series_neighbor",
					Weight: normalizeScore(weight),
					Evidence: map[string]any{
						"series": next.SeriesName,
						"delta":  next.Delta,
					},
				})
			}

			seriesCandidates, err := e.store.GetSeriesCandidatesForWork(ctx, seedID, e.opts.SeriesLimit)
			if err != nil {
				return nil, err
			}
			for _, candidate := range seriesCandidates {
				if exclude[candidate.WorkID] {
					continue
				}
				weight := e.opts.Weights.SameSeries - distancePenalty(candidate.Delta)
				e.addCandidate(candidateMap, candidate.WorkID, weight, Reason{
					Type:   "same_series",
					Weight: normalizeScore(weight),
					Evidence: map[string]any{
						"series": candidate.SeriesName,
						"delta":  candidate.Delta,
					},
				})
			}
		}

		if include["author"] {
			authorCandidates, err := e.store.GetAuthorCandidatesForWork(ctx, seedID, e.opts.AuthorLimit)
			if err != nil {
				return nil, err
			}
			for _, candidate := range authorCandidates {
				if exclude[candidate.WorkID] {
					continue
				}
				e.addCandidate(candidateMap, candidate.WorkID, e.opts.Weights.SameAuthor, Reason{
					Type:   "same_author",
					Weight: normalizeScore(e.opts.Weights.SameAuthor),
					Evidence: map[string]any{
						"authors": candidate.SharedAuthors,
					},
				})
			}
		}

		if include["subjects"] {
			subjectCandidates, err := e.store.GetSubjectCandidatesForWork(ctx, seedID, e.opts.MaxSubjects, e.opts.MaxWorksPerSubject)
			if err != nil {
				return nil, err
			}
			for _, candidate := range subjectCandidates {
				if exclude[candidate.WorkID] {
					continue
				}
				weight := subjectContribution(e.opts.Weights.SharedSubject, candidate.OverlapRatio)
				e.addCandidate(candidateMap, candidate.WorkID, weight, Reason{
					Type:   "shared_subject",
					Weight: normalizeScore(weight),
					Evidence: map[string]any{
						"subjects":      candidate.SharedSubjects,
						"overlap_ratio": candidate.OverlapRatio,
					},
				})
			}
		}
	}

	candidateList := make([]candidateScore, 0, len(candidateMap))
	for _, candidate := range candidateMap {
		candidate.score = normalizeScore(candidate.score)
		candidateList = append(candidateList, *candidate)
	}

	sort.SliceStable(candidateList, func(i, j int) bool {
		if candidateList[i].score == candidateList[j].score {
			return candidateList[i].workID < candidateList[j].workID
		}
		return candidateList[i].score > candidateList[j].score
	})
	if len(candidateList) > req.MaxCandidates {
		candidateList = candidateList[:req.MaxCandidates]
	}
	metrics.RecommendCandidatesGeneratedTotal.Add(float64(len(candidateList)))

	candidateIDs := make([]string, 0, len(candidateList))
	for _, candidate := range candidateList {
		candidateIDs = append(candidateIDs, candidate.workID)
	}

	works, err := e.store.GetWorksByIDs(ctx, candidateIDs)
	if err != nil {
		return nil, err
	}
	workMap := make(map[string]model.Work, len(works))
	for _, work := range works {
		workMap[work.ID] = work
	}

	if len(req.Preferences.Formats) > 0 || len(req.Preferences.Languages) > 0 {
		editionsByWork, editionErr := e.store.GetEditionsByWorkIDs(ctx, candidateIDs)
		if editionErr == nil {
			e.applyPreferenceBoost(candidateList, editionsByWork, req.Preferences)
		}
	}

	results := make([]RecommendationResult, 0, req.Limit)
	for _, candidate := range candidateList {
		workData, ok := workMap[candidate.workID]
		if !ok {
			continue
		}
		if len(candidate.reasons) == 0 {
			continue
		}
		results = append(results, RecommendationResult{
			Work:    workData,
			Score:   normalizeScore(candidate.score),
			Reasons: candidate.reasons,
		})
		if len(results) >= req.Limit {
			break
		}
	}
	metrics.RecommendResultsReturnedTotal.Add(float64(len(results)))
	e.setCache(cacheKey, results)

	log.Info().
		Strs("seed_work_ids", req.SeedWorkIDs).
		Int("limit", req.Limit).
		Bool("cache_hit", false).
		Int("candidate_count", len(candidateList)).
		Dur("duration", time.Since(started)).
		Msg("recommendation request served")

	return results, nil
}

func (e *Engine) RecommendNextInSeries(ctx context.Context, workID string) (*RecommendationResult, error) {
	nextReq := RecommendationRequest{
		SeedWorkIDs:  []string{workID},
		Limit:        5,
		IncludeTypes: []string{"series"},
	}
	results, err := e.Recommend(ctx, nextReq)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		for _, reason := range result.Reasons {
			if reason.Type == "series_neighbor" {
				copyResult := result
				return &copyResult, nil
			}
		}
	}
	return nil, nil
}

func (e *Engine) RecommendSimilar(ctx context.Context, workID string, limit int, preferences RecommendationPreferences) ([]RecommendationResult, error) {
	similarReq := RecommendationRequest{
		SeedWorkIDs:  []string{workID},
		Limit:        limit,
		IncludeTypes: []string{"subjects", "author", "relationships"},
		Preferences:  preferences,
	}
	return e.Recommend(ctx, similarReq)
}

func (e *Engine) addCandidate(candidateMap map[string]*candidateScore, workID string, score float64, reason Reason) {
	if strings.TrimSpace(workID) == "" || score <= 0 {
		return
	}
	candidate, ok := candidateMap[workID]
	if !ok {
		candidate = &candidateScore{workID: workID, reasons: make([]Reason, 0, 4)}
		candidateMap[workID] = candidate
	}
	candidate.score += score
	candidate.reasons = append(candidate.reasons, reason)
}

func includeSet(values []string) map[string]bool {
	defaults := map[string]bool{"series": true, "author": true, "subjects": true, "relationships": true}
	if len(values) == 0 {
		return defaults
	}
	out := map[string]bool{"series": false, "author": false, "subjects": false, "relationships": false}
	for _, value := range values {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "series":
			out["series"] = true
		case "author":
			out["author"] = true
		case "subjects":
			out["subjects"] = true
		case "relationships", "explicit", "explicit_related":
			out["relationships"] = true
		}
	}
	return out
}

func excludeSet(excludeIDs []string, seedIDs []string) map[string]bool {
	out := map[string]bool{}
	for _, id := range excludeIDs {
		value := strings.TrimSpace(id)
		if value != "" {
			out[value] = true
		}
	}
	for _, id := range seedIDs {
		value := strings.TrimSpace(id)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func (e *Engine) applyPreferenceBoost(candidateList []candidateScore, editionsByWork map[string][]model.Edition, preferences RecommendationPreferences) {
	formatSet := map[string]struct{}{}
	for _, format := range preferences.Formats {
		value := strings.ToLower(strings.TrimSpace(format))
		if value != "" {
			formatSet[value] = struct{}{}
		}
	}
	for i := range candidateList {
		editions := editionsByWork[candidateList[i].workID]
		if len(editions) == 0 {
			continue
		}
		matched := false
		matchedFormats := make([]string, 0, 2)
		for _, edition := range editions {
			format := strings.ToLower(strings.TrimSpace(edition.Format))
			if _, ok := formatSet[format]; ok {
				matched = true
				matchedFormats = append(matchedFormats, format)
			}
		}
		if matched {
			candidateList[i].score += e.opts.Weights.PreferenceBoost
			candidateList[i].reasons = append(candidateList[i].reasons, Reason{
				Type:   "preference_match",
				Weight: e.opts.Weights.PreferenceBoost,
				Evidence: map[string]any{
					"formats": uniqueStrings(matchedFormats),
				},
			})
		}
	}
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func recommendationCacheKey(req RecommendationRequest) string {
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	payload := map[string]any{
		"seed":      sortedNormalized(req.SeedWorkIDs),
		"limit":     limit,
		"include":   sortedNormalized(req.IncludeTypes),
		"exclude":   sortedNormalized(req.ExcludeIDs),
		"formats":   sortedNormalized(req.Preferences.Formats),
		"languages": sortedNormalized(req.Preferences.Languages),
	}
	encoded, _ := json.Marshal(payload)
	sum := sha1.Sum(encoded)
	return "rec:" + hex.EncodeToString(sum[:])
}

func sortedNormalized(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func (e *Engine) getFromCache(key string) ([]RecommendationResult, bool) {
	if e.cache == nil {
		return nil, false
	}
	value, ok := e.cache.Get(key)
	if !ok {
		return nil, false
	}
	results, ok := value.([]RecommendationResult)
	if !ok {
		return nil, false
	}
	return results, true
}

func (e *Engine) setCache(key string, results []RecommendationResult) {
	if e.cache == nil {
		return
	}
	e.cache.Set(key, results, e.opts.CacheTTL)
}
