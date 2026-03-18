package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	GraphUpdatesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "graph_updates_total",
		Help: "Total number of successful graph update operations.",
	})

	GraphUpdateFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "graph_update_failures_total",
		Help: "Total number of failed graph update operations.",
	})

	GraphRelationshipsCreatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "graph_relationships_created_total",
		Help: "Total number of graph relationships created or refreshed by type.",
	}, []string{"relationship_type"})

	GraphSeriesTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "graph_series_total",
		Help: "Current total number of series nodes in metadata graph.",
	})

	GraphSubjectsTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "graph_subjects_total",
		Help: "Current total number of subject nodes in metadata graph.",
	})

	GraphRelationshipsTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "graph_relationships_total",
		Help: "Current total number of graph relationships by relationship_type.",
	}, []string{"relationship_type"})
)
