package download

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQBitTorrentClientMagnetAndStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/torrents/add":
			w.WriteHeader(http.StatusOK)
		case "/api/v2/torrents/info":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"state":        "uploading",
					"progress":     1.0,
					"content_path": "/downloads/torrents/Dune",
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewQBitTorrentClient(QBitTorrentConfig{
		BaseURL:  srv.URL,
		Username: "user",
		Password: "pass",
	})
	magnet := "magnet:?xt=urn:btih:ABCDEF1234567890&dn=dune"
	id, err := client.AddDownload(context.Background(), AddRequest{URI: magnet, Tags: []string{"bookwyrm:grab:1"}})
	if err != nil {
		t.Fatalf("add download: %v", err)
	}
	if id == "" || !strings.Contains(strings.ToLower(id), "abcdef1234567890") {
		t.Fatalf("expected hash id from magnet, got %s", id)
	}

	status, err := client.GetStatus(context.Background(), id)
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if status.State != "completed" {
		t.Fatalf("expected completed status, got %s", status.State)
	}
	if status.OutputPath == "" {
		t.Fatalf("expected output path")
	}
}
