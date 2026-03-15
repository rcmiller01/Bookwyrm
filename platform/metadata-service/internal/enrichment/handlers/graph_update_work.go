package handlers

import (
	"context"
	"fmt"
	"strings"

	"metadata-service/internal/graph"
	"metadata-service/internal/metrics"
	"metadata-service/internal/model"

	"github.com/rs/zerolog/log"
)

type graphWorkUpdater interface {
	UpdateGraphForWorkWithSummary(ctx context.Context, workID string) (graph.UpdateSummary, error)
}

// GraphUpdateWorkHandler rebuilds graph nodes/edges for a canonical work.
type GraphUpdateWorkHandler struct {
	builder graphWorkUpdater
}

func NewGraphUpdateWorkHandler(builder *graph.Builder) *GraphUpdateWorkHandler {
	return &GraphUpdateWorkHandler{builder: builder}
}

func NewGraphUpdateWorkHandlerWithUpdater(builder graphWorkUpdater) *GraphUpdateWorkHandler {
	return &GraphUpdateWorkHandler{builder: builder}
}

func (h *GraphUpdateWorkHandler) Type() string {
	return model.EnrichmentJobTypeGraphUpdate
}

func (h *GraphUpdateWorkHandler) Handle(ctx context.Context, job model.EnrichmentJob) error {
	fail := func(err error) error {
		metrics.GraphUpdateFailuresTotal.Inc()
		return err
	}

	if h.builder == nil {
		return fail(fmt.Errorf("graph builder not configured"))
	}
	if job.EntityType != "work" {
		return fail(fmt.Errorf("graph_update_work expects entity_type=work, got %q", job.EntityType))
	}
	if strings.TrimSpace(job.EntityID) == "" {
		return fail(fmt.Errorf("graph_update_work job missing entity_id"))
	}

	summary, err := h.builder.UpdateGraphForWorkWithSummary(ctx, job.EntityID)
	if err != nil {
		log.Error().Err(err).Str("work_id", job.EntityID).Msg("graph update failed")
		return fail(err)
	}

	metrics.GraphUpdatesTotal.Inc()
	for relationshipType, count := range summary.AddedByType {
		if count <= 0 {
			continue
		}
		metrics.GraphRelationshipsCreatedTotal.WithLabelValues(relationshipType).Add(float64(count))
	}

	log.Info().
		Str("work_id", job.EntityID).
		Int("subjects_updated", summary.SubjectsUpdated).
		Bool("series_updated", summary.SeriesUpdated).
		Int("same_author_added", summary.AddedByType["same_author"]).
		Int("same_series_added", summary.AddedByType["same_series"]).
		Int("related_subject_added", summary.AddedByType["related_subject"]).
		Msg("graph update completed")

	return nil
}
