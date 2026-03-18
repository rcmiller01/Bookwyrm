package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"indexer-service/internal/indexer"
)

func TestClientSearchUsesToolContract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp/tool" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		if req["tool"] != "indexer.search" {
			t.Fatalf("expected tool indexer.search, got %v", req["tool"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": []map[string]any{
				{
					"title":        "Dune",
					"protocol":     "usenet",
					"grab_payload": map[string]any{"protocol": "usenet", "guid": "g1"},
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, time.Second)
	cands, err := client.Search(context.Background(), indexer.QuerySpec{EntityType: "work", EntityID: "w1", Title: "Dune"}, nil)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("expected one candidate, got %d", len(cands))
	}
}

func TestClientHealthUsesToolContract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp/tool" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"result": map[string]any{"ok": true},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, time.Second)
	if err := client.Health(context.Background(), nil); err != nil {
		t.Fatalf("health failed: %v", err)
	}
}
