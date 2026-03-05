package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddlewareRejectsMissingAPIKey(t *testing.T) {
	h := &Handlers{}
	router := NewRouter(h, RouterOptions{AuthEnabled: true, APIKeys: []string{"phase10-key"}})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/providers/policy", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAuthMiddlewareAcceptsAPIKeyHeader(t *testing.T) {
	h := &Handlers{}
	router := NewRouter(h, RouterOptions{AuthEnabled: true, APIKeys: []string{"phase10-key"}})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/providers/policy", nil)
	req.Header.Set("X-API-Key", "phase10-key")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestRateLimitMiddlewareRejectsExcessRequests(t *testing.T) {
	h := &Handlers{}
	router := NewRouter(h, RouterOptions{RateLimitEnabled: true, RateLimitPerMinute: 1, RateLimitBurst: 0})

	rr1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/v1/providers/policy", nil)
	req1.RemoteAddr = "10.0.0.1:1234"
	router.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected first request status %d, got %d", http.StatusOK, rr1.Code)
	}

	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/v1/providers/policy", nil)
	req2.RemoteAddr = "10.0.0.1:5678"
	router.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request status %d, got %d", http.StatusTooManyRequests, rr2.Code)
	}
}

func TestV1VersionHeaderIsSet(t *testing.T) {
	h := &Handlers{}
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/providers/policy", nil)
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Bookwyrm-API-Version"); got != "v1" {
		t.Fatalf("expected api version header v1, got %q", got)
	}
}
