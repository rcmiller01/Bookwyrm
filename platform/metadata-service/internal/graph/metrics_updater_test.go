package graph

import (
	"context"
	"testing"
	"time"

	"metadata-service/internal/metrics"
	"metadata-service/internal/model"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeMetricsSeriesStore struct {
	count int64
}

func (f *fakeMetricsSeriesStore) UpsertSeries(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}
func (f *fakeMetricsSeriesStore) UpsertSeriesEntry(_ context.Context, _ string, _ string, _ *float64) error {
	return nil
}
func (f *fakeMetricsSeriesStore) GetSeriesByID(_ context.Context, _ string) (*model.Series, error) {
	return nil, nil
}
func (f *fakeMetricsSeriesStore) GetSeriesEntries(_ context.Context, _ string) ([]model.SeriesEntry, error) {
	return nil, nil
}
func (f *fakeMetricsSeriesStore) GetSeriesForWork(_ context.Context, _ string) (*model.Series, error) {
	return nil, nil
}
func (f *fakeMetricsSeriesStore) CountSeries(_ context.Context) (int64, error) {
	return f.count, nil
}

type fakeMetricsSubjectStore struct {
	count int64
}

func (f *fakeMetricsSubjectStore) UpsertSubject(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}
func (f *fakeMetricsSubjectStore) SetWorkSubjects(_ context.Context, _ string, _ []string) error {
	return nil
}
func (f *fakeMetricsSubjectStore) GetSubjectByID(_ context.Context, _ string) (*model.Subject, error) {
	return nil, nil
}
func (f *fakeMetricsSubjectStore) GetSubjectsForWork(_ context.Context, _ string) ([]model.Subject, error) {
	return nil, nil
}
func (f *fakeMetricsSubjectStore) GetWorksForSubject(_ context.Context, _ string, _ int, _ int) ([]model.Work, error) {
	return nil, nil
}
func (f *fakeMetricsSubjectStore) CountSubjects(_ context.Context) (int64, error) {
	return f.count, nil
}

type fakeMetricsWorkRelStore struct {
	counts map[string]int64
}

func (f *fakeMetricsWorkRelStore) UpsertRelationship(_ context.Context, _ string, _ string, _ string, _ float64, _ *string) error {
	return nil
}
func (f *fakeMetricsWorkRelStore) GetRelatedWorks(_ context.Context, _ string, _ *string, _ int) ([]model.WorkRelationship, error) {
	return nil, nil
}
func (f *fakeMetricsWorkRelStore) DeleteRelationshipsForWork(_ context.Context, _ string, _ *string) error {
	return nil
}
func (f *fakeMetricsWorkRelStore) CountRelationshipsByType(_ context.Context) (map[string]int64, error) {
	return f.counts, nil
}

func TestStartMetricsUpdater_UpdatesGraphGauges(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	seriesStore := &fakeMetricsSeriesStore{count: 4}
	subjectStore := &fakeMetricsSubjectStore{count: 9}
	relStore := &fakeMetricsWorkRelStore{counts: map[string]int64{
		"same_author":     7,
		"same_series":     3,
		"related_subject": 5,
	}}

	go StartMetricsUpdater(ctx, seriesStore, subjectStore, relStore, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	if got := testutil.ToFloat64(metrics.GraphSeriesTotal); got != 4 {
		t.Fatalf("expected graph_series_total=4, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.GraphSubjectsTotal); got != 9 {
		t.Fatalf("expected graph_subjects_total=9, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.GraphRelationshipsTotal.WithLabelValues("same_author")); got != 7 {
		t.Fatalf("expected graph_relationships_total{same_author}=7, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.GraphRelationshipsTotal.WithLabelValues("same_series")); got != 3 {
		t.Fatalf("expected graph_relationships_total{same_series}=3, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.GraphRelationshipsTotal.WithLabelValues("related_subject")); got != 5 {
		t.Fatalf("expected graph_relationships_total{related_subject}=5, got %v", got)
	}
}
