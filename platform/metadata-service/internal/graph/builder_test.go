package graph

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"metadata-service/internal/model"
)

type fakeBuilderWorkStore struct {
	work model.Work
}

func (f *fakeBuilderWorkStore) GetWorkByID(_ context.Context, _ string) (*model.Work, error) {
	cp := f.work
	return &cp, nil
}
func (f *fakeBuilderWorkStore) SearchWorks(_ context.Context, _ string) ([]model.Work, error) {
	return nil, nil
}
func (f *fakeBuilderWorkStore) InsertWork(_ context.Context, _ model.Work) error { return nil }
func (f *fakeBuilderWorkStore) UpdateWork(_ context.Context, _ model.Work) error { return nil }
func (f *fakeBuilderWorkStore) GetWorkByFingerprint(_ context.Context, _ string) (*model.Work, error) {
	return nil, nil
}

type fakeBuilderSeriesStore struct {
	seriesByID   map[string]model.Series
	entriesBySID map[string][]model.SeriesEntry
	seriesByWork map[string]string
}

func newFakeBuilderSeriesStore() *fakeBuilderSeriesStore {
	return &fakeBuilderSeriesStore{
		seriesByID:   map[string]model.Series{},
		entriesBySID: map[string][]model.SeriesEntry{},
		seriesByWork: map[string]string{},
	}
}

