package queue

import platformqueue "bookwyrm/platform/queue"

func DefaultDedupeIndexName(queueTable string) string {
	return platformqueue.DefaultDedupeIndexName(queueTable)
}

func ActiveQueueStatuses() []string {
	return platformqueue.ActiveQueueStatuses()
}

func IsUniqueViolation(err error) bool {
	return platformqueue.IsUniqueViolation(err)
}
