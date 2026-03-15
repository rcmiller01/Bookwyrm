package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/importer"
	"app-backend/internal/store"
)

func TestSupportBundle_GeneratesZipAndRedactsSecrets(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "bookwyrm.log")
	if err := os.WriteFile(logFile, []byte("DATABASE_DSN=postgres://user:secret@localhost/db\nok=line\n"), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}
	t.Setenv("BOOKWYRM_LOG_FILE", logFile)
	t.Setenv("DATABASE_DSN", "postgres://user:supersecret@localhost/db")

	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	h.SetStartupTime(time.Now().UTC())
	h.SetLibraryRoot("C:\\library")
	router := NewRouter(h)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/support-bundle", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	zr, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	entries := map[string]string{}
	for _, f := range zr.File {
		r, openErr := f.Open()
		if openErr != nil {
			t.Fatalf("open entry %s: %v", f.Name, openErr)
		}
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r)
		_ = r.Close()
		entries[f.Name] = buf.String()
	}
	if _, ok := entries["system/config-summary.json"]; !ok {
		t.Fatalf("missing config summary entry")
	}
	if _, ok := entries["system/readyz.json"]; !ok {
		t.Fatalf("missing readyz entry")
	}
	if _, ok := entries["system/dependencies.json"]; !ok {
		t.Fatalf("missing dependencies entry")
	}
	logBody, ok := entries["logs/bookwyrm.log"]
	if !ok {
		t.Fatalf("missing log entry")
	}
	if strings.Contains(strings.ToLower(logBody), "supersecret") || strings.Contains(strings.ToLower(logBody), "postgres://") {
		t.Fatalf("expected redacted log output, got %q", logBody)
	}
	if !strings.Contains(logBody, "<redacted>") {
		t.Fatalf("expected redaction marker in log output")
	}
}

func TestSystemActions_RetryFailedDownloadAndImport(t *testing.T) {
	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	dStore := downloadqueue.NewStore()
	dMgr := downloadqueue.NewManager(dStore, nil, nil, "last_resort")
	h.SetDownloadManager(dMgr)
	iStore := importer.NewMemoryStore()
	h.SetImportStore(iStore)
	router := NewRouter(h)

	dj, err := dStore.CreateJob(downloadqueue.Job{
		GrabID:      11,
		CandidateID: 22,
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create download job: %v", err)
	}
	if err := dStore.UpdateProgress(dj.ID, downloadqueue.JobStatusFailed, "", "boom"); err != nil {
		t.Fatalf("mark download failed: %v", err)
	}
	ij, err := iStore.CreateOrGetFromDownload(downloadqueue.Job{
		ID:         dj.ID,
		OutputPath: filepath.Join(t.TempDir(), "incoming"),
	}, filepath.Join(t.TempDir(), "library"))
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}
	if err := iStore.MarkFailed(ij.ID, "import failed", true); err != nil {
		t.Fatalf("mark import failed: %v", err)
	}

	downloadRetry := httptest.NewRecorder()
	downloadReq := httptest.NewRequest(http.MethodPost, "/api/v1/system/actions/retry-failed-downloads", nil)
	router.ServeHTTP(downloadRetry, downloadReq)
	if downloadRetry.Code != http.StatusOK {
		t.Fatalf("download retry status=%d", downloadRetry.Code)
	}
	var downloadBody map[string]any
	_ = json.NewDecoder(downloadRetry.Body).Decode(&downloadBody)
	if retried, _ := downloadBody["retried"].(float64); retried < 1 {
		t.Fatalf("expected retried download count >= 1, got %v", downloadBody["retried"])
	}

	importRetry := httptest.NewRecorder()
	importReq := httptest.NewRequest(http.MethodPost, "/api/v1/system/actions/retry-failed-imports", nil)
	router.ServeHTTP(importRetry, importReq)
	if importRetry.Code != http.StatusOK {
		t.Fatalf("import retry status=%d", importRetry.Code)
	}
	var importBody map[string]any
	_ = json.NewDecoder(importRetry.Body).Decode(&importBody)
	if retried, _ := importBody["retried"].(float64); retried < 1 {
		t.Fatalf("expected retried import count >= 1, got %v", importBody["retried"])
	}
}
