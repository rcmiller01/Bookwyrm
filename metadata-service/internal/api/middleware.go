package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type RouterOptions struct {
	AuthEnabled        bool
	APIKeys            []string
	RateLimitEnabled   bool
	RateLimitPerMinute int
	RateLimitBurst     int
}

func apiVersionMiddleware(version string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Bookwyrm-API-Version", version)
			next.ServeHTTP(w, r)
		})
	}
}

func authMiddleware(enabled bool, keys []string) mux.MiddlewareFunc {
	if !enabled {
		return passthroughMiddleware
	}

	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			keySet[trimmed] = struct{}{}
		}
	}

	if len(keySet) == 0 {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeMiddlewareError(w, "api auth enabled but no api keys configured", http.StatusServiceUnavailable)
			})
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				writeMiddlewareError(w, "missing api key", http.StatusUnauthorized)
				return
			}
			if _, ok := keySet[token]; !ok {
				writeMiddlewareError(w, "invalid api key", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type rateWindowCounter struct {
	windowStart time.Time
	count       int
}

type apiRateLimiter struct {
	mu                sync.Mutex
	perMinute         int
	burst             int
	counters          map[string]*rateWindowCounter
	lastCleanupWindow time.Time
}

func newAPIRateLimiter(perMinute int, burst int) *apiRateLimiter {
	if perMinute <= 0 {
		perMinute = 120
	}
	if burst < 0 {
		burst = 0
	}
	return &apiRateLimiter{
		perMinute:         perMinute,
		burst:             burst,
		counters:          make(map[string]*rateWindowCounter),
		lastCleanupWindow: time.Now().UTC().Truncate(time.Minute),
	}
}

func (l *apiRateLimiter) allow(clientID string, now time.Time) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	window := now.UTC().Truncate(time.Minute)
	if window.After(l.lastCleanupWindow) {
		for id, c := range l.counters {
			if c.windowStart.Before(window) {
				delete(l.counters, id)
			}
		}
		l.lastCleanupWindow = window
	}

	allowed := l.perMinute + l.burst
	counter, ok := l.counters[clientID]
	if !ok || !counter.windowStart.Equal(window) {
		l.counters[clientID] = &rateWindowCounter{windowStart: window, count: 1}
		return true, maxInt(allowed-1, 0)
	}

	if counter.count >= allowed {
		return false, 0
	}
	counter.count++
	return true, maxInt(allowed-counter.count, 0)
}

func rateLimitMiddleware(enabled bool, limiter *apiRateLimiter) mux.MiddlewareFunc {
	if !enabled || limiter == nil {
		return passthroughMiddleware
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := extractClientID(r)
			allowed, remaining := limiter.allow(clientID, time.Now())
			w.Header().Set("X-RateLimit-Limit", intToString(limiter.perMinute+limiter.burst))
			w.Header().Set("X-RateLimit-Remaining", intToString(remaining))
			if !allowed {
				w.Header().Set("Retry-After", "60")
				writeMiddlewareError(w, "api rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type muxMiddleware = func(http.Handler) http.Handler
func passthroughMiddleware(next http.Handler) http.Handler { return next }

func extractToken(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get("X-API-Key")); token != "" {
		return token
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func extractClientID(r *http.Request) string {
	if token := extractToken(r); token != "" {
		return "key:" + token
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return "ip:" + strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return "ip:" + host
	}
	if strings.TrimSpace(r.RemoteAddr) != "" {
		return "ip:" + strings.TrimSpace(r.RemoteAddr)
	}
	return "ip:unknown"
}

func writeMiddlewareError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func intToString(v int) string {
	return strconv.Itoa(v)
}
