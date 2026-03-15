package books

import (
	"strings"

	"app-backend/internal/domain/contract"
)

type queryBuilder struct{}

func (q queryBuilder) BuildSearch(workID string, prefs contract.Preferences) contract.QuerySpec {
	s := prefs.Snapshot
	resolvedWorkID := strings.TrimSpace(workID)
	if resolvedWorkID == "" {
		resolvedWorkID = strings.TrimSpace(s.WorkID)
	}

	return contract.QuerySpec{
		Metadata: map[string]any{
			"work_id":          resolvedWorkID,
			"edition_id":       s.EditionID,
			"entity_type":      "work",
			"entity_id":        resolvedWorkID,
			"isbn_10":          s.ISBN10,
			"isbn_13":          s.ISBN13,
			"title":            s.Title,
			"authors":          s.Authors,
			"language":         s.Language,
			"publication_year": s.PublicationYear,
		},
		RequestedCapabilities: prefs.RequestedCapabilities,
		Priority:              prefs.Priority,
		PolicyProfile:         prefs.PolicyProfile,
		BackendGroups:         prefs.BackendGroups,
	}
}
