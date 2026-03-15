package autograb

import (
	"context"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode"

	"app-backend/internal/downloadqueue"
	"app-backend/internal/importer"
	"app-backend/internal/integration/indexer"
)

type downloadManager interface {
	EnqueueFromGrab(ctx context.Context, grabID int64, preferredClient string, upgradeAction string) (downloadqueue.Job, error)
	ListJobs(filter downloadqueue.JobFilter) []downloadqueue.Job
}

type Worker struct {
	indexerClient *indexer.Client
	downloadMgr   downloadManager
	importStore   importer.Store
	minScore      float64
	interval      time.Duration
	lastSeen      time.Time
	processed     map[int64]struct{}
}

const startupLookback = 15 * time.Minute

var episodePattern = regexp.MustCompile(`(?i)\bs\d{1,2}e\d{1,3}\b|\bepisode\s*\d+\b`)

func NewWorker(indexerClient *indexer.Client, downloadMgr downloadManager, importStore importer.Store, interval time.Duration, minScore float64) *Worker {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if minScore <= 0 {
		minScore = 0.70
	}
	return &Worker{
		indexerClient: indexerClient,
		downloadMgr:   downloadMgr,
		importStore:   importStore,
		minScore:      minScore,
		interval:      interval,
		lastSeen:      time.Now().UTC().Add(-startupLookback),
		processed:     map[int64]struct{}{},
	}
}

func (w *Worker) Start(ctx context.Context) {
	if w == nil || w.indexerClient == nil || w.downloadMgr == nil || w.importStore == nil {
		log.Printf("auto-grab: worker not started; missing dependency")
		return
	}
	log.Printf("auto-grab: worker started interval=%s min_score=%.2f initial_last_seen=%s", w.interval, w.minScore, w.lastSeen.Format(time.RFC3339))
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	updatedAfter := w.lastSeen.Add(-1 * time.Second)
	requests, err := w.indexerClient.ListSearchRequests(ctx, "succeeded", &updatedAfter, 50)
	if err != nil {
		log.Printf("auto-grab: list search requests failed: %v", err)
		return
	}
	log.Printf("auto-grab: fetched %d succeeded searches since %s", len(requests), updatedAfter.Format(time.RFC3339))
	maxSeen := w.lastSeen
	for _, req := range requests {
		if req.UpdatedAt.After(maxSeen) {
			maxSeen = req.UpdatedAt
		}
		if !req.Query.AutoGrab || req.EntityType != "work" || strings.TrimSpace(req.EntityID) == "" {
			log.Printf("auto-grab: skipping request=%d auto_grab=%t entity_type=%s entity_id=%s", req.ID, req.Query.AutoGrab, req.EntityType, req.EntityID)
			continue
		}
		if _, seen := w.processed[req.ID]; seen {
			log.Printf("auto-grab: request %d already processed", req.ID)
			continue
		}
		if len(w.importStore.ListLibraryItems(req.EntityID, 1)) > 0 {
			log.Printf("auto-grab: request %d skipped; library already has work %s", req.ID, req.EntityID)
			w.processed[req.ID] = struct{}{}
			continue
		}
		candidate, ok := w.pickCandidate(ctx, req)
		if !ok {
			log.Printf("auto-grab: request %d has no acceptable candidate", req.ID)
			w.processed[req.ID] = struct{}{}
			continue
		}
		if w.hasExistingJob(req.EntityID, candidate.ID) {
			log.Printf("auto-grab: request %d skipped; existing job for work=%s candidate=%d", req.ID, req.EntityID, candidate.ID)
			w.processed[req.ID] = struct{}{}
			continue
		}
		grab, err := w.indexerClient.GrabCandidate(ctx, candidate.ID)
		if err != nil {
			log.Printf("auto-grab: grab candidate %d for search %d failed: %v", candidate.ID, req.ID, err)
			continue
		}
		if _, err := w.downloadMgr.EnqueueFromGrab(ctx, grab.ID, "", "ask"); err != nil {
			log.Printf("auto-grab: enqueue download from grab %d failed: %v", grab.ID, err)
			continue
		}
		w.processed[req.ID] = struct{}{}
		log.Printf("auto-grab: search_request_id=%d work_id=%s candidate_id=%d grab_id=%d queued", req.ID, req.EntityID, candidate.ID, grab.ID)
	}
	w.lastSeen = maxSeen
}

