package pipeline

import (
	"app-backend/internal/domain/contract"
	"app-backend/internal/integration/metadata"
)

type ImporterPipeline struct {
	domain contract.Domain
}

func NewImporterPipeline(domain contract.Domain) *ImporterPipeline {
	return &ImporterPipeline{domain: domain}
}

func (p *ImporterPipeline) BuildSearchSpec(
	snapshot metadata.MetadataSnapshot,
	requestedCapabilities []string,
	priority string,
	policyProfile string,
	backendGroups []string,
) contract.QuerySpec {
	if p == nil || p.domain == nil {
		return contract.QuerySpec{}
	}

	return p.domain.QueryBuilder().BuildSearch(snapshot.WorkID, contract.Preferences{
		Snapshot: contract.MetadataSnapshot{
			WorkID:          snapshot.WorkID,
			EditionID:       snapshot.EditionID,
			ISBN10:          snapshot.ISBN10,
			ISBN13:          snapshot.ISBN13,
			Title:           snapshot.Title,
			Authors:         snapshot.Authors,
			Language:        snapshot.Language,
			PublicationYear: snapshot.PublicationYear,
		},
		RequestedCapabilities: requestedCapabilities,
		Priority:              priority,
		PolicyProfile:         policyProfile,
		BackendGroups:         backendGroups,
	})
}

func (p *ImporterPipeline) GroupFiles(files []string) []contract.FileGroup {
	if p == nil || p.domain == nil {
		return []contract.FileGroup{}
	}
	return p.domain.ImportRules().GroupFiles(files)
}
