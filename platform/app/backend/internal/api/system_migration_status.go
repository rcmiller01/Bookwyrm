package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"app-backend/internal/downloadqueue"

	_ "github.com/lib/pq"
)

type migrationRecord struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	AppliedAt time.Time `json:"applied_at"`
}

var querySchemaMigrations = func(ctx context.Context, dsn string) ([]migrationRecord, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	rows, err := db.QueryContext(queryCtx, `SELECT version, name, applied_at FROM backend_schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]migrationRecord, 0)
	for rows.Next() {
		var rec migrationRecord
		if err := rows.Scan(&rec.Version, &rec.Name, &rec.AppliedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (h *Handlers) SystemMigrationStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	writeJSON(w, h.computeMigrationStatus(ctx))
}

func (h *Handlers) computeMigrationStatus(ctx context.Context) map[string]any {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_DSN"))
	expectedVersions := downloadqueue.EmbeddedMigrationVersions()
	expectedLatest := 0
	if len(expectedVersions) > 0 {
		expectedLatest = expectedVersions[len(expectedVersions)-1]
	}

	base := map[string]any{
		"status":            "unconfigured",
		"ready":             false,
		"database_dsn":      dsn != "",
		"expected_versions": expectedVersions,
		"expected_latest":   expectedLatest,
		"applied_versions":  []int{},
		"applied_latest":    0,
		"pending_versions":  expectedVersions,
		"pending_count":     len(expectedVersions),
	}
	if dsn == "" {
		base["detail"] = "DATABASE_DSN is not configured"
		return base
	}

	records, err := querySchemaMigrations(ctx, dsn)
	if err != nil {
		base["status"] = "failed"
		base["detail"] = err.Error()
		return base
	}

	applied := make([]int, 0, len(records))
	appliedSet := make(map[int]struct{}, len(records))
	for _, rec := range records {
		applied = append(applied, rec.Version)
		appliedSet[rec.Version] = struct{}{}
	}
	sort.Ints(applied)
	appliedLatest := 0
	if len(applied) > 0 {
		appliedLatest = applied[len(applied)-1]
	}

	pending := make([]int, 0)
	for _, v := range expectedVersions {
		if _, ok := appliedSet[v]; !ok {
			pending = append(pending, v)
		}
	}

	status := "ok"
	if len(pending) > 0 {
		status = "pending"
	}

	return map[string]any{
		"status":            status,
		"ready":             len(pending) == 0,
		"database_dsn":      true,
		"expected_versions": expectedVersions,
		"expected_latest":   expectedLatest,
		"applied_versions":  applied,
		"applied_latest":    appliedLatest,
		"pending_versions":  pending,
		"pending_count":     len(pending),
		"record_count":      len(records),
		"checked_at":        time.Now().UTC().Format(time.RFC3339),
		"detail":            fmt.Sprintf("applied %d/%d migration(s)", len(applied), len(expectedVersions)),
	}
}
