package indexer

import "strings"

type EntityRef struct {
	Type      string
	ID        string
	ParentIDs map[string]string
}

func (m MetadataSnapshot) NormalizeEntityRef() EntityRef {
	entityType := strings.ToLower(strings.TrimSpace(m.EntityType))
	entityID := strings.TrimSpace(m.EntityID)

	if entityType == "" {
		switch {
		case strings.TrimSpace(m.WorkID) != "":
			entityType = "work"
			entityID = strings.TrimSpace(m.WorkID)
		case strings.TrimSpace(m.EditionID) != "":
			entityType = "edition"
			entityID = strings.TrimSpace(m.EditionID)
		}
	}

	parents := map[string]string{}
	if workID := strings.TrimSpace(m.WorkID); workID != "" {
		parents["work"] = workID
	}
	if editionID := strings.TrimSpace(m.EditionID); editionID != "" {
		parents["edition"] = editionID
	}

	if entityType == "work" && entityID != "" {
		parents["work"] = entityID
	}
	if entityType == "edition" && entityID != "" {
		parents["edition"] = entityID
	}

	return EntityRef{Type: entityType, ID: entityID, ParentIDs: parents}
}
