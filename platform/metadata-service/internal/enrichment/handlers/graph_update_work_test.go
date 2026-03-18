package handlers

import (
	"context"
	"errors"
	"testing"

	"metadata-service/internal/graph"
	"metadata-service/internal/metrics"
	"metadata-service/internal/model"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeGraphUpdater struct {
	summary graph.UpdateSummary
	err     error
	workID  string
}

func (f *fakeGraphUpdater) UpdateGraphForWorkWithSummary(_ context.Context, workID string) (graph.UpdateSummary, error) {
	f.workID = workID
	if f.err != nil {
		return graph.UpdateSummary{}, f.err
	}
	return f.summary, nil
}

func TestGraphUpdateWorkHandler_MetricsSuccess(t *testing.T) {
	updater := &fakeGraphUpdater{
		summary: graph.UpdateSummary{AddedByType: map[string]int{
			"same_author":     2,
			"same_series":     1,
			"related_subject": 3,
		}},
	}
	h := NewGraphUpdateWorkHandlerWithUpdater(updater)

	beforeUpdates := testutil.ToFloat64(metrics.GraphUpdatesTotal)
	beforeFailures := testutil.ToFloat64(metrics.GraphUpdateFailuresTotal)
	beforeAuthor := testutil.ToFloat64(metrics.GraphRelationshipsCreatedTotal.WithLabelValues("same_author"))
	beforeSeries := testutil.ToFloat64(metrics.GraphRelationshipsCreatedTotal.WithLabelValues("same_series"))
	beforeSubject := testutil.ToFloat64(metrics.GraphRelationshipsCreatedTotal.WithLabelValues("related_subject"))

	err := h.Handle(context.Background(), model.EnrichmentJob{
		JobType:    model.EnrichmentJobTypeGraphUpdate,
		EntityType: "work",
		EntityID:   "w-123",
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if updater.workID != "w-123" {
		t.Fatalf("expected updater called with work id w-123, got %q", updater.workID)
	}

	afterUpdates := testutil.ToFloat64(metrics.GraphUpdatesTotal)
	afterFailures := testutil.ToFloat64(metrics.GraphUpdateFailuresTotal)
	afterAuthor := testutil.ToFloat64(metrics.GraphRelationshipsCreatedTotal.WithLabelValues("same_author"))
	afterSeries := testutil.ToFloat64(metrics.GraphRelationshipsCreatedTotal.WithLabelValues("same_series"))
	afterSubject := testutil.ToFloat64(metrics.GraphRelationshipsCreatedTotal.WithLabelValues("related_subject"))

	if afterUpdates-beforeUpdates != 1 {
		t.Fatalf("expected graph updates counter to increase by 1, got %v", afterUpdates-beforeUpdates)
	}
	if afterFailures-beforeFailures != 0 {
		t.Fatalf("expected graph failures counter unchanged, got delta %v", afterFailures-beforeFailures)
	}
	if afterAuthor-beforeAuthor != 2 {
		t.Fatalf("expected same_author created delta 2, got %v", afterAuthor-beforeAuthor)
	}
	if afterSeries-beforeSeries != 1 {
		t.Fatalf("expected same_series created delta 1, got %v", afterSeries-beforeSeries)
	}
	if afterSubject-beforeSubject != 3 {
		t.Fatalf("expected related_subject created delta 3, got %v", afterSubject-beforeSubject)
	}
}

func TestGraphUpdateWorkHandler_MetricsFailure(t *testing.T) {
	h := NewGraphUpdateWorkHandlerWithUpdater(&fakeGraphUpdater{err: errors.New("boom")})

	beforeUpdates := testutil.ToFloat64(metrics.GraphUpdatesTotal)
	beforeFailures := testutil.ToFloat64(metrics.GraphUpdateFailuresTotal)

	err := h.Handle(context.Background(), model.EnrichmentJob{
		JobType:    model.EnrichmentJobTypeGraphUpdate,
		EntityType: "work",
		EntityID:   "w-500",
	})
	if err == nil {
		t.Fatalf("expected error from updater")
	}

	afterUpdates := testutil.ToFloat64(metrics.GraphUpdatesTotal)
	afterFailures := testutil.ToFloat64(metrics.GraphUpdateFailuresTotal)

	if afterUpdates-beforeUpdates != 0 {
		t.Fatalf("expected graph updates counter unchanged, got delta %v", afterUpdates-beforeUpdates)
	}
	if afterFailures-beforeFailures != 1 {
		t.Fatalf("expected graph failures counter to increase by 1, got %v", afterFailures-beforeFailures)
	}
}
