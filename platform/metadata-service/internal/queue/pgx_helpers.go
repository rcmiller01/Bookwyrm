package queue

import (
	platformqueue "bookwyrm/platform/queue"
	"context"
	"time"
)

type Queryer = platformqueue.Queryer

type RowQueryer = platformqueue.RowQueryer

func LockNextQueuedQuery(table string) string {
	return platformqueue.LockNextQueuedQuery(table)
}

func CountByStatus(ctx context.Context, db Queryer, table string) (map[string]int64, error) {
	return platformqueue.CountByStatus(ctx, db, table)
}

func NextRunnableAt(ctx context.Context, db RowQueryer, table string) (*time.Time, error) {
	return platformqueue.NextRunnableAt(ctx, db, table)
}
