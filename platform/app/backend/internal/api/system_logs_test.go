package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"app-backend/internal/store"
)

func TestSystemLogsLocation(t *testing.T) {
	t.Setenv("BOOKWYRM_LOG_DIR", "C:\\ProgramData\\Bookwyrm\\logs")
	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/logs-location", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ProgramData") {
		t.Fatalf("expected logs path in response, got %s", rr.Body.String())
	}
}
