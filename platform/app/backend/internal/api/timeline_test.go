package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/importer"
	"app-backend/internal/integration/download"
	"app-backend/internal/store"
)

func TestWorkTimelineEndpointContract(t *testing.T) {
	h := NewHandlers(nil, nil, store.NewInMemoryWatchlistStore())
	importStore := importer.NewMemoryStore()
	h.SetImportStore(importStore)
	dStore := downloadqueue.NewStore()
	dMgr := downloadqueue.NewManager(dStore, download.NewService(), nil, "last_resort")
	h.SetDownloadManager(dMgr)
	router := NewRouter(h)

	dj, err := dStore.CreateJob(downloadqueue.Job{
		GrabID:      901,
		CandidateID: 902,
		WorkID:      "work-timeline-1",
		EditionID:   "ed-1",
		Protocol:    "usenet",
		ClientName:  "nzbget",
		MaxAttempts: 3,
		NotBefore:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create download job: %v", err)
	}
	if err := dStore.UpdateProgress(dj.ID, downloadqueue.JobStatusCompleted, "/downloads/work-timeline-1", ""); err != nil {
		t.Fatalf("mark download completed: %v", err)
	}
	if _, err := dStore.AddEvent(downloadqueue.Event{JobID: dj.ID, EventType: "submitted", Message: "download submitted", Data: map[string]any{"download_job_id": dj.ID, "grab_id": dj.GrabID, "candidate_id": dj.CandidateID, "work_id": dj.WorkID, "edition_id": dj.EditionID}}); err != nil {
		t.Fatalf("add download event: %v", err)
	}

	ij, err := importStore.CreateOrGetFromDownload(dj, "/library")
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}
	if err := importStore.MarkImported(ij.ID, "/library/Unknown Author/Title", map[string]any{"mode": "timeline-test"}, map[string]any{}); err != nil {
		t.Fatalf("mark import imported: %v", err)
	}
	if err := importStore.AddEvent(ij.ID, "completed", "import complete", map[string]any{"import_job_id": ij.ID, "download_job_id": dj.ID, "work_id": dj.WorkID, "edition_id": dj.EditionID}); err != nil {
		t.Fatalf("add import event: %v", err)
	}
	if _, err := importStore.UpsertLibraryItem(importer.LibraryItem{WorkID: dj.WorkID, EditionID: dj.EditionID, Path: "/library/Unknown Author/Title/Title.epub", Format: "epub", SizeBytes: 1234}); err != nil {
		t.Fatalf("upsert library item: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/work/work-timeline-1/timeline", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["work_id"] != "work-timeline-1" {
		t.Fatalf("expected work_id in response")
	}
	timeline, ok := payload["timeline"].(map[string]any)
	if !ok {
		t.Fatalf("expected timeline object")
	}

	for _, key := range []string{"searches", "grabs", "downloads", "imports", "library_items"} {
		if _, exists := timeline[key]; !exists {
			t.Fatalf("missing timeline key %s", key)
		}
	}

	downloads, _ := timeline["downloads"].([]any)
	if len(downloads) != 1 {
		t.Fatalf("expected one download timeline item, got %d", len(downloads))
	}
	downloadEnvelope, _ := downloads[0].(map[string]any)
	downloadEvents, _ := downloadEnvelope["events"].([]any)
	if len(downloadEvents) == 0 {
		t.Fatalf("expected download events in timeline")
	}
	firstDownloadEvent, _ := downloadEvents[0].(map[string]any)
	downloadData, _ := firstDownloadEvent["data_json"].(map[string]any)
	if downloadData["download_job_id"] != float64(dj.ID) {
		t.Fatalf("expected download event correlation download_job_id")
	}

	imports, _ := timeline["imports"].([]any)
	if len(imports) != 1 {
		t.Fatalf("expected one import timeline item, got %d", len(imports))
	}
	importEnvelope, _ := imports[0].(map[string]any)
	importEvents, _ := importEnvelope["events"].([]any)
	if len(importEvents) == 0 {
		t.Fatalf("expected import events in timeline")
	}
	firstImportEvent, _ := importEvents[0].(map[string]any)
	importPayload, _ := firstImportEvent["payload"].(map[string]any)
	if importPayload["import_job_id"] != float64(ij.ID) {
		t.Fatalf("expected import event correlation import_job_id")
	}
}
