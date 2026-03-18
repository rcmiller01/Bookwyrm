package graph

import (
	"context"
	"time"

	"metadata-service/internal/metrics"
	"metadata-service/internal/store"
)

func StartMetricsUpdater(
	ctx context.Context,
	seriesStore store.SeriesStore,
	subjectStore store.SubjectStore,
	relationshipStore store.WorkRelationshipStore,
	interval time.Duration,
) {
	if seriesStore == nil || subjectStore == nil || relationshipStore == nil {
		return
	}
	if interval <= 0 {
		interval = 45 * time.Second
	}

	update := func() {
		seriesCount, err := seriesStore.CountSeries(ctx)
		if err == nil {
			metrics.GraphSeriesTotal.Set(float64(seriesCount))
		}

		subjectCount, err := subjectStore.CountSubjects(ctx)
		if err == nil {
			metrics.GraphSubjectsTotal.Set(float64(subjectCount))
		}

		relByType, err := relationshipStore.CountRelationshipsByType(ctx)
		if err == nil {
			metrics.GraphRelationshipsTotal.Reset()
			for relType, count := range relByType {
				metrics.GraphRelationshipsTotal.WithLabelValues(relType).Set(float64(count))
			}
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	update()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			update()
		}
	}
}
