package queue

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
)

type Queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type RowQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var sqlIdentifierPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func LockNextQueuedQuery(table string) string {
	t := validatedIdentifier(table)
	return fmt.Sprintf(`
		WITH next_job AS (
			SELECT id
			FROM %s
			WHERE status = 'queued'
			  AND (not_before IS NULL OR not_before <= NOW())
			ORDER BY priority ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE %s AS j
		SET status = 'running',
		    locked_at = NOW(),
		    locked_by = $1,
		    updated_at = NOW()
		FROM next_job
		WHERE j.id = next_job.id
		RETURNING j.id, j.job_type, j.entity_type, j.entity_id, j.status, j.priority,
		          j.attempt_count, j.max_attempts, j.not_before, j.locked_at, j.locked_by,
		          j.last_error, j.created_at, j.updated_at`, t, t)
}

func CountByStatus(ctx context.Context, db Queryer, table string) (map[string]int64, error) {
	t := validatedIdentifier(table)
	rows, err := db.Query(ctx, fmt.Sprintf(`
		SELECT status, COUNT(*)
		FROM %s
		GROUP BY status`, t))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]int64{}
	for rows.Next() {
		var status string
		var count int64
		if scanErr := rows.Scan(&status, &count); scanErr != nil {
			return nil, scanErr
		}
		out[status] = count
	}
	return out, rows.Err()
}

func NextRunnableAt(ctx context.Context, db RowQueryer, table string) (*time.Time, error) {
	t := validatedIdentifier(table)
	var nextAt *time.Time
	err := db.QueryRow(ctx, fmt.Sprintf(`
		SELECT MIN(
			CASE
				WHEN not_before IS NULL OR not_before <= NOW() THEN NOW()
				ELSE not_before
			END
		)
		FROM %s
		WHERE status = 'queued'`, t)).Scan(&nextAt)
	if err != nil {
		return nil, err
	}
	return nextAt, nil
}

func validatedIdentifier(value string) string {
	if !sqlIdentifierPattern.MatchString(value) {
		panic("invalid SQL identifier")
	}
	return value
}
