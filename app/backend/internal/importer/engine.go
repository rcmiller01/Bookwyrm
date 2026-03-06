package importer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"app-backend/internal/downloadqueue"
)

type Engine struct {
	cfg           Config
	store         Store
	downloadStore downloadqueue.Storage
}

func NewEngine(cfg Config, store Store, downloadStore downloadqueue.Storage) *Engine {
	if strings.TrimSpace(cfg.LibraryRoot) == "" {
		cfg.LibraryRoot = filepath.Clean("." + string(os.PathSeparator) + "library")
	}
	if cfg.MaxScanFiles <= 0 {
		cfg.MaxScanFiles = 5000
	}
	return &Engine{
		cfg:           cfg,
		store:         store,
		downloadStore: downloadStore,
	}
}

func (e *Engine) Start(ctx context.Context) {
	go e.creationLoop(ctx)
	go e.workerLoop(ctx)
}

func (e *Engine) creationLoop(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, dj := range e.downloadStore.ListCompletedNotImported(100) {
				if strings.TrimSpace(dj.OutputPath) == "" {
					continue
				}
				job, err := e.store.CreateOrGetFromDownload(dj, e.cfg.LibraryRoot)
				if err != nil {
					continue
				}
				_ = e.store.AddEvent(job.ID, "detected", "import job detected from completed download", map[string]any{
					"download_job_id": dj.ID,
					"source_path":     dj.OutputPath,
				})
			}
		}
	}
}

func (e *Engine) workerLoop(ctx context.Context) {
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, ok, err := e.store.ClaimNextQueued("import-worker", time.Now().UTC())
			if err != nil || !ok {
				continue
			}
			if err := e.processJob(ctx, job); err != nil {
				terminal := job.AttemptCount >= job.MaxAttempts
				_ = e.store.MarkFailed(job.ID, err.Error(), terminal)
				_ = e.store.AddEvent(job.ID, "error", err.Error(), map[string]any{})
			}
		}
	}
}

func (e *Engine) processJob(_ context.Context, job Job) error {
	files, err := ScanMediaFiles(job.SourcePath, e.cfg.MaxScanFiles)
	if err != nil {
		return fmt.Errorf("scan source path: %w", err)
	}
	if len(files) == 0 {
		return errors.New("no supported media files found in source path")
	}
	incomingDir := filepath.Join(job.TargetRoot, "_incoming", fmt.Sprintf("%d", job.DownloadJobID))
	if err := os.MkdirAll(incomingDir, 0o755); err != nil {
		return fmt.Errorf("create incoming dir: %w", err)
	}

	movedCount := 0
	for _, f := range files {
		rel, err := filepath.Rel(job.SourcePath, f.Path)
		if err != nil {
			rel = filepath.Base(f.Path)
		}
		target := filepath.Clean(filepath.Join(incomingDir, rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create target dir: %w", err)
		}
		if err := e.moveOrCopy(f.Path, target); err != nil {
			return fmt.Errorf("move file %s: %w", f.Path, err)
		}
		if info, err := os.Stat(target); err == nil {
			_, _ = e.store.UpsertLibraryItem(LibraryItem{
				WorkID:    job.WorkID,
				EditionID: job.EditionID,
				Path:      target,
				Format:    formatFromExt(f.Ext),
				SizeBytes: info.Size(),
			})
		}
		movedCount++
	}

	naming := map[string]any{
		"mode":            "slice_a_move_only",
		"incoming_target": incomingDir,
		"moved_count":     movedCount,
	}
	decision := map[string]any{
		"strategy": "deterministic_incoming",
		"slice":    "A",
	}
	if err := e.store.MarkImported(job.ID, incomingDir, naming, decision); err != nil {
		return err
	}
	if err := e.downloadStore.MarkImported(job.DownloadJobID, true); err != nil {
		return err
	}
	_ = e.store.AddEvent(job.ID, "completed", "imported into incoming staging", map[string]any{
		"target_path": incomingDir,
		"moved_count": movedCount,
	})
	return nil
}

func (e *Engine) moveOrCopy(src string, dst string) error {
	if sameFile(src, dst) {
		return nil
	}
	if _, err := os.Stat(dst); err == nil {
		srcInfo, srcErr := os.Stat(src)
		dstInfo, dstErr := os.Stat(dst)
		if srcErr == nil && dstErr == nil && srcInfo.Size() == dstInfo.Size() {
			_ = os.Remove(src)
			return nil
		}
		return fmt.Errorf("target already exists: %s", dst)
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if !e.cfg.AllowCrossDeviceMove {
		return fmt.Errorf("cross-device move not allowed: %s -> %s", src, dst)
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	_ = os.Remove(src)
	return nil
}

func sameFile(a string, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
