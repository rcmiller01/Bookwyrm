package download

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNZBGetClient_AddAndStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/file.nzb":
			w.Header().Set("Content-Type", "application/x-nzb")
			_, _ = w.Write([]byte("<nzb></nzb>"))
			return
		case "/jsonrpc":
			var req struct {
				Method string `json:"method"`
				Params []any  `json:"params"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			switch req.Method {
			case "append":
				if len(req.Params) != 10 {
					t.Fatalf("expected 10 params, got %d", len(req.Params))
				}
				if got := req.Params[0]; got != "file.nzb" {
					t.Fatalf("expected filename file.nzb, got %#v", got)
				}
				content, _ := req.Params[1].(string)
				decoded, err := base64.StdEncoding.DecodeString(content)
				if err != nil {
					t.Fatalf("expected base64 content: %v", err)
				}
				if string(decoded) != "<nzb></nzb>" {
					t.Fatalf("unexpected NZB content: %q", string(decoded))
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"result": 42, "error": nil})
			case "listgroups":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"result": []map[string]any{
						{
							"NZBID":            42,
							"Status":           "DOWNLOADING",
							"DownloadedSizeMB": 50.0,
							"RemainingSizeMB":  50.0,
							"DestDir":          "/downloads/completed/Dune",
						},
					},
					"error": nil,
				})
			case "history":
				_ = json.NewEncoder(w).Encode(map[string]any{"result": []map[string]any{}, "error": nil})
			default:
				_ = json.NewEncoder(w).Encode(map[string]any{"result": true, "error": nil})
			}
			return
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}))
	defer srv.Close()

	client := NewNZBGetClient(NZBGetConfig{BaseURL: srv.URL})
	id, err := client.AddDownload(context.Background(), AddRequest{URI: srv.URL + "/file.nzb"})
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
	if status.OutputPath != "/downloads/completed/Dune" {
		t.Fatalf("unexpected output path: %s", status.OutputPath)
	}
}

func TestNZBGetClient_GetStatusFallsBackToHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Method {
		case "listgroups":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": []map[string]any{}, "error": nil})
		case "history":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{{
					"NZBID":   907,
					"Status":  "DELETED/DUPE",
					"DestDir": "/downloads/history/ProjectHailMary",
				}},
				"error": nil,
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"result": true, "error": nil})
		}
	}))
	defer srv.Close()

	client := NewNZBGetClient(NZBGetConfig{BaseURL: srv.URL})
	status, err := client.GetStatus(context.Background(), "907")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if status.State != "canceled" {
		t.Fatalf("expected canceled state, got %s", status.State)
	}
	if status.OutputPath != "/downloads/history/ProjectHailMary" {
		t.Fatalf("unexpected output path: %s", status.OutputPath)
	}
}

func TestNZBGetClient_PostProcessRemainsActiveUntilSuccess(t *testing.T) {
	record := map[string]any{
		"NZBID":            914,
		"Status":           "PP_QUEUED",
		"DownloadedSizeMB": 447.0,
		"RemainingSizeMB":  0.0,
		"Directory":        "C:/ProgramData/NZBGet/intermediate/Project-Hail-Mary.#914",
		"DestDir":          "C:/ProgramData/NZBGet/complete/Project-Hail-Mary",
	}
	status := statusFromNZBGetRecord(record)
	if status.State == "completed" {
		t.Fatalf("post-processing record should not be completed: %+v", status)
	}
	if status.State != "unpacking" {
		t.Fatalf("expected unpacking during post-processing, got %s", status.State)
	}
	if status.OutputPath != "C:/ProgramData/NZBGet/complete/Project-Hail-Mary" {
		t.Fatalf("expected final destination to be preferred, got %s", status.OutputPath)
	}
}

func TestNZBFilenameFromURI_UsesFileQuery(t *testing.T) {
	got := nzbFilenameFromURI("https://example.invalid/download?file=Project%20Hail%20Mary.azw3")
	if got != "Project Hail Mary.azw3.nzb" {
		t.Fatalf("unexpected filename: %s", got)
	}
}
