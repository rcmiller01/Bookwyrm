package queue

import (
	platformqueue "bookwyrm/platform/queue"
	"time"
)

type BackoffPolicy = platformqueue.BackoffPolicy

func NewExponentialBackoffPolicy(max time.Duration) BackoffPolicy {
	return platformqueue.NewExponentialBackoffPolicy(max)
}
