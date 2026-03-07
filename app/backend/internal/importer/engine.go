package importer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/integration/metadata"
)

type Engine struct {
	cfg           Config
	store         Store
	downloadStore downloadqueue.Storage
	metaClient    *metadata.Client
	renameFn      func(oldpath, newpath string) error
}

var ErrInvalidDecisionAction = errors.New("invalid decision action")

func NewEngine(cfg Config, store Store, downloadStore downloadqueue.Storage, metaClient *metadata.Client) *Engine {
	if strings.TrimSpace(cfg.LibraryRoot) == "" {
		cfg.LibraryRoot = filepath.Clean("." + string(os.PathSeparator) + "library")
	}
	if cfg.MaxScanFiles <= 0 {
		cfg.MaxScanFiles = 5000
	}
	if strings.TrimSpace(cfg.TemplateEbook) == "" {
		cfg.TemplateEbook = "{Author}/{Title} ({Year})/{Title} - {Author}.{Ext}"
	}
	if strings.TrimSpace(cfg.TemplateAudiobookSingle) == "" {
		cfg.TemplateAudiobookSingle = "{Author}/{Title} ({Year})/{Title} - {Author}.{Ext}"
	}
	if strings.TrimSpace(cfg.TemplateAudiobookFolder) == "" {
		cfg.TemplateAudiobookFolder = "{Author}/{Title} ({Year})"
	}
	if cfg.MaxPathLen <= 0 {
		cfg.MaxPathLen = 240
	}
	if cfg.KeepIncomingDays < 0 {
		cfg.KeepIncomingDays = 0
	}
	if cfg.KeepTrashDays < 0 {
		cfg.KeepTrashDays = 0
	}
	return &Engine{
		cfg:           cfg,
		store:         store,
		downloadStore: downloadStore,
		metaClient:    metaClient,
		renameFn:      os.Rename,
	}
}

func (e *Engine) Start(ctx context.Context) {
	e.startWorker(ctx, "creation-loop", e.creationLoop)
	e.startWorker(ctx, "worker-loop", e.workerLoop)
	e.startWorker(ctx, "recovery-loop", e.recoveryLoop)
	e.startWorker(ctx, "incoming-reconcile-loop", e.incomingReconcileLoop)
	e.startWorker(ctx, "cleanup-loop", e.cleanupLoop)
}

// RunMaintenance executes safe cleanup and reconciliation tasks on-demand.
func (e *Engine) RunMaintenance(now time.Time) (map[string]int, error) {
	incomingRemoved, incomingErr := e.cleanupIncoming(now)
	trashRemoved, trashErr := e.cleanupTrash(now)
	reconciled, reconcileErr := e.reconcileIncomingOrphans(now)
	result := map[string]int{
		"incoming_removed": incomingRemoved,
		"trash_removed":    trashRemoved,
		"reconciled":       reconciled,
	}
	if incomingErr != nil {
		return result, incomingErr
	}
	if trashErr != nil {
		return result, trashErr
	}
	if reconcileErr != nil {
		return result, reconcileErr
	}
	return result, nil
}

func (e *Engine) startWorker(ctx context.Context, name string, fn func(context.Context)) {
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			panicked := false
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						panicked = true
						log.Printf("importer %s panic: %v\n%s", name, rec, string(debug.Stack()))
					}
				}()
				fn(ctx)
			}()
			if !panicked {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}()
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
				e.addEvent(job, "detected", "import job detected from completed download", map[string]any{
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
				e.addEvent(job, "error", err.Error(), map[string]any{})
			}
		}
	}
}

func (e *Engine) recoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = e.store.RecoverExpiredLeases(time.Now().UTC(), 100)
		}
	}
}

func (e *Engine) incomingReconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(45 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = e.reconcileIncomingOrphans(time.Now().UTC())
		}
	}
}

func (e *Engine) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = e.cleanupIncoming(time.Now().UTC())
			_, _ = e.cleanupTrash(time.Now().UTC())
		}
	}
}

