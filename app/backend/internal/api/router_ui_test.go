package api

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"app-backend/internal/store"
)

func TestRouterServesSPAAssetsAndFallback(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "index.html"), []byte("<html><body>bookwyrm-ui</body></html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "assets", "app.js"), []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatalf("write assets/app.js: %v", err)
	}

	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	router := NewRouterWithConfig(h, RouterConfig{UIAssetsDir: tmp})

	assetReq := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	assetRes := httptest.NewRecorder()
	router.ServeHTTP(assetRes, assetReq)
	if assetRes.Code != http.StatusOK {
		t.Fatalf("expected 200 for asset, got %d", assetRes.Code)
	}

	spaReq := httptest.NewRequest(http.MethodGet, "/authors/123", nil)
	spaRes := httptest.NewRecorder()
	router.ServeHTTP(spaRes, spaReq)
	if spaRes.Code != http.StatusOK {
		t.Fatalf("expected 200 for spa fallback, got %d", spaRes.Code)
	}
	if !strings.Contains(spaRes.Body.String(), "bookwyrm-ui") {
		t.Fatalf("expected index.html body in fallback response")
	}

	apiReq := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	apiRes := httptest.NewRecorder()
	router.ServeHTTP(apiRes, apiReq)
	if apiRes.Code != http.StatusOK {
		t.Fatalf("expected 200 for existing api route, got %d", apiRes.Code)
	}
}

func TestRouterProxiesMetadataAndIndexerUIAPI(t *testing.T) {
	metadataServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/providers" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "metadata-ok")
	}))
	defer metadataServer.Close()

	indexerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/indexer/search/work/work-1" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, r.URL.Path)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "indexer-ok")
	}))
	defer indexerServer.Close()

	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	router := NewRouterWithConfig(h, RouterConfig{
		MetadataProxyBaseURL: metadataServer.URL,
		IndexerProxyBaseURL:  indexerServer.URL,
	})

	metadataReq := httptest.NewRequest(http.MethodGet, "/ui-api/metadata/providers", nil)
	metadataRes := httptest.NewRecorder()
	router.ServeHTTP(metadataRes, metadataReq)
	if metadataRes.Code != http.StatusOK {
		t.Fatalf("expected 200 for metadata proxy, got %d body=%q", metadataRes.Code, metadataRes.Body.String())
	}
	if strings.TrimSpace(metadataRes.Body.String()) != "metadata-ok" {
		t.Fatalf("unexpected metadata proxy body: %q", metadataRes.Body.String())
	}

	indexerReq := httptest.NewRequest(http.MethodPost, "/ui-api/indexer/search/work/work-1", nil)
	indexerRes := httptest.NewRecorder()
	router.ServeHTTP(indexerRes, indexerReq)
	if indexerRes.Code != http.StatusOK {
		t.Fatalf("expected 200 for indexer proxy, got %d body=%q", indexerRes.Code, indexerRes.Body.String())
	}
	if strings.TrimSpace(indexerRes.Body.String()) != "indexer-ok" {
		t.Fatalf("unexpected indexer proxy body: %q", indexerRes.Body.String())
	}
}

func TestProxyReturns502WhenUpstreamDown(t *testing.T) {
	// Start a listener, get its address, then close it immediately so nothing is listening.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	deadAddr := "http://" + ln.Addr().String()
	ln.Close()

	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	router := NewRouterWithConfig(h, RouterConfig{
		MetadataProxyBaseURL: deadAddr,
	})

	req := httptest.NewRequest(http.MethodGet, "/ui-api/metadata/providers", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when upstream is down, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !strings.Contains(body["error"], "unavailable") {
		t.Fatalf("expected 'unavailable' in error body, got %q", body["error"])
	}
}
