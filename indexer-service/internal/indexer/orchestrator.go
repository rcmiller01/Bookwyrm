package indexer

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"
)

type Orchestrator struct {
	store               Storage
	backends            map[string]SearchBackend
	quarantineMode      string
	candidateLimit      int
	maxEnqueuesPerTick  int
	once                sync.Once
}

func NewOrchestrator(store Storage, quarantineMode string) *Orchestrator {
	if strings.TrimSpace(quarantineMode) == "" {
		quarantineMode = "last_resort"
	}
	return &Orchestrator{
		store:              store,
		backends:           map[string]SearchBackend{},
		quarantineMode:     quarantineMode,
		candidateLimit:     50,
		maxEnqueuesPerTick: 5,
	}
}

func (o *Orchestrator) SetCandidateRetention(limit int) {
	if limit <= 0 {
		return
	}
	o.candidateLimit = limit
}

func (o *Orchestrator) SetMaxEnqueuesPerTick(n int) {
	if n <= 0 {
		return
	}
	o.maxEnqueuesPerTick = n
}

func (o *Orchestrator) RegisterBackend(backend SearchBackend, rec BackendRecord) {
	if backend == nil || strings.TrimSpace(rec.ID) == "" {
		return
	}
	o.backends[rec.ID] = backend
	o.store.UpsertBackend(rec)
}

func (o *Orchestrator) Start(ctx context.Context, workerCount int) {
	o.once.Do(func() {
		if workerCount <= 0 {
			workerCount = 2
		}
		for i := 0; i < workerCount; i++ {
			go o.worker(ctx)
		}
		go o.recoveryWorker(ctx)
		go o.wantedSchedulerWorker(ctx)
		go o.candidateCleanupWorker(ctx)
	})
}

func (o *Orchestrator) Enqueue(query QuerySpec) SearchRequestRecord {
	reqKey := buildRequestKey(query)
	rec := o.store.CreateOrGetSearchRequest(reqKey, query, 3)
	return rec
}

func (o *Orchestrator) ProcessRequest(ctx context.Context, requestID int64) error {
	rec, err := o.store.GetSearchRequest(requestID)
	if err != nil {
		return err
	}

	active := o.orderedBackends()
	merged := make([]Candidate, 0)
	for _, b := range active {
		timeout := time.Duration(rec.Query.Limits.TimeoutSec) * time.Second
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		start := time.Now()
		pCtx, cancel := context.WithTimeout(ctx, timeout)
		cands, searchErr := b.Search(pCtx, rec.Query)
		cancel()
		if searchErr != nil {
			_ = o.store.RecordBackendSearchResult(b.ID(), false, time.Since(start), false)
			continue
		}
		_ = o.store.RecordBackendSearchResult(b.ID(), true, time.Since(start), len(cands) > 0)
		for i := range cands {
			cands[i].SourceBackendID = b.ID()
			cands[i].SourcePipeline = b.Pipeline()
			if cands[i].NormalizedTitle == "" {
				cands[i].NormalizedTitle = normalizeText(cands[i].Title)
			}
		}
		merged = append(merged, cands...)
	}

	backendMap := map[string]BackendRecord{}
	for _, rec := range o.store.ListBackends() {
		backendMap[rec.ID] = rec
	}
	merged = DedupeCandidates(merged)
	merged = ApplyScoring(merged, backendMap, rec.Query)
	limit := rec.Query.Limits.MaxCandidates
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if len(merged) > limit {
		merged = merged[:limit]
	}
	if _, err := o.store.ReplaceCandidates(requestID, merged); err != nil {
		_ = o.store.RescheduleSearchRequest(requestID, err.Error(), time.Now().UTC().Add(backoffForAttempt(rec.AttemptCount)), rec.AttemptCount >= rec.MaxAttempts)
		return err
	}
	_ = o.store.MarkSearchRequestSucceeded(requestID)
	return nil
}

func (o *Orchestrator) worker(ctx context.Context) {
	workerID := "worker"
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rec, ok, err := o.store.TryLockNextSearchRequest(workerID, time.Now().UTC())
			if err != nil || !ok {
				continue
			}
			if err := o.ProcessRequest(ctx, rec.ID); err != nil {
				terminal := rec.AttemptCount >= rec.MaxAttempts
				_ = o.store.RescheduleSearchRequest(rec.ID, err.Error(), time.Now().UTC().Add(backoffForAttempt(rec.AttemptCount)), terminal)
			}
		}
	}
}