func (e *Engine) cleanupIncoming(now time.Time) (int, error) {
	if !e.cfg.KeepIncoming || e.cfg.KeepIncomingDays <= 0 {
		return 0, nil
	}
	root := filepath.Join(e.cfg.LibraryRoot, "_incoming")
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	cutoff := now.Add(-time.Duration(e.cfg.KeepIncomingDays) * 24 * time.Hour)
	removed := 0
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, statErr := os.Stat(path)
		if statErr != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if downloadID, parseErr := strconv.ParseInt(strings.TrimSpace(entry.Name()), 10, 64); parseErr == nil && downloadID > 0 {
			if e.store.ExistsDownloadJob(downloadID) {
				continue
			}
		}
		if removeErr := os.RemoveAll(path); removeErr == nil {
			removed++
		}
	}
	return removed, nil
}

func (e *Engine) cleanupTrash(now time.Time) (int, error) {
	if e.cfg.KeepTrashDays <= 0 {
		return 0, nil
	}
	trashDir := os.Getenv("LIBRARY_TRASH_DIR")
	if strings.TrimSpace(trashDir) == "" {
		trashDir = filepath.Join(e.cfg.LibraryRoot, "_trash")
	}
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	cutoff := now.Add(-time.Duration(e.cfg.KeepTrashDays) * 24 * time.Hour)
	removed := 0
	for _, entry := range entries {
		path := filepath.Join(trashDir, entry.Name())
		info, statErr := os.Stat(path)
		if statErr != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if removeErr := os.RemoveAll(path); removeErr == nil {
			removed++
		}
	}
	return removed, nil
}

func (e *Engine) reconcileIncomingOrphans(now time.Time) (int, error) {
	incomingRoot := filepath.Join(e.cfg.LibraryRoot, "_incoming")
	entries, err := os.ReadDir(incomingRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}

	reconciled := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		downloadID, parseErr := strconv.ParseInt(strings.TrimSpace(entry.Name()), 10, 64)
		if parseErr != nil || downloadID <= 0 {
			continue
		}
		if e.store.ExistsDownloadJob(downloadID) {
			continue
		}

		sourcePath := filepath.Join(incomingRoot, entry.Name())
		downloadJob, getErr := e.downloadStore.GetJob(downloadID)
		if getErr != nil {
			downloadJob = downloadqueue.Job{ID: downloadID, OutputPath: sourcePath, WorkID: ""}
		} else {
			downloadJob.OutputPath = sourcePath
		}

		job, createErr := e.store.CreateOrGetFromDownload(downloadJob, e.cfg.LibraryRoot)
		if createErr != nil {
			continue
		}
		_ = e.store.MarkNeedsReview(job.ID, "orphan incoming directory detected; review required", map[string]any{
			"mode":        "slice_a3_incoming_reconcile",
			"detected_at": now.UTC().Format(time.RFC3339),
		}, map[string]any{
			"reason":          "orphan_incoming_directory",
			"source_path":     sourcePath,
			"download_job_id": downloadID,
		})
		e.addEvent(job, "reconciled", "orphan incoming directory reconciled into needs_review", map[string]any{
			"source_path":     sourcePath,
			"download_job_id": downloadID,
		})
		reconciled++
	}

	return reconciled, nil
}

