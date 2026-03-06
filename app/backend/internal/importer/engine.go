package importer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
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
	return &Engine{
		cfg:           cfg,
		store:         store,
		downloadStore: downloadStore,
		metaClient:    metaClient,
		renameFn:      os.Rename,
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
		_ = e.store.AddEvent(job.ID, "warning", "needs review due to low match confidence", decision)
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
			decision["collision"] = map[string]any{
				"target_path": p.TargetPath,
				"reason":      "existing file differs",
			}
			_ = e.store.MarkNeedsReview(job.ID, "target exists with different content; review required", naming, decision)
			_ = e.store.AddEvent(job.ID, "warning", "collision requires review", map[string]any{
				"target_path": p.TargetPath,
				"source_path": p.SourcePath,
			})
			return nil
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
	_ = e.store.AddEvent(job.ID, "completed", "imported into final library layout", map[string]any{
		"target_path": finalTarget,
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