func (o *Orchestrator) recoveryWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = o.store.RecoverExpiredSearchRequests(time.Now().UTC(), 100)
		}
	}
}

func (o *Orchestrator) wantedSchedulerWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.enqueueDueWanted(time.Now().UTC())
		}
	}
}

func (o *Orchestrator) candidateCleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = o.store.PruneStaleCandidates(o.candidateLimit)
		}
	}
}

func (o *Orchestrator) enqueueDueWanted(now time.Time) {
	remaining := o.maxEnqueuesPerTick
	if remaining <= 0 {
		remaining = 5
	}
	for _, rec := range o.store.ListDueWantedWorks(now) {
		if remaining <= 0 {
			break
		}
		query := QuerySpec{
			EntityType: "work",
			EntityID:   rec.WorkID,
		}
		query.Preferences.Formats = append([]string(nil), rec.Formats...)
		query.Preferences.Languages = append([]string(nil), rec.Languages...)
		o.Enqueue(query)
		_ = o.store.MarkWantedWorkEnqueued(rec.WorkID, now)
		remaining--
	}
	for _, rec := range o.store.ListDueWantedAuthors(now) {
		if remaining <= 0 {
			break
		}
		query := QuerySpec{
			EntityType: "author",
			EntityID:   rec.AuthorID,
		}
		query.Preferences.Formats = append([]string(nil), rec.Formats...)
		query.Preferences.Languages = append([]string(nil), rec.Languages...)
		o.Enqueue(query)
		_ = o.store.MarkWantedAuthorEnqueued(rec.AuthorID, now)
		remaining--
	}
}

func (o *Orchestrator) orderedBackends() []SearchBackend {
	records := o.store.ListBackends()
	active := make([]BackendRecord, 0, len(records))
	for _, rec := range records {
		if !rec.Enabled {
			continue
		}
		if rec.Tier == TierQuarantine && o.quarantineMode == "disabled" {
			continue
		}
		if _, ok := o.backends[rec.ID]; !ok {
			continue
		}
		active = append(active, rec)
	}
	sort.SliceStable(active, func(i, j int) bool {
		pi := isBackendPreferred(active[i])
		pj := isBackendPreferred(active[j])
		if pi != pj {
			return pi
		}
		ti := tierRank(active[i].Tier)
		tj := tierRank(active[j].Tier)
		if ti != tj {
			return ti < tj
		}
		if active[i].ReliabilityScore != active[j].ReliabilityScore {
			return active[i].ReliabilityScore > active[j].ReliabilityScore
		}
		return active[i].Priority < active[j].Priority
	})
	out := make([]SearchBackend, 0, len(active))
	for _, rec := range active {
		out = append(out, o.backends[rec.ID])
	}
	return out
}

func isBackendPreferred(rec BackendRecord) bool {
	if rec.Config == nil {
		return false
	}
	v, ok := rec.Config["preferred"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func tierRank(t DispatchTier) int {
	switch t {
	case TierPrimary:
		return 0
	case TierSecondary:
		return 1
	case TierFallback:
		return 2
	case TierQuarantine:
		return 3
	default:
		return 4
	}
}

func buildRequestKey(q QuerySpec) string {
	base := strings.Join([]string{
		strings.TrimSpace(q.EntityType),
		strings.TrimSpace(q.EntityID),
		strings.TrimSpace(q.Title),
		strings.TrimSpace(q.Author),
		strings.TrimSpace(q.ISBN),
		strings.TrimSpace(q.DOI),
		strings.Join(q.Preferences.Formats, ","),
		strings.Join(q.Preferences.Languages, ","),
	}, "|")
	sum := sha1.Sum([]byte(base))
	return hex.EncodeToString(sum[:])
}

func normalizeText(v string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(v)), " "))
}

func backoffForAttempt(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	backoff := time.Duration(1<<minInt(attempt, 7)) * time.Second
	if backoff > 5*time.Minute {
		return 5 * time.Minute
	}
	return backoff
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
