package resolver

import (
	"sort"
	"strings"

	"metadata-service/internal/provider"
)

// ApplyRoutingBias applies request-scoped provider ordering boosts after the
// registry has already enforced reliability tiers and quarantine policy.
func ApplyRoutingBias(cq ClassifiedQuery, providers []provider.Provider) []provider.Provider {
	if len(providers) <= 1 {
		return providers
	}

	boostFor := func(p provider.Provider) int {
		caps := provider.CapabilitiesFor(p)
		switch cq.Type {
		case QueryTypeDOI:
			if caps.SupportsDOI {
				return 100
			}
		case QueryTypeISBN10, QueryTypeISBN13:
			if caps.SupportsISBN {
				if p.Name() == "openlibrary" || p.Name() == "googlebooks" {
					return 100
				}
				return 50
			}
		default:
			if caps.SupportsSearch {
				if p.Name() == "openlibrary" || p.Name() == "googlebooks" || p.Name() == "hardcover" {
					return 80
				}
				if caps.SupportsAuthorSearch || looksLikeAuthorTitleQuery(cq.Normalized) {
					return 40
				}
			}
		}
		return 0
	}

	type entry struct {
		p     provider.Provider
		boost int
	}
	entries := make([]entry, 0, len(providers))
	for _, p := range providers {
		entries = append(entries, entry{p: p, boost: boostFor(p)})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].boost > entries[j].boost
	})

	out := make([]provider.Provider, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.p)
	}
	return out
}

func looksLikeAuthorTitleQuery(normalized string) bool {
	return len(strings.Fields(strings.TrimSpace(normalized))) >= 2
}