func (w *Worker) pickCandidate(ctx context.Context, req indexer.SearchRequestRecord) (indexer.CandidateRecord, bool) {
	candidates, err := w.indexerClient.ListCandidates(ctx, req.ID, 10)
	if err != nil {
		log.Printf("auto-grab: list candidates for search %d failed: %v", req.ID, err)
		return indexer.CandidateRecord{}, false
	}
	log.Printf("auto-grab: search %d returned %d candidates", req.ID, len(candidates))
	for _, candidate := range candidates {
		log.Printf("auto-grab: candidate id=%d score=%.3f title=%q protocol=%s payload=%d", candidate.ID, candidate.Candidate.Score, candidate.Candidate.Title, candidate.Candidate.Protocol, len(candidate.Candidate.GrabPayload))
		if !candidateEligible(candidate, req) {
			continue
		}
		return candidate, true
	}
	return indexer.CandidateRecord{}, false
}

func candidateEligible(candidate indexer.CandidateRecord, req indexer.SearchRequestRecord) bool {
	if candidate.Candidate.Score < 0.70 {
		return false
	}
	if len(candidate.Candidate.GrabPayload) == 0 {
		return false
	}
	protocol := strings.ToLower(strings.TrimSpace(candidate.Candidate.Protocol))
	if protocol != "usenet" && protocol != "torrent" && protocol != "" {
		return false
	}
	title := strings.ToLower(strings.TrimSpace(candidate.Candidate.Title))
	if title == "" {
		return false
	}
	if looksLikeAuxiliaryPost(title) || looksLikeBlockedRelease(title) {
		return false
	}
	if !matchesRequestedTitle(title, req.Query.Title) {
		return false
	}
	if !looksLikeBookRelease(title, req.Query.Preferences.Formats) {
		return false
	}
	return true
}

func looksLikeAuxiliaryPost(title string) bool {
	for _, token := range []string{".par2", ".sfv", ".nfo", ".jpg", ".jpeg", ".png"} {
		if strings.Contains(title, token) {
			return true
		}
	}
	return false
}

func looksLikeBlockedRelease(title string) bool {
	if episodePattern.MatchString(title) {
		return true
	}
	for _, token := range []string{
		"1080p", "720p", "2160p", "bluray", "bdrip", "brrip", "webrip", "web-dl", "x264", "x265",
		"h264", "h265", "macos", "macosx", "windows", "linux", "android", "ios", ".exe", ".msi", ".dmg",
		"lynda.com", "udemy", "pluralsight", "course", "tutorial", "reaktor", "plugin", "single", "ep-web",
		"wav", "flac", "vinyl", "dj", "mix", "soundtrack",
	} {
		if strings.Contains(title, token) {
			return true
		}
	}
	return false
}

func looksLikeBookRelease(title string, preferredFormats []string) bool {
	if strings.Contains(title, "audiobook") || strings.Contains(title, "audio book") || strings.Contains(title, "ebook") {
		return true
	}
	formats := []string{"epub", "azw3", "mobi", "pdf", "m4b", "mp3"}
	formats = append(formats, preferredFormats...)
	tokens := tokenSet(title)
	for _, format := range formats {
		format = strings.ToLower(strings.TrimSpace(format))
		if format == "" {
			continue
		}
		if _, ok := tokens[format]; ok {
			return true
		}
		if strings.Contains(title, "."+format) {
			return true
		}
	}
	return false
}

func matchesRequestedTitle(candidateTitle string, requestedTitle string) bool {
	requestedTokens := significantTokens(requestedTitle)
	if len(requestedTokens) == 0 {
		return false
	}
	candidateTokens := tokenSet(candidateTitle)
	matched := 0
	for _, token := range requestedTokens {
		if _, ok := candidateTokens[token]; ok {
			matched++
		}
	}
	if len(requestedTokens) == 1 {
		return matched == 1
	}
	return matched == len(requestedTokens)
}

func significantTokens(value string) []string {
	stopwords := map[string]struct{}{
		"a": {}, "an": {}, "and": {}, "of": {}, "the": {}, "to": {}, "in": {}, "on": {},
	}
	parts := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) < 2 {
			continue
		}
		if _, blocked := stopwords[part]; blocked {
			continue
		}
		out = append(out, part)
	}
	return out
}

func tokenSet(value string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range significantTokens(value) {
		out[token] = struct{}{}
	}
	return out
}

func (w *Worker) hasExistingJob(workID string, candidateID int64) bool {
	jobs := w.downloadMgr.ListJobs(downloadqueue.JobFilter{Limit: 500})
	for _, job := range jobs {
		if job.CandidateID == candidateID {
			return true
		}
		if job.WorkID != workID {
			continue
		}
		if job.Status != downloadqueue.JobStatusFailed && job.Status != downloadqueue.JobStatusCanceled {
			return true
		}
	}
	return false
}
