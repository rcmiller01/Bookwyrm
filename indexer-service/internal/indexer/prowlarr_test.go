package indexer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProwlarrAdapterSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := []map[string]any{
			{
				"title":       "Dune Frank Herbert EPUB",
				"guid":        "guid-1",
				"downloadUrl": "https://example.invalid/download/1",
				"protocol":    "usenet",
				"indexer":     "IndexerA",
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	adapter := NewProwlarrAdapter("prowlarr-main", srv.URL, "key", 2*time.Second)
	res, err := adapter.Search(context.Background(), SearchRequest{
		Metadata: MetadataSnapshot{WorkID: "w1", Title: "Dune", Authors: []string{"Frank Herbert"}},
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(res.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(res.Candidates))
	}
	if res.Candidates[0].Provenance != "prowlarr:IndexerA" {
		t.Fatalf("unexpected provenance: %s", res.Candidates[0].Provenance)
	}
	if res.Candidates[0].GrabPayload["nzb_url"] == nil {
		t.Fatalf("expected nzb_url in grab payload")
	}
	if res.Candidates[0].GrabPayload["guid"] == nil {
		t.Fatalf("expected guid in grab payload")
	}
}

func TestProwlarrAdapterSearchTorrentGrabPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		payload := []map[string]any{
			{
				"title":      "Dune Frank Herbert Torrent",
				"guid":       "guid-2",
				"magnetUrl":  "magnet:?xt=urn:btih:abcdef",
				"torrentUrl": "https://example.invalid/torrents/2.torrent",
				"protocol":   "torrent",
				"indexer":    "IndexerB",
			},
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	adapter := NewProwlarrAdapter("prowlarr-main", srv.URL, "key", 2*time.Second)
	res, err := adapter.Search(context.Background(), SearchRequest{
		Metadata: MetadataSnapshot{WorkID: "w2", Title: "Dune", Authors: []string{"Frank Herbert"}},
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(res.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(res.Candidates))
	}
	if res.Candidates[0].Protocol != "torrent" {
		t.Fatalf("expected protocol torrent, got %s", res.Candidates[0].Protocol)
	}
	if res.Candidates[0].GrabPayload["magnet"] == nil {
		t.Fatalf("expected magnet in grab payload")
	}
	if res.Candidates[0].GrabPayload["torrent_url"] == nil {
		t.Fatalf("expected torrent_url in grab payload")
	}
}
