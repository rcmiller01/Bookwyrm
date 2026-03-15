package metrics

import platformmetrics "bookwyrm/platform/metrics"

var (
	EnrichmentJobsEnqueuedTotal    = platformmetrics.EnrichmentJobsEnqueuedTotal
	EnrichmentJobsStartedTotal     = platformmetrics.EnrichmentJobsStartedTotal
	EnrichmentJobsSucceededTotal   = platformmetrics.EnrichmentJobsSucceededTotal
	EnrichmentJobsFailedTotal      = platformmetrics.EnrichmentJobsFailedTotal
	EnrichmentJobsDeadTotal        = platformmetrics.EnrichmentJobsDeadTotal
	EnrichmentJobDurationSeconds   = platformmetrics.EnrichmentJobDurationSeconds
	EnrichmentJobBackoffSeconds    = platformmetrics.EnrichmentJobBackoffSeconds
	EnrichmentQueueDepth           = platformmetrics.EnrichmentQueueDepth
	EnrichmentWorkersActive        = platformmetrics.EnrichmentWorkersActive
	EnrichmentWorkersTotal         = platformmetrics.EnrichmentWorkersTotal
	EnrichmentWorkerIdleLoopsTotal = platformmetrics.EnrichmentWorkerIdleLoopsTotal
)
