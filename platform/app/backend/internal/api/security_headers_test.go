package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"app-backend/internal/store"
)

func TestSecurityHeadersOnSPAAndAssets(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "assets", "app.js"), []byte("//ok"), 0o644); err != nil {
		t.Fatalf("write app.js: %v", err)
	}

	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	router := NewRouterWithConfig(h, RouterConfig{UIAssetsDir: tmp})

	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
		"Permissions-Policy":    "camera=(), microphone=(), geolocation=()",
	}

	paths := []string{"/", "/assets/app.js", "/library/books"}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		for header, want := range expected {
			got := rec.Header().Get(header)
			if got != want {
				t.Errorf("%s: header %s = %q, want %q", path, header, got, want)
			}
		}

		csp := rec.Header().Get("Content-Security-Policy-Report-Only")
		if csp == "" {
			t.Errorf("%s: missing Content-Security-Policy-Report-Only header", path)
		}
	}
}

func TestSecurityHeadersOnAPIRoutes(t *testing.T) {
	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	router := NewRouterWithConfig(h, RouterConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options on API routes")
	}
}
