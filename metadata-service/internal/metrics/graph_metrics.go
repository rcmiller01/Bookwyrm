package metrics

import platformmetrics "bookwyrm/platform/metrics"

var (
	GraphUpdatesTotal              = platformmetrics.GraphUpdatesTotal
	GraphUpdateFailuresTotal       = platformmetrics.GraphUpdateFailuresTotal
	GraphRelationshipsCreatedTotal = platformmetrics.GraphRelationshipsCreatedTotal
	GraphSeriesTotal               = platformmetrics.GraphSeriesTotal
	GraphSubjectsTotal             = platformmetrics.GraphSubjectsTotal
	GraphRelationshipsTotal        = platformmetrics.GraphRelationshipsTotal
)
