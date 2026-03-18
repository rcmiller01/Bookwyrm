package download

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSABnzbdClientStatusFallsBackToHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("mode")
		switch mode {
		case "queue":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"queue": map[string]any{"slots": []any{}},
			})
		case "history":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"history": map[string]any{
					"slots": []map[string]any{
						{
							"nzo_id":  "SAB1",
							"status":  "Completed",
							"storage": "/downloads/completed/Dune",
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewSABnzbdClient(SABnzbdConfig{
		BaseURL: srv.URL,
		APIKey:  "k",
	})
	status, err := client.GetStatus(context.Background(), "SAB1")
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if status.State != "completed" {
		t.Fatalf("expected completed, got %s", status.State)
	}
	if status.OutputPath != "/downloads/completed/Dune" {
		t.Fatalf("unexpected output path: %s", status.OutputPath)
	}
}
