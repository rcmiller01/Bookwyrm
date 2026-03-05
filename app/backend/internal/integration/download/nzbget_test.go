package download

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNZBGetClient_AddAndStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		method, _ := req["method"].(string)
		switch method {
		case "appendurl":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": 42, "error": nil})
		case "listgroups":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{
					{
						"NZBID":            42,
						"Status":           "DOWNLOADING",
						"DownloadedSizeMB": 50.0,
						"RemainingSizeMB":  50.0,
					},
				},
				"error": nil,
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"result": true, "error": nil})
		}
	}))
	defer srv.Close()

	client := NewNZBGetClient(NZBGetConfig{
		BaseURL: srv.URL,
	})

	id, err := client.AddDownload(context.Background(), AddRequest{URI: "https://example.invalid/file.nzb"})
	if err != nil {
		t.Fatalf("add download: %v", err)
	}
	if id != "42" {
		t.Fatalf("expected id 42, got %s", id)
	}

	status, err := client.GetStatus(context.Background(), id)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status.State != "downloading" {
		t.Fatalf("unexpected state: %s", status.State)
	}
	if status.Progress <= 0 {
		t.Fatalf("expected positive progress, got %f", status.Progress)
	}
}