func (e *Engine) processJob(_ context.Context, job Job) error {
	files, err := ScanMediaFiles(job.SourcePath, e.cfg.MaxScanFiles)
	if err != nil {
		return fmt.Errorf("scan source path: %w", err)
	}
	if len(files) == 0 {
		return errors.New("no supported media files found in source path")
	}
	matchedJob, confidence, candidates := e.matchJob(job, files)
	if confidence < 0.85 {
		naming := map[string]any{
			"mode":       "slice_c_review_gate",
			"confidence": confidence,
		}
		decision := map[string]any{
			"reason":     "low_confidence_identity_match",
			"confidence": confidence,
			"candidates": candidates,
		}
		_ = e.store.MarkNeedsReview(job.ID, "needs review: ambiguous work/edition match", naming, decision)
		e.addEvent(job, "warning", "needs review due to low match confidence", decision)
		return nil
	}
	job = matchedJob
	incomingDir := filepath.Join(job.TargetRoot, "_incoming", fmt.Sprintf("%d", job.DownloadJobID))
	if err := os.MkdirAll(incomingDir, 0o755); err != nil {
		return fmt.Errorf("create incoming dir: %w", err)
	}

	movedCount := 0
	staged := make([]ScannedFile, 0, len(files))
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
			staged = append(staged, ScannedFile{Path: target, Ext: f.Ext, Size: info.Size()})
		}
		movedCount++
	}

	plans := make([]NamingPlan, 0, len(staged))
	audioCount := 0
	for _, f := range staged {
		if isAudiobookExt(strings.TrimPrefix(strings.ToLower(filepath.Ext(f.Path)), ".")) {
			audioCount++
		}
	}
	for _, f := range staged {
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(f.Path)), ".")
		trackMode := isAudiobookExt(ext) && audioCount > 1
		plans = append(plans, BuildNamingPlan(e.cfg, job, f.Path, trackMode))
	}
	if len(plans) == 0 {
		return errors.New("no staged files available for rename plan")
	}
	naming := map[string]any{
		"mode":            "slice_b_template_final",
		"incoming_target": incomingDir,
		"moved_count":     movedCount,
		"plans":           plansToMaps(plans),
	}
	decision := map[string]any{
		"strategy": "template_rename",
		"slice":    "B",
	}

	for i, p := range plans {
		srcInfo, srcErr := os.Stat(p.SourcePath)
		if srcErr != nil {
			return srcErr
		}
		if dstInfo, err := os.Stat(p.TargetPath); err == nil {
			if dstInfo.Size() == srcInfo.Size() {
				// Idempotent existing final file; remove staged duplicate.
				_ = os.Remove(p.SourcePath)
				continue
			}
			// Check download job's upgrade_action for auto-resolution.
			upgradeAction := "ask"
			if dJob, dErr := e.downloadStore.GetJob(job.DownloadJobID); dErr == nil && dJob.UpgradeAction != "" {
				upgradeAction = dJob.UpgradeAction
			}
			switch upgradeAction {
			case "replace":
				if err := e.moveExistingToTrash(p.TargetPath); err != nil {
					return err
				}
				e.addEvent(job, "auto_upgrade", "auto-replacing existing file per profile policy", map[string]any{
					"target_path":    p.TargetPath,
					"upgrade_action": "replace",
				})
				// Fall through to normal move/copy below.
			case "keep_both":
				newTarget := e.nextKeepBothPath(p.TargetPath)
				if err := os.MkdirAll(filepath.Dir(newTarget), 0o755); err != nil {
					return err
				}
				if err := e.moveOrCopy(p.SourcePath, newTarget); err != nil {
					return err
				}
				if info, statErr := os.Stat(newTarget); statErr == nil {
					_, _ = e.store.UpsertLibraryItem(LibraryItem{
						WorkID:    fallback(job.WorkID, "unknown-work"),
						EditionID: job.EditionID,
						Path:      newTarget,
						Format:    formatFromExt(filepath.Ext(newTarget)),
						SizeBytes: info.Size(),
					})
				}
				e.addEvent(job, "auto_upgrade", "auto-keeping both files per profile policy", map[string]any{
					"target_path":    p.TargetPath,
					"new_path":       newTarget,
					"upgrade_action": "keep_both",
				})
				plans[i].SourcePath = newTarget
				continue
			default: // "ask" or unrecognized — fall through to needs_review
				decision["collision"] = map[string]any{
					"target_path": p.TargetPath,
					"reason":      "existing file differs",
				}
				_ = e.store.MarkNeedsReview(job.ID, "target exists with different content; review required", naming, decision)
				e.addEvent(job, "warning", "collision requires review", map[string]any{
					"target_path": p.TargetPath,
					"source_path": p.SourcePath,
				})
				return nil
			}
		}
		if err := os.MkdirAll(filepath.Dir(p.TargetPath), 0o755); err != nil {
			return err
		}
		if err := e.moveOrCopy(p.SourcePath, p.TargetPath); err != nil {
			return err
		}
		if info, err := os.Stat(p.TargetPath); err == nil {
			_, _ = e.store.UpsertLibraryItem(LibraryItem{
				WorkID:    fallback(job.WorkID, "unknown-work"),
				EditionID: job.EditionID,
				Path:      p.TargetPath,
				Format:    formatFromExt(filepath.Ext(p.TargetPath)),
				SizeBytes: info.Size(),
			})
		}
		plans[i].SourcePath = p.TargetPath
	}

	finalTarget := filepath.Dir(plans[0].TargetPath)

	if !e.cfg.KeepIncoming {
		_ = os.RemoveAll(incomingDir)
	}

	if err := e.store.MarkImported(job.ID, finalTarget, naming, decision); err != nil {
		return err
	}
	if err := e.downloadStore.MarkImported(job.DownloadJobID, true); err != nil {
		return err
	}
	e.addEvent(job, "completed", "imported into final library layout", map[string]any{
		"target_path": finalTarget,
		"moved_count": movedCount,
	})
	return nil
}

