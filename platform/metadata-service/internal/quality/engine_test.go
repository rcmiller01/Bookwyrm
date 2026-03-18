package quality

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	series          []SeriesAnomaly
	conflicts       []PublicationYearConflict
	duplicates      []DuplicateEdition
	candidates      []IdentifierCandidate
	repairSeries    map[string]int
	syncWork        map[string]bool
	removedIDs      map[string]bool
	errorOnDetect   error
	errorOnRepairID string
}

func (f *fakeRepository) DetectSeriesAnomalies(_ context.Context, _ int) ([]SeriesAnomaly, error) {
	if f.errorOnDetect != nil {
		return nil, f.errorOnDetect
	}
	return f.series, nil
}

func (f *fakeRepository) DetectPublicationYearConflicts(_ context.Context, _ int) ([]PublicationYearConflict, error) {
	if f.errorOnDetect != nil {
		return nil, f.errorOnDetect
	}
	return f.conflicts, nil
}

func (f *fakeRepository) DetectDuplicateEditions(_ context.Context, _ int) ([]DuplicateEdition, error) {
	if f.errorOnDetect != nil {
		return nil, f.errorOnDetect
	}
	return f.duplicates, nil
}

func (f *fakeRepository) ListIdentifierCandidates(_ context.Context, _ int) ([]IdentifierCandidate, error) {
	if f.errorOnDetect != nil {
		return nil, f.errorOnDetect
	}
	return f.candidates, nil
}

func (f *fakeRepository) RepairSeriesOrder(_ context.Context, seriesID string) (int, error) {
	if f.errorOnRepairID == seriesID {
		return 0, errors.New("repair series failed")
	}
	return f.repairSeries[seriesID], nil
}

func (f *fakeRepository) SyncWorkFirstPublicationYear(_ context.Context, workID string) (bool, error) {
	return f.syncWork[workID], nil
}

func (f *fakeRepository) RemoveIdentifier(_ context.Context, id IdentifierCandidate) (bool, error) {
	key := id.EditionID + "|" + id.Type + "|" + id.Value
	return f.removedIDs[key], nil
}

func TestEngineAuditAndRepair(t *testing.T) {
	repo := &fakeRepository{
		series: []SeriesAnomaly{
			{SeriesID: "s1"},
			{SeriesID: "s1"},
			{SeriesID: "s2"},
		},
		conflicts: []PublicationYearConflict{
			{WorkID: "w1"},
			{WorkID: "w2"},
		},
		duplicates: []DuplicateEdition{{WorkID: "w1"}},
		candidates: []IdentifierCandidate{
			{EditionID: "e1", Type: "ISBN_13", Value: "9780306406158"},
			{EditionID: "e2", Type: "ISBN_10", Value: "0306406152"},
		},
		repairSeries: map[string]int{"s1": 2, "s2": 0},
		syncWork:     map[string]bool{"w1": true, "w2": false},
		removedIDs:   map[string]bool{"e1|ISBN_13|9780306406158": true},
	}

	engine := NewEngine(repo)
	report, err := engine.Audit(context.Background(), 10)
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if len(report.InvalidIdentifiers) != 2 {
		t.Fatalf("expected 2 invalid identifiers, got %d", len(report.InvalidIdentifiers))
	}

	result, err := engine.Repair(context.Background(), RepairRequest{Limit: 10, DryRun: false, RemoveInvalidIdentifiers: true})
	if err != nil {
		t.Fatalf("repair failed: %v", err)
	}
	if result.SeriesReordered != 1 {
		t.Fatalf("expected 1 reordered series, got %d", result.SeriesReordered)
	}
	if result.PublicationYearsNormalized != 1 {
		t.Fatalf("expected 1 normalized work publication year, got %d", result.PublicationYearsNormalized)
	}
	if result.InvalidIdentifiersRemoved != 1 {
		t.Fatalf("expected 1 removed identifier, got %d", result.InvalidIdentifiersRemoved)
	}
}

func TestEngineDryRun(t *testing.T) {
	repo := &fakeRepository{
		series:     []SeriesAnomaly{{SeriesID: "s1"}},
		conflicts:  []PublicationYearConflict{{WorkID: "w1"}},
		candidates: []IdentifierCandidate{{EditionID: "e1", Type: "ISBN_13", Value: "invalid"}},
	}
	engine := NewEngine(repo)
	result, err := engine.Repair(context.Background(), RepairRequest{Limit: 10, DryRun: true, RemoveInvalidIdentifiers: true})
	if err != nil {
		t.Fatalf("dry-run repair failed: %v", err)
	}
	if !result.DryRun {
		t.Fatalf("expected dry-run result")
	}
	if result.SeriesReordered != 0 || result.PublicationYearsNormalized != 0 || result.InvalidIdentifiersRemoved != 0 {
		t.Fatalf("expected no mutations in dry-run, got %+v", result)
	}
}
