package queue

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func DefaultDedupeIndexName(queueTable string) string {
	return "uq_" + queueTable + "_dedupe"
}

func ActiveQueueStatuses() []string {
	return []string{"queued", "running"}
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