func (e *Engine) Decide(jobID int64, action DecisionAction) error {
	if !IsValidDecisionAction(action) {
		return ErrInvalidDecisionAction
	}
	job, err := e.store.GetJob(jobID)
	if err != nil {
		return err
	}
	if job.Status != JobStatusNeedsReview {
		return fmt.Errorf("import job is not in needs_review state")
	}
	if action == DecisionSkip {
		if err := e.store.Skip(jobID, "operator decision: skip"); err != nil {
			return err
		}
		e.addEvent(job, "decision_applied", "decision action applied", map[string]any{"action": string(action)})
		return nil
	}

	incomingDir := filepath.Join(job.TargetRoot, "_incoming", fmt.Sprintf("%d", job.DownloadJobID))
	files, scanErr := ScanMediaFiles(incomingDir, e.cfg.MaxScanFiles)
	if scanErr != nil {
		return fmt.Errorf("scan incoming dir: %w", scanErr)
	}
	if len(files) == 0 {
		return fmt.Errorf("no staged files found for decision")
	}

	plans := make([]NamingPlan, 0, len(files))
	audioCount := 0
	for _, f := range files {
		if isAudiobookExt(strings.TrimPrefix(strings.ToLower(filepath.Ext(f.Path)), ".")) {
			audioCount++
		}
	}
	for _, f := range files {
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(f.Path)), ".")
		trackMode := isAudiobookExt(ext) && audioCount > 1
		plans = append(plans, BuildNamingPlan(e.cfg, job, f.Path, trackMode))
	}
	if len(plans) == 0 {
		return fmt.Errorf("no rename plans generated for decision")
	}

	applied := make([]map[string]any, 0, len(plans))
	for _, plan := range plans {
		target := plan.TargetPath
		switch action {
		case DecisionKeepBoth:
			target = e.nextKeepBothPath(plan.TargetPath)
		case DecisionReplaceExisting:
			if err := e.moveExistingToTrash(plan.TargetPath); err != nil {
				return err
			}
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := e.moveOrCopy(plan.SourcePath, target); err != nil {
			return err
		}
		if info, statErr := os.Stat(target); statErr == nil {
			_, _ = e.store.UpsertLibraryItem(LibraryItem{
				WorkID:    fallback(job.WorkID, "unknown-work"),
				EditionID: job.EditionID,
				Path:      target,
				Format:    formatFromExt(filepath.Ext(target)),
				SizeBytes: info.Size(),
			})
		}
		applied = append(applied, map[string]any{
			"source_path": plan.SourcePath,
			"target_path": target,
		})
	}

	if !e.cfg.KeepIncoming {
		_ = os.RemoveAll(incomingDir)
	}

	if len(applied) == 0 {
		return fmt.Errorf("no decision paths were applied")
	}
	finalTargetPath, ok := applied[0]["target_path"].(string)
	if !ok || strings.TrimSpace(finalTargetPath) == "" {
		return fmt.Errorf("invalid decision final target path")
	}
	finalTarget := filepath.Dir(finalTargetPath)
	decision := cloneDecision(job.Decision)
	decision["action"] = string(action)
	decision["applied_paths"] = applied
	naming := cloneDecision(job.NamingResult)
	naming["mode"] = "slice_b_decision_applied"

	if err := e.store.MarkImported(jobID, finalTarget, naming, decision); err != nil {
		return err
	}
	if err := e.downloadStore.MarkImported(job.DownloadJobID, true); err != nil {
		return err
	}
	e.addEvent(job, "decision_applied", "decision action applied", map[string]any{"action": string(action), "final_target": finalTarget})
	return nil
}

