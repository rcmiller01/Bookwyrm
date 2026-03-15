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
		if pref, ok := backend.Config["preferred"]; ok {
			if preferred, isBool := pref.(bool); isBool && preferred {
				score += 0.10
				reasons = append(reasons, Reason{Code: "preferred_source", Weight: 0.10})
			}
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
		key := ReleaseFingerprint(c)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}

// ReleaseFingerprint builds a composite dedup key for a candidate.
// The key combines: normalized title, protocol, MB-level size bucket,
// and sorted identifiers (ISBN, DOI, etc.). Two candidates with the same
// ISBN but different titles will share a fingerprint and be deduped.
func ReleaseFingerprint(c Candidate) string {
	title := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(c.Title)), " "))
	protocol := strings.ToLower(strings.TrimSpace(c.Protocol))

	sizePart := ""
	if c.SizeBytes != nil {
		bucket := *c.SizeBytes / (1024 * 1024) // MB-level bucket
		sizePart = itoa64(bucket)
	}

	idPart := sortedIdentifiers(c.Identifiers)

	// If identifiers are present, they dominate the key (two releases with
	// the same ISBN are the same release even if titles differ slightly).
	if idPart != "" {
		return idPart + "|" + protocol + "|" + sizePart
	}
	return title + "|" + protocol + "|" + sizePart
}

func sortedIdentifiers(ids map[string]any) string {
	if len(ids) == 0 {
		return ""
	}
	// Collect all non-empty identifier values.
	vals := make([]string, 0, len(ids))
	for _, v := range ids {
		s := strings.ToLower(strings.TrimSpace(toString(v)))
		if s != "" {
			vals = append(vals, s)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	sort.Strings(vals)
	return strings.Join(vals, ";")
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