func (f *fakeBuilderSeriesStore) UpsertSeries(_ context.Context, name string, normalized string) (string, error) {
	id := "series:" + normalized
	f.seriesByID[id] = model.Series{ID: id, Name: name, NormalizedName: normalized, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	return id, nil
}

func (f *fakeBuilderSeriesStore) UpsertSeriesEntry(_ context.Context, seriesID string, workID string, seriesIndex *float64) error {
	entry := model.SeriesEntry{SeriesID: seriesID, WorkID: workID, SeriesIndex: seriesIndex}
	entries := f.entriesBySID[seriesID]
	filtered := entries[:0]
	for _, existing := range entries {
		if existing.WorkID != workID {
			filtered = append(filtered, existing)
		}
	}
	f.entriesBySID[seriesID] = append(filtered, entry)
	f.seriesByWork[workID] = seriesID
	return nil
}

func (f *fakeBuilderSeriesStore) GetSeriesByID(_ context.Context, id string) (*model.Series, error) {
	series, ok := f.seriesByID[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := series
	return &cp, nil
}

func (f *fakeBuilderSeriesStore) GetSeriesEntries(_ context.Context, seriesID string) ([]model.SeriesEntry, error) {
	return append([]model.SeriesEntry{}, f.entriesBySID[seriesID]...), nil
}

func (f *fakeBuilderSeriesStore) GetSeriesForWork(_ context.Context, workID string) (*model.Series, error) {
	seriesID := f.seriesByWork[workID]
	if seriesID == "" {
		return nil, nil
	}
	series, ok := f.seriesByID[seriesID]
	if !ok {
		return nil, nil
	}
	cp := series
	return &cp, nil
}

func (f *fakeBuilderSeriesStore) CountSeries(_ context.Context) (int64, error) {
	return int64(len(f.seriesByID)), nil
}

type fakeBuilderSubjectStore struct {
	subjectByID      map[string]model.Subject
	normalizedToID   map[string]string
	workToSubjectIDs map[string][]string
	worksBySubjectID map[string][]model.Work
}

func newFakeBuilderSubjectStore() *fakeBuilderSubjectStore {
	return &fakeBuilderSubjectStore{
		subjectByID:      map[string]model.Subject{},
		normalizedToID:   map[string]string{},
		workToSubjectIDs: map[string][]string{},
		worksBySubjectID: map[string][]model.Work{},
	}
}

func (f *fakeBuilderSubjectStore) UpsertSubject(_ context.Context, name string, normalized string) (string, error) {
	if id, ok := f.normalizedToID[normalized]; ok {
		f.subjectByID[id] = model.Subject{ID: id, Name: name, NormalizedName: normalized}
		return id, nil
	}
	id := "subject:" + normalized
	f.normalizedToID[normalized] = id
	f.subjectByID[id] = model.Subject{ID: id, Name: name, NormalizedName: normalized}
	return id, nil
}

func (f *fakeBuilderSubjectStore) SetWorkSubjects(_ context.Context, workID string, subjectIDs []string) error {
	f.workToSubjectIDs[workID] = append([]string{}, subjectIDs...)
	return nil
}

func (f *fakeBuilderSubjectStore) GetSubjectByID(_ context.Context, subjectID string) (*model.Subject, error) {
	subject := f.subjectByID[subjectID]
	cp := subject
	return &cp, nil
}

func (f *fakeBuilderSubjectStore) GetSubjectsForWork(_ context.Context, workID string) ([]model.Subject, error) {
	ids := f.workToSubjectIDs[workID]
	subjects := make([]model.Subject, 0, len(ids))
	for _, id := range ids {
		subjects = append(subjects, f.subjectByID[id])
	}
	return subjects, nil
}

func (f *fakeBuilderSubjectStore) GetWorksForSubject(_ context.Context, subjectID string, limit int, offset int) ([]model.Work, error) {
	works := f.worksBySubjectID[subjectID]
	if offset >= len(works) {
		return nil, nil
	}
	end := len(works)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return append([]model.Work{}, works[offset:end]...), nil
}

func (f *fakeBuilderSubjectStore) CountSubjects(_ context.Context) (int64, error) {
	return int64(len(f.subjectByID)), nil
}

type fakeBuilderWorkRelStore struct {
	rels map[string]map[string]map[string]model.WorkRelationship
}

func newFakeBuilderWorkRelStore() *fakeBuilderWorkRelStore {
	return &fakeBuilderWorkRelStore{rels: map[string]map[string]map[string]model.WorkRelationship{}}
}

func (f *fakeBuilderWorkRelStore) UpsertRelationship(_ context.Context, sourceID string, targetID string, relationshipType string, confidence float64, provider *string) error {
	if _, ok := f.rels[sourceID]; !ok {
		f.rels[sourceID] = map[string]map[string]model.WorkRelationship{}
	}
	if _, ok := f.rels[sourceID][relationshipType]; !ok {
		f.rels[sourceID][relationshipType] = map[string]model.WorkRelationship{}
	}
	f.rels[sourceID][relationshipType][targetID] = model.WorkRelationship{
		SourceWorkID:     sourceID,
		TargetWorkID:     targetID,
		RelationshipType: relationshipType,
		Confidence:       confidence,
		Provider:         provider,
	}
	return nil
}

func (f *fakeBuilderWorkRelStore) GetRelatedWorks(_ context.Context, sourceID string, relationshipType *string, _ int) ([]model.WorkRelationship, error) {
	byType := f.rels[sourceID]
	out := make([]model.WorkRelationship, 0)
	if relationshipType != nil && *relationshipType != "" {
		for _, rel := range byType[*relationshipType] {
			out = append(out, rel)
		}
		return out, nil
	}
	for _, byTarget := range byType {
		for _, rel := range byTarget {
			out = append(out, rel)
		}
	}
	return out, nil
}

func (f *fakeBuilderWorkRelStore) DeleteRelationshipsForWork(_ context.Context, sourceID string, relationshipType *string) error {
	if relationshipType == nil || *relationshipType == "" {
		delete(f.rels, sourceID)
		return nil
	}
	if _, ok := f.rels[sourceID]; ok {
		delete(f.rels[sourceID], *relationshipType)
	}
	return nil
}

func (f *fakeBuilderWorkRelStore) CountRelationshipsByType(_ context.Context) (map[string]int64, error) {
	counts := map[string]int64{}
	for _, byType := range f.rels {
		for relType, byTarget := range byType {
			counts[relType] += int64(len(byTarget))
		}
	}
	return counts, nil
}

func (f *fakeBuilderWorkRelStore) countForSourceAndType(sourceID, relationshipType string) int {
	return len(f.rels[sourceID][relationshipType])
}

func (f *fakeBuilderWorkRelStore) targetsForSourceAndType(sourceID, relationshipType string) []string {
	byTarget := f.rels[sourceID][relationshipType]
	targets := make([]string, 0, len(byTarget))
	for target := range byTarget {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}

func TestBuilder_UpdateGraphForWork_Idempotent(t *testing.T) {
	seriesName := "Saga"
	seriesIndex := 2.0
	workStore := &fakeBuilderWorkStore{work: model.Work{
		ID:          "w1",
		Title:       "Work One",
		SeriesName:  &seriesName,
		SeriesIndex: &seriesIndex,
		Subjects:    []string{"Fantasy"},
	}}
	seriesStore := newFakeBuilderSeriesStore()
	subjectStore := newFakeBuilderSubjectStore()
	relStore := newFakeBuilderWorkRelStore()

	builder := NewBuilder(nil, workStore, seriesStore, subjectStore, relStore)
	builder.fetchAuthorIDs = func(_ context.Context, _ string) ([]string, error) {
		return []string{"author-1"}, nil
	}
	builder.fetchWorksForAuthor = func(_ context.Context, _ string, _ string, _ int) ([]string, error) {
		return []string{"wa1", "wa2", "wa1"}, nil
	}
	builder.fetchSeriesIndex = func(_ context.Context, _ string, _ string) (float64, bool, error) {
		return 2.0, true, nil
	}
	builder.fetchSeriesNeighbor = func(_ context.Context, _ string, _ string, _ float64, previous bool) (string, error) {
		if previous {
			return "ws0", nil
		}
		return "ws2", nil
	}
	builder.fetchSeriesCandidates = func(_ context.Context, _ string, _ string, _ int) ([]string, error) {
		return nil, nil
	}

	summary1, err := builder.UpdateGraphForWorkWithSummary(context.Background(), "w1")
	if err != nil {
		t.Fatalf("first update failed: %v", err)
	}
	subjectIDs := subjectStore.workToSubjectIDs["w1"]
	if len(subjectIDs) != 1 {
		t.Fatalf("expected one subject for work, got %d", len(subjectIDs))
	}
	subjectStore.worksBySubjectID[subjectIDs[0]] = []model.Work{{ID: "w1"}, {ID: "su1"}, {ID: "su2"}, {ID: "su1"}}

	summary1, err = builder.UpdateGraphForWorkWithSummary(context.Background(), "w1")
	if err != nil {
		t.Fatalf("second update failed: %v", err)
	}

	summary2, err := builder.UpdateGraphForWorkWithSummary(context.Background(), "w1")
	if err != nil {
		t.Fatalf("third update failed: %v", err)
	}

	if summary1.AddedByType[relationshipTypeSameAuthor] != 2 || summary2.AddedByType[relationshipTypeSameAuthor] != 2 {
		t.Fatalf("expected same_author adds to remain stable at 2, got first=%d second=%d", summary1.AddedByType[relationshipTypeSameAuthor], summary2.AddedByType[relationshipTypeSameAuthor])
	}
	if summary1.AddedByType[relationshipTypeSameSeries] != 2 || summary2.AddedByType[relationshipTypeSameSeries] != 2 {
		t.Fatalf("expected same_series adds to remain stable at 2, got first=%d second=%d", summary1.AddedByType[relationshipTypeSameSeries], summary2.AddedByType[relationshipTypeSameSeries])
	}
	if summary1.AddedByType[relationshipTypeRelatedSubject] != 2 || summary2.AddedByType[relationshipTypeRelatedSubject] != 2 {
		t.Fatalf("expected related_subject adds to remain stable at 2, got first=%d second=%d", summary1.AddedByType[relationshipTypeRelatedSubject], summary2.AddedByType[relationshipTypeRelatedSubject])
	}

	if got := relStore.countForSourceAndType("w1", relationshipTypeSameAuthor); got != 2 {
		t.Fatalf("expected 2 same_author edges after repeated updates, got %d", got)
	}
	if got := relStore.countForSourceAndType("w1", relationshipTypeSameSeries); got != 2 {
		t.Fatalf("expected 2 same_series edges after repeated updates, got %d", got)
	}
	if got := relStore.countForSourceAndType("w1", relationshipTypeRelatedSubject); got != 2 {
		t.Fatalf("expected 2 related_subject edges after repeated updates, got %d", got)
	}
}

func TestBuilder_UpdateGraphForWork_RespectsCaps(t *testing.T) {
	seriesName := "Chronicles"
	workStore := &fakeBuilderWorkStore{work: model.Work{
		ID:         "w-cap",
		Title:      "Cap Work",
		SeriesName: &seriesName,
		Subjects:   []string{"History"},
	}}
	seriesStore := newFakeBuilderSeriesStore()
	subjectStore := newFakeBuilderSubjectStore()
	relStore := newFakeBuilderWorkRelStore()

	builder := NewBuilder(nil, workStore, seriesStore, subjectStore, relStore)
	builder.maxSameAuthor = 3
	builder.maxSameSeries = 4
	builder.maxRelatedSubject = 2

	builder.fetchAuthorIDs = func(_ context.Context, _ string) ([]string, error) {
		return []string{"a-cap"}, nil
	}
	builder.fetchWorksForAuthor = func(_ context.Context, _ string, _ string, _ int) ([]string, error) {
		return []string{"a1", "a2", "a3", "a4", "a5"}, nil
	}
	builder.fetchSeriesIndex = func(_ context.Context, _ string, _ string) (float64, bool, error) {
		return 0, false, nil
	}
	builder.fetchSeriesNeighbor = func(_ context.Context, _ string, _ string, _ float64, _ bool) (string, error) {
		return "", nil
	}
	builder.fetchSeriesCandidates = func(_ context.Context, _ string, _ string, limit int) ([]string, error) {
		candidates := []string{"s1", "s2", "s3", "s4", "s5", "s6"}
		if limit < len(candidates) {
			return candidates[:limit], nil
		}
		return candidates, nil
	}

	summary, err := builder.UpdateGraphForWorkWithSummary(context.Background(), "w-cap")
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	subjectIDs := subjectStore.workToSubjectIDs["w-cap"]
	if len(subjectIDs) != 1 {
		t.Fatalf("expected one subject id, got %d", len(subjectIDs))
	}
	subjectStore.worksBySubjectID[subjectIDs[0]] = []model.Work{{ID: "r1"}, {ID: "r2"}, {ID: "r3"}, {ID: "r4"}}

	summary, err = builder.UpdateGraphForWorkWithSummary(context.Background(), "w-cap")
	if err != nil {
		t.Fatalf("second update failed: %v", err)
	}

	if summary.AddedByType[relationshipTypeSameAuthor] != 3 {
		t.Fatalf("expected same_author cap=3, got %d", summary.AddedByType[relationshipTypeSameAuthor])
	}
	if summary.AddedByType[relationshipTypeSameSeries] != 4 {
		t.Fatalf("expected same_series cap=4, got %d", summary.AddedByType[relationshipTypeSameSeries])
	}
	if summary.AddedByType[relationshipTypeRelatedSubject] != 2 {
		t.Fatalf("expected related_subject cap=2, got %d", summary.AddedByType[relationshipTypeRelatedSubject])
	}

	if got := relStore.countForSourceAndType("w-cap", relationshipTypeSameAuthor); got != 3 {
		t.Fatalf("expected 3 same_author edges, got %d", got)
	}
	if got := relStore.countForSourceAndType("w-cap", relationshipTypeSameSeries); got != 4 {
		t.Fatalf("expected 4 same_series edges, got %d", got)
	}
	if got := relStore.countForSourceAndType("w-cap", relationshipTypeRelatedSubject); got != 2 {
		t.Fatalf("expected 2 related_subject edges, got %d", got)
	}
}
