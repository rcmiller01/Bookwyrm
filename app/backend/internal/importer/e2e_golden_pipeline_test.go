package importer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/integration/download"
	idxclient "app-backend/internal/integration/indexer"
)

type goldenGrab struct {
	CandidateID int64
	WorkID      string
	URI         string
	Protocol    string
}

type fixtureDownloadClient struct {
	mu         sync.Mutex
	nextID     int
	idToURI    map[string]string
	uriToPath  map[string]string
	clientName string
}

func newFixtureDownloadClient(uriToPath map[string]string) *fixtureDownloadClient {
	return &fixtureDownloadClient{
		nextID:     1,
		idToURI:    map[string]string{},
		uriToPath:  uriToPath,
		clientName: "nzbget",
	}
}

func (c *fixtureDownloadClient) Name() string { return c.clientName }

func (c *fixtureDownloadClient) AddDownload(_ context.Context, req download.AddRequest) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := "dl-" + strconv.Itoa(c.nextID)
	c.nextID++
	c.idToURI[id] = strings.TrimSpace(req.URI)
	return id, nil
}

func (c *fixtureDownloadClient) GetStatus(_ context.Context, downloadID string) (download.DownloadStatus, error) {
	c.mu.Lock()
	uri := c.idToURI[downloadID]
	c.mu.Unlock()
	outputPath, ok := c.uriToPath[uri]
	if !ok {
		return download.DownloadStatus{}, download.ErrDownloadNotFound
	}
	return download.DownloadStatus{
		ID:         downloadID,
		State:      "completed",
		Progress:   100,
		OutputPath: outputPath,
	}, nil
}

func (c *fixtureDownloadClient) Remove(_ context.Context, _ string, _ bool) error { return nil }

func newGoldenIndexerServer(t *testing.T, grabs map[int64]goldenGrab) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSpace(r.URL.Path)
		if strings.HasPrefix(path, "/v1/indexer/grabs/") {
			idRaw := strings.TrimPrefix(path, "/v1/indexer/grabs/")
			id, _ := strconv.ParseInt(strings.TrimSpace(idRaw), 10, 64)
			grab, ok := grabs[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "grab not found"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"grab": map[string]any{
				"id":           id,
				"candidate_id": grab.CandidateID,
				"entity_type":  "work",
				"entity_id":    grab.WorkID,
				"status":       "created",
			}})
			return
		}
		if strings.HasPrefix(path, "/v1/indexer/candidates/id/") {
			idRaw := strings.TrimPrefix(path, "/v1/indexer/candidates/id/")
			candidateID, _ := strconv.ParseInt(strings.TrimSpace(idRaw), 10, 64)
			for _, grab := range grabs {
				if grab.CandidateID != candidateID {
					continue
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"candidate": map[string]any{
					"id": candidateID,
					"candidate": map[string]any{
						"protocol": grab.Protocol,
						"grab_payload": map[string]any{
							"nzb_url": grab.URI,
						},
					},
				}})
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "candidate not found"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "not found"})
	}))
}

func waitImportJobForDownload(t *testing.T, store Store, downloadJobID int64, status JobStatus, timeout time.Duration) Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		jobs := store.ListJobs(JobFilter{Status: status, Limit: 100})
		for _, job := range jobs {
			if job.DownloadJobID == downloadJobID {
				return job
			}
		}
		time.Sleep(120 * time.Millisecond)
	}
	return Job{}
}

func TestGoldenE2E_EbookPipeline(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	sourceRoot := filepath.Join(t.TempDir(), "completed", "ebook")
	mustWriteFileEngine(t, filepath.Join(sourceRoot, "Dune.epub"), "ebook-content")

	grabID := int64(1001)
	uri := "fixture://ebook-golden"
	grabs := map[int64]goldenGrab{
		grabID: {CandidateID: 2001, WorkID: "work-ebook", URI: uri, Protocol: "usenet"},
	}

	indexerServer := newGoldenIndexerServer(t, grabs)
	defer indexerServer.Close()

	downloadStore := downloadqueue.NewStore()
	downloadSvc := download.NewService(newFixtureDownloadClient(map[string]string{uri: sourceRoot}))
	indexerClient := idxclient.NewClient(idxclient.Config{BaseURL: indexerServer.URL, Timeout: time.Second})
	manager := downloadqueue.NewManager(downloadStore, downloadSvc, indexerClient, "last_resort")

	importStore := NewMemoryStore()
	engine := NewEngine(Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
		KeepIncoming:            true,
	}, importStore, downloadStore, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.Start(ctx)
	engine.Start(ctx)

	dJob, err := manager.EnqueueFromGrab(context.Background(), grabID, "nzbget")
	if err != nil {
		t.Fatalf("enqueue from grab: %v", err)
	}

	imported := waitImportJobForDownload(t, importStore, dJob.ID, JobStatusImported, 12*time.Second)
	if imported.ID == 0 {
		t.Fatalf("expected imported job for ebook flow")
	}

	finalPath := filepath.Join(libraryRoot, "Unknown Author", "Dune", "Dune.epub")
	if _, err := os.Stat(finalPath); err != nil {
		t.Fatalf("expected final imported file at %s: %v", finalPath, err)
	}

	updatedDownload, err := downloadStore.GetJob(dJob.ID)
	if err != nil {
		t.Fatalf("get download job: %v", err)
	}
	if !updatedDownload.Imported {
		t.Fatalf("expected download job imported=true")
	}

	events := importStore.ListEvents(imported.ID)
	if len(events) == 0 {
		t.Fatalf("expected import events to be recorded")
	}
}

