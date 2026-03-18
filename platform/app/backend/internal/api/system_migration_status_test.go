package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app-backend/internal/store"
)

func TestSystemMigrationStatus_UnconfiguredDSN(t *testing.T) {
	t.Setenv("DATABASE_DSN", "")
	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/migration-status", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, `"status":"unconfigured"`) {
		t.Fatalf("expected unconfigured migration status, got %s", body)
	}
}

func TestSystemMigrationStatus_PendingMigrations(t *testing.T) {
	t.Setenv("DATABASE_DSN", "postgres://bookwyrm:test@localhost:5432/bookwyrm?sslmode=disable")
	prev := querySchemaMigrations
	querySchemaMigrations = func(_ context.Context, _ string) ([]migrationRecord, error) {
		return []migrationRecord{
			{Version: 1, Name: "000001_download_core.up.sql", AppliedAt: time.Now().UTC()},
			{Version: 2, Name: "000002_download_reliability_and_import_flag.up.sql", AppliedAt: time.Now().UTC()},
		}, nil
	}
	defer func() { querySchemaMigrations = prev }()

	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/migration-status", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); !strings.Contains(body, `"status":"pending"`) {
		t.Fatalf("expected pending migration status, got %s", body)
	}
}
