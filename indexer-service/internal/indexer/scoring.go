package indexer

import (
	"sort"
	"strings"
)

func ApplyScoring(candidates []Candidate, backendMap map[string]BackendRecord, query QuerySpec) []Candidate {
	out := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		backend := backendMap[c.SourceBackendID]
		score := 0.30 + (backend.ReliabilityScore * 0.50)
		reasons := []Reason{
			{Code: "backend_reliability", Weight: backend.ReliabilityScore},
		}
		if query.ISBN != "" {
			if hasIdentifier(c, query.ISBN) {
				score += 0.15
				reasons = append(reasons, Reason{Code: "identifier_match", Weight: 0.15})
			}
		}
		if query.DOI != "" {
			if hasIdentifier(c, query.DOI) {
				score += 0.15
				reasons = append(reasons, Reason{Code: "identifier_match", Weight: 0.15})
			}
		}
		if strings.Contains(strings.ToLower(c.Title), strings.ToLower(query.Title)) && strings.TrimSpace(query.Title) != "" {
			score += 0.05
			reasons = append(reasons, Reason{Code: "title_overlap", Weight: 0.05})
		}
		if c.Seeders != nil && *c.Seeders > 0 {
			score += 0.05
			reasons = append(reasons, Reason{Code: "seeders_present", Weight: 0.05})
		}
		if score > 0.99 {
			score = 0.99
		}
		c.Score = score
		c.MatchConfidence = score
		c.Reasons = reasons
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].CandidateID < out[j].CandidateID
		}
		return out[i].Score > out[j].Score
	})
	return out
}

func DedupeCandidates(candidates []Candidate) []Candidate {
	seen := map[string]struct{}{}
	out := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		key := strings.ToLower(strings.TrimSpace(c.Title)) + "|" + strings.ToLower(strings.TrimSpace(c.Protocol))
		if c.SizeBytes != nil {
			key += "|" + itoa64(*c.SizeBytes)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}

func hasIdentifier(c Candidate, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	for _, v := range c.Identifiers {
		if strings.ToLower(strings.TrimSpace(toString(v))) == target {
			return true
		}
	}
	return false
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append([]byte{byte('0' + (v % 10))}, buf...)
		v /= 10
	}
	return sign + string(buf)
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}