func TestGoldenE2E_AudiobookFolderPipeline(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	sourceRoot := filepath.Join(t.TempDir(), "completed", "audiobook")
	mustWriteFileEngine(t, filepath.Join(sourceRoot, "Track 01.mp3"), "track1")
	mustWriteFileEngine(t, filepath.Join(sourceRoot, "Track 02.mp3"), "track2")

	grabID := int64(1002)
	uri := "fixture://audiobook-golden"
	grabs := map[int64]goldenGrab{
		grabID: {CandidateID: 2002, WorkID: "work-audio", URI: uri, Protocol: "torrent"},
	}

	indexerServer := newGoldenIndexerServer(t, grabs)
	defer indexerServer.Close()

	downloadStore := downloadqueue.NewStore()
	downloadSvc := download.NewService(newFixtureDownloadClient(map[string]string{uri: sourceRoot}))
	indexerClient := idxclient.NewClient(idxclient.Config{BaseURL: indexerServer.URL, Timeout: time.Second})
	manager := downloadqueue.NewManager(downloadStore, downloadSvc, indexerClient, "last_resort")

	importStore := NewMemoryStore()
	engine := NewEngine(Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
		KeepIncoming:            true,
	}, importStore, downloadStore, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.Start(ctx)
	engine.Start(ctx)

	dJob, err := manager.EnqueueFromGrab(context.Background(), grabID, "nzbget")
	if err != nil {
		t.Fatalf("enqueue from grab: %v", err)
	}

	imported := waitImportJobForDownload(t, importStore, dJob.ID, JobStatusImported, 12*time.Second)
	if imported.ID == 0 {
		t.Fatalf("expected imported job for audiobook flow")
	}

	mp3Count := 0
	_ = filepath.WalkDir(libraryRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".mp3") {
			mp3Count++
		}
		return nil
	})
	if mp3Count < 2 {
		t.Fatalf("expected audiobook import to materialize at least 2 mp3 files, got %d", mp3Count)
	}
}

func TestGoldenE2E_CollisionUpgradeReplace(t *testing.T) {
	libraryRoot := filepath.Join(t.TempDir(), "library")
	oldSource := filepath.Join(t.TempDir(), "completed", "upgrade-old")
	newSource := filepath.Join(t.TempDir(), "completed", "upgrade-new")
	mustWriteFileEngine(t, filepath.Join(oldSource, "Dune.epub"), "old-content")
	mustWriteFileEngine(t, filepath.Join(newSource, "Dune.epub"), "new-content-upgraded")

	grabs := map[int64]goldenGrab{
		1003: {CandidateID: 2003, WorkID: "work-upgrade", URI: "fixture://upgrade-old", Protocol: "usenet"},
		1004: {CandidateID: 2004, WorkID: "work-upgrade", URI: "fixture://upgrade-new", Protocol: "usenet"},
	}

	indexerServer := newGoldenIndexerServer(t, grabs)
	defer indexerServer.Close()

	downloadStore := downloadqueue.NewStore()
	downloadSvc := download.NewService(newFixtureDownloadClient(map[string]string{
		"fixture://upgrade-old": oldSource,
		"fixture://upgrade-new": newSource,
	}))
	indexerClient := idxclient.NewClient(idxclient.Config{BaseURL: indexerServer.URL, Timeout: time.Second})
	manager := downloadqueue.NewManager(downloadStore, downloadSvc, indexerClient, "last_resort")

	importStore := NewMemoryStore()
	engine := NewEngine(Config{
		LibraryRoot:             libraryRoot,
		AllowCrossDeviceMove:    true,
		MaxScanFiles:            100,
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
		KeepIncoming:            true,
	}, importStore, downloadStore, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.Start(ctx)
	engine.Start(ctx)

	first, err := manager.EnqueueFromGrab(context.Background(), 1003, "nzbget")
	if err != nil {
		t.Fatalf("enqueue first upgrade grab: %v", err)
	}
	firstImported := waitImportJobForDownload(t, importStore, first.ID, JobStatusImported, 12*time.Second)
	if firstImported.ID == 0 {
		t.Fatalf("expected first import to complete")
	}

	second, err := manager.EnqueueFromGrab(context.Background(), 1004, "nzbget")
	if err != nil {
		t.Fatalf("enqueue second upgrade grab: %v", err)
	}
	reviewJob := waitImportJobForDownload(t, importStore, second.ID, JobStatusNeedsReview, 12*time.Second)
	if reviewJob.ID == 0 {
		t.Fatalf("expected second import to require review for collision")
	}

	if err := engine.Decide(reviewJob.ID, DecisionReplaceExisting); err != nil {
		t.Fatalf("apply replace_existing decision: %v", err)
	}

	postDecision, err := importStore.GetJob(reviewJob.ID)
	if err != nil {
		t.Fatalf("get post-decision job: %v", err)
	}
	if postDecision.Status != JobStatusImported {
		t.Fatalf("expected imported after decision, got %s", postDecision.Status)
	}

	finalPath := filepath.Join(libraryRoot, "Unknown Author", "Dune", "Dune.epub")
	raw, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	if string(raw) != "new-content-upgraded" {
		t.Fatalf("expected upgraded final content, got %q", string(raw))
	}

	trashEntries, err := os.ReadDir(filepath.Join(libraryRoot, "_trash"))
	if err != nil || len(trashEntries) == 0 {
		t.Fatalf("expected replaced content to be moved to trash")
	}
}
