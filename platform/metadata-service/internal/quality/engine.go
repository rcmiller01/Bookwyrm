package quality

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type qualityEngine struct {
	repo Repository
}

func NewEngine(repo Repository) Engine {
	return &qualityEngine{repo: repo}
}

func (e *qualityEngine) Audit(ctx context.Context, limit int) (*AuditReport, error) {
	if limit <= 0 {
		limit = 25
	}

	seriesAnomalies, err := e.repo.DetectSeriesAnomalies(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("detect series anomalies: %w", err)
	}
	publicationConflicts, err := e.repo.DetectPublicationYearConflicts(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("detect publication year conflicts: %w", err)
	}
	duplicateEditions, err := e.repo.DetectDuplicateEditions(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("detect duplicate editions: %w", err)
	}

	candidateLimit := limit * 8
	if candidateLimit < 200 {
		candidateLimit = 200
	}
	identifierCandidates, err := e.repo.ListIdentifierCandidates(ctx, candidateLimit)
	if err != nil {
		return nil, fmt.Errorf("list identifier candidates: %w", err)
	}
	invalidIdentifiers := make([]InvalidIdentifier, 0)
	for _, candidate := range identifierCandidates {
		normalizedType := strings.ToUpper(strings.TrimSpace(candidate.Type))
		if normalizedType == "ISBN_10" || normalizedType == "ISBN10" {
			invalidIdentifiers = append(invalidIdentifiers, InvalidIdentifier{
				EditionID: candidate.EditionID,
				Type:      candidate.Type,
				Value:     candidate.Value,
				Reason:    "non-canonical ISBN-10; prefer ISBN-13",
			})
			if len(invalidIdentifiers) >= limit {
				break
			}
			continue
		}
		valid, reason := VerifyIdentifier(candidate.Type, candidate.Value)
		if valid {
			continue
		}
		invalidIdentifiers = append(invalidIdentifiers, InvalidIdentifier{
			EditionID: candidate.EditionID,
			Type:      candidate.Type,
			Value:     candidate.Value,
			Reason:    reason,
		})
		if len(invalidIdentifiers) >= limit {
			break
		}
	}

	return &AuditReport{
		GeneratedAt:              time.Now().UTC(),
		SeriesAnomalies:          seriesAnomalies,
		PublicationYearConflicts: publicationConflicts,
		DuplicateEditions:        duplicateEditions,
		InvalidIdentifiers:       invalidIdentifiers,
	}, nil
}

func (e *qualityEngine) Repair(ctx context.Context, req RepairRequest) (*RepairResult, error) {
	if req.Limit <= 0 {
		req.Limit = 25
	}

	report, err := e.Audit(ctx, req.Limit)
	if err != nil {
		return nil, err
	}

	result := &RepairResult{
		DryRun:                     req.DryRun,
		ExaminedSeriesAnomalies:    len(report.SeriesAnomalies),
		ExaminedYearConflicts:      len(report.PublicationYearConflicts),
		ExaminedInvalidIdentifiers: len(report.InvalidIdentifiers),
	}
	if req.DryRun {
		return result, nil
	}

	seenSeries := make(map[string]struct{})
	for _, anomaly := range report.SeriesAnomalies {
		if _, exists := seenSeries[anomaly.SeriesID]; exists {
			continue
		}
		seenSeries[anomaly.SeriesID] = struct{}{}
		updated, err := e.repo.RepairSeriesOrder(ctx, anomaly.SeriesID)
		if err != nil {
			return nil, fmt.Errorf("repair series %s: %w", anomaly.SeriesID, err)
		}
		if updated > 0 {
			result.SeriesReordered++
		}
	}

	for _, conflict := range report.PublicationYearConflicts {
		changed, err := e.repo.SyncWorkFirstPublicationYear(ctx, conflict.WorkID)
		if err != nil {
			return nil, fmt.Errorf("normalize publication year for %s: %w", conflict.WorkID, err)
		}
		if changed {
			result.PublicationYearsNormalized++
		}
	}

	if req.RemoveInvalidIdentifiers {
		for _, invalid := range report.InvalidIdentifiers {
			removed, err := e.repo.RemoveIdentifier(ctx, IdentifierCandidate{
				EditionID: invalid.EditionID,
				Type:      invalid.Type,
				Value:     invalid.Value,
			})
			if err != nil {
				return nil, fmt.Errorf("remove invalid identifier %s:%s: %w", invalid.Type, invalid.Value, err)
			}
			if removed {
				result.InvalidIdentifiersRemoved++
			}
		}
	}

	return result, nil
}