func (e *Engine) addEvent(job Job, eventType string, message string, payload map[string]any) {
	_ = e.store.AddEvent(job.ID, eventType, message, e.withCorrelation(job, payload))
}

func (e *Engine) withCorrelation(job Job, payload map[string]any) map[string]any {
	out := map[string]any{
		"import_job_id":   job.ID,
		"download_job_id": job.DownloadJobID,
		"work_id":         job.WorkID,
		"edition_id":      job.EditionID,
	}
	if payload != nil {
		for k, v := range payload {
			out[k] = v
		}
	}
	return out
}

func (e *Engine) nextKeepBothPath(original string) string {
	dir := filepath.Dir(original)
	ext := filepath.Ext(original)
	base := strings.TrimSuffix(filepath.Base(original), ext)
	candidate := filepath.Join(dir, base+" (copy)"+ext)
	if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
		return candidate
	}
	for i := 2; i <= 1000; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s (copy %d)%s", base, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return filepath.Join(dir, fmt.Sprintf("%s (copy-%d)%s", base, time.Now().UTC().Unix(), ext))
}

func (e *Engine) moveExistingToTrash(target string) error {
	if _, err := os.Stat(target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	trashDir := os.Getenv("LIBRARY_TRASH_DIR")
	if strings.TrimSpace(trashDir) == "" {
		trashDir = filepath.Join(e.cfg.LibraryRoot, "_trash")
	}
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return err
	}
	base := filepath.Base(target)
	trashPath := e.uniqueTrashPath(trashDir, base)
	if err := e.moveOrCopy(target, trashPath); err != nil {
		return err
	}
	return nil
}

func (e *Engine) uniqueTrashPath(trashDir string, base string) string {
	stamp := time.Now().UTC().Format("20060102T150405.000000000")
	candidate := filepath.Join(trashDir, fmt.Sprintf("%s.%s", base, stamp))
	if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
		return candidate
	}
	for i := 1; i <= 1000; i++ {
		candidate = filepath.Join(trashDir, fmt.Sprintf("%s.%s.%d", base, stamp, i))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return filepath.Join(trashDir, fmt.Sprintf("%s.%d", base, time.Now().UTC().UnixNano()))
}

