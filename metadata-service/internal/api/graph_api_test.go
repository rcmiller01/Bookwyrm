package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"metadata-service/internal/model"
)

type fakeGraphAPIWorkStore struct {
	works map[string]model.Work
}

func (f *fakeGraphAPIWorkStore) GetWorkByID(_ context.Context, id string) (*model.Work, error) {
	work := f.works[id]
	return &work, nil
}
func (f *fakeGraphAPIWorkStore) SearchWorks(_ context.Context, _ string) ([]model.Work, error) {
	return nil, nil
}
func (f *fakeGraphAPIWorkStore) InsertWork(_ context.Context, _ model.Work) error { return nil }
func (f *fakeGraphAPIWorkStore) UpdateWork(_ context.Context, _ model.Work) error { return nil }
func (f *fakeGraphAPIWorkStore) GetWorkByFingerprint(_ context.Context, _ string) (*model.Work, error) {
	return nil, nil
}

type fakeGraphAPISeriesStore struct {
	series      model.Series
	entries     []model.SeriesEntry
	seriesCount int64
}

func (f *fakeGraphAPISeriesStore) UpsertSeries(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}
func (f *fakeGraphAPISeriesStore) UpsertSeriesEntry(_ context.Context, _ string, _ string, _ *float64) error {
	return nil
}
func (f *fakeGraphAPISeriesStore) GetSeriesByID(_ context.Context, _ string) (*model.Series, error) {
	cp := f.series
	return &cp, nil
}
func (f *fakeGraphAPISeriesStore) GetSeriesEntries(_ context.Context, _ string) ([]model.SeriesEntry, error) {
	return f.entries, nil
}
func (f *fakeGraphAPISeriesStore) GetSeriesForWork(_ context.Context, _ string) (*model.Series, error) {
	cp := f.series
	return &cp, nil
}
func (f *fakeGraphAPISeriesStore) CountSeries(_ context.Context) (int64, error) {
	return f.seriesCount, nil
}

type fakeGraphAPISubjectStore struct {
	subject      model.Subject
	subjects     []model.Subject
	works        []model.Work
	subjectCount int64
}

func (f *fakeGraphAPISubjectStore) UpsertSubject(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}
func (f *fakeGraphAPISubjectStore) SetWorkSubjects(_ context.Context, _ string, _ []string) error {
	return nil
}
func (f *fakeGraphAPISubjectStore) GetSubjectByID(_ context.Context, _ string) (*model.Subject, error) {
	cp := f.subject
	return &cp, nil
}
func (f *fakeGraphAPISubjectStore) GetSubjectsForWork(_ context.Context, _ string) ([]model.Subject, error) {
	return f.subjects, nil
}
func (f *fakeGraphAPISubjectStore) GetWorksForSubject(_ context.Context, _ string, _ int, _ int) ([]model.Work, error) {
	return f.works, nil
}
func (f *fakeGraphAPISubjectStore) CountSubjects(_ context.Context) (int64, error) {
	return f.subjectCount, nil
}

type fakeGraphAPIRelStore struct {
	related []model.WorkRelationship
	counts  map[string]int64
}

func (f *fakeGraphAPIRelStore) UpsertRelationship(_ context.Context, _ string, _ string, _ string, _ float64, _ *string) error {
	return nil
}
func (f *fakeGraphAPIRelStore) GetRelatedWorks(_ context.Context, _ string, _ *string, _ int) ([]model.WorkRelationship, error) {
	return f.related, nil
}
func (f *fakeGraphAPIRelStore) DeleteRelationshipsForWork(_ context.Context, _ string, _ *string) error {
	return nil
}
func (f *fakeGraphAPIRelStore) CountRelationshipsByType(_ context.Context) (map[string]int64, error) {
	return f.counts, nil
}

func TestGraphAPI_GetWorkGraphContract(t *testing.T) {
	seriesStore := &fakeGraphAPISeriesStore{
		series:  model.Series{ID: "series:saga", Name: "Saga", NormalizedName: "saga", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		entries: []model.SeriesEntry{{SeriesID: "series:saga", WorkID: "w-1"}},
	}
	subjectStore := &fakeGraphAPISubjectStore{
		subjects: []model.Subject{{ID: "subject:fantasy", Name: "Fantasy", NormalizedName: "fantasy"}},
	}
	relStore := &fakeGraphAPIRelStore{related: []model.WorkRelationship{{
		SourceWorkID:     "w-1",
		TargetWorkID:     "w-2",
		RelationshipType: "same_author",
		Confidence:       0.7,
	}}}
	workStore := &fakeGraphAPIWorkStore{works: map[string]model.Work{"w-2": {ID: "w-2", Title: "Related"}}}

	h := &Handlers{
		workStore:    workStore,
		seriesStore:  seriesStore,
		subjectStore: subjectStore,
		workRelStore: relStore,
	}
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/work/w-1/graph", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var body WorkGraphResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if body.WorkID != "w-1" {
		t.Fatalf("expected work_id w-1, got %q", body.WorkID)
	}
	if body.Series == nil || body.Series.ID != "series:saga" {
		t.Fatalf("expected series payload, got %+v", body.Series)
	}
	if len(body.Subjects) != 1 || body.Subjects[0].ID != "subject:fantasy" {
		t.Fatalf("expected subject payload, got %+v", body.Subjects)
	}
	if len(body.Related) != 1 || body.Related[0].Work.ID != "w-2" {
		t.Fatalf("expected related work payload, got %+v", body.Related)
	}
}

func TestGraphAPI_GetGraphStatsContract(t *testing.T) {
	h := &Handlers{
		seriesStore:  &fakeGraphAPISeriesStore{seriesCount: 11},
		subjectStore: &fakeGraphAPISubjectStore{subjectCount: 22},
		workRelStore: &fakeGraphAPIRelStore{counts: map[string]int64{"same_author": 7, "same_series": 3}},
	}
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/graph/stats", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var body GraphStatsResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if body.SeriesCount != 11 || body.SubjectsCount != 22 {
		t.Fatalf("unexpected graph totals: %+v", body)
	}
	if body.RelationshipCountByType["same_author"] != 7 {
		t.Fatalf("expected same_author count=7, got %+v", body.RelationshipCountByType)
	}
}