func cloneDecision(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
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
	if err := e.renameFn(src, dst); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if !e.cfg.AllowCrossDeviceMove {
		return fmt.Errorf("cross-device move not allowed: %s -> %s", src, dst)
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	srcInfo, srcErr := os.Stat(src)
	dstInfo, dstErr := os.Stat(dst)
	if srcErr != nil || dstErr != nil || srcInfo.Size() != dstInfo.Size() {
		return fmt.Errorf("copy verify failed for %s -> %s", src, dst)
	}
	_ = os.Remove(src)
	return nil
}

func sameFile(a string, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func (e *Engine) matchJob(job Job, files []ScannedFile) (Job, float64, []map[string]any) {
	titleHint, authorHint, isbnHint := parseNameHints(files[0].Path)
	if strings.TrimSpace(job.WorkID) != "" {
		confidence := 1.0
		candidates := []map[string]any{{"work_id": job.WorkID, "reason": "download_job_work_id"}}
		if e.metaClient != nil && titleHint != "" {
			if workEnvelope, err := e.metaClient.GetWork(context.Background(), job.WorkID); err == nil {
				work, _ := workEnvelope["work"].(map[string]any)
				workTitle, _ := work["title"].(string)
				score := scoreTitleSimilarity(titleHint, workTitle)
				candidates[0]["title"] = workTitle
				candidates[0]["title_score"] = score
				if score < 0.45 {
					confidence = score
					candidates[0]["reason"] = "download_job_work_id_title_mismatch"
				}
			}
		}
		return job, confidence, candidates
	}
	if e.metaClient == nil || len(files) == 0 {
		return job, 0.2, nil
	}
	query := sanitizeQuery(titleHint)
	if query == "" {
		query = sanitizeQuery(strings.TrimSuffix(filepath.Base(files[0].Path), filepath.Ext(files[0].Path)))
	}
	if authorHint != "" {
		query = strings.TrimSpace(query + " " + authorHint)
	}
	if isbnHint != "" {
		query = isbnHint
	}
	resp, err := e.metaClient.Search(context.Background(), query)
	if err != nil {
		return job, 0.3, nil
	}
	rawWorks, _ := resp["works"].([]any)
	candidates := make([]map[string]any, 0, minInt(len(rawWorks), 5))
	bestScore := 0.0
	bestWorkID := ""
	for i, raw := range rawWorks {
		if i >= 5 {
			break
		}
		work, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		workID, _ := work["id"].(string)
		title, _ := work["title"].(string)
		titleScore := scoreTitleSimilarity(titleHint, title)
		authorScore := scoreAuthorSimilarity(authorHint, extractAuthors(work))
		score := titleScore
		if authorHint != "" {
			score = 0.85*titleScore + 0.15*authorScore
		}
		candidates = append(candidates, map[string]any{
			"work_id":      workID,
			"title":        title,
			"title_score":  titleScore,
			"author_score": authorScore,
			"score":        score,
		})
		if score > bestScore {
			bestScore = score
			bestWorkID = workID
		}
	}
	if bestWorkID != "" {
		job.WorkID = bestWorkID
	}
	return job, bestScore, candidates
}

func sanitizeQuery(v string) string {
	v = strings.ReplaceAll(v, ".", " ")
	v = strings.ReplaceAll(v, "_", " ")
	v = strings.Join(strings.Fields(v), " ")
	return strings.TrimSpace(v)
}

var isbnRegex = regexp.MustCompile(`\b(?:97[89])?\d{9}[\dXx]\b`)

func parseNameHints(path string) (title string, author string, isbn string) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	parts := strings.SplitN(base, " - ", 2)
	if len(parts) == 2 {
		title = sanitizeQuery(parts[0])
		author = sanitizeQuery(parts[1])
	} else {
		title = sanitizeQuery(base)
		author = ""
	}
	isbn = extractISBN(base)
	return title, author, isbn
}

func extractISBN(name string) string {
	match := isbnRegex.FindString(name)
	return strings.TrimSpace(match)
}

func scoreTitleSimilarity(a string, b string) float64 {
	a = strings.ToLower(sanitizeQuery(a))
	b = strings.ToLower(sanitizeQuery(b))
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return 0.9
	}
	// lightweight token overlap ratio
	aTokens := strings.Fields(a)
	bTokens := strings.Fields(b)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return 0
	}
	set := map[string]struct{}{}
	for _, t := range bTokens {
		set[t] = struct{}{}
	}
	match := 0.0
	for _, t := range aTokens {
		if _, ok := set[t]; ok {
			match++
		}
	}
	return math.Min(0.89, match/float64(maxInt(len(aTokens), len(bTokens))))
}

func scoreAuthorSimilarity(authorHint string, candidateAuthors []string) float64 {
	authorHint = strings.ToLower(sanitizeQuery(authorHint))
	if authorHint == "" || len(candidateAuthors) == 0 {
		return 0
	}
	best := 0.0
	for _, candidate := range candidateAuthors {
		score := scoreTitleSimilarity(authorHint, candidate)
		if score > best {
			best = score
		}
	}
	return best
}

func extractAuthors(work map[string]any) []string {
	out := []string{}
	rawAuthors, ok := work["authors"].([]any)
	if !ok {
		return out
	}
	for _, raw := range rawAuthors {
		if m, ok := raw.(map[string]any); ok {
			if name, ok := m["name"].(string); ok && strings.TrimSpace(name) != "" {
				out = append(out, name)
			}
			continue
		}
		if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func plansToMaps(plans []NamingPlan) []map[string]any {
	out := make([]map[string]any, 0, len(plans))
	for _, p := range plans {
		out = append(out, map[string]any{
			"source_path": p.SourcePath,
			"target_path": p.TargetPath,
			"format":      p.Format,
		})
	}
	return out
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
