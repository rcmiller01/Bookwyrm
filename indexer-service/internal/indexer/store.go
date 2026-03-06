package indexer

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("not found")
)

type Store struct {
	mu sync.RWMutex

	nextRequestID   int64
	nextCandidateID int64
	nextGrabID      int64

	backends        map[string]BackendRecord
	mcpServers      map[string]MCPServerRecord
	searchRequests  map[int64]SearchRequestRecord
	requestKeyToID  map[string]int64
	candidatesByReq map[int64][]CandidateRecord
	candidateByID   map[int64]CandidateRecord
	grabsByID       map[int64]GrabRecord
	wantedWorks     map[string]WantedWorkRecord
	wantedAuthors   map[string]WantedAuthorRecord
	backendMetrics  map[string]indexerMetrics
}

type indexerMetrics struct {
	SuccessCount        int64
	FailureCount        int64
	TotalLatencyMS      int64
	SearchCount         int64
	CandidateYieldCount int64
}

func NewStore() *Store {
	return &Store{
		nextRequestID:   1,
		nextCandidateID: 1,
		nextGrabID:      1,
		backends:        map[string]BackendRecord{},
		mcpServers:      map[string]MCPServerRecord{},
		searchRequests:  map[int64]SearchRequestRecord{},
		requestKeyToID:  map[string]int64{},
		candidatesByReq: map[int64][]CandidateRecord{},
		candidateByID:   map[int64]CandidateRecord{},
		grabsByID:       map[int64]GrabRecord{},
		wantedWorks:     map[string]WantedWorkRecord{},
		wantedAuthors:   map[string]WantedAuthorRecord{},
		backendMetrics:  map[string]indexerMetrics{},
	}
}

func (s *Store) UpsertBackend(rec BackendRecord) BackendRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	existing, ok := s.backends[rec.ID]
	if !ok {
		rec.CreatedAt = now
	} else {
		rec.CreatedAt = existing.CreatedAt
	}
	rec.UpdatedAt = now
	if rec.ReliabilityScore == 0 {
		rec.ReliabilityScore = 0.70
	}
	if rec.Tier == "" {
		rec.Tier = TierUnclassified
	}
	s.backends[rec.ID] = rec
	return rec
}

func (s *Store) ListBackends() []BackendRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]BackendRecord, 0, len(s.backends))
	for _, b := range s.backends {
		out = append(out, b)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].ID < out[j].ID
		}
		return out[i].Priority < out[j].Priority
	})
	return out
}

func (s *Store) SetBackendEnabled(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.backends[id]
	if !ok {
		return ErrNotFound
	}
	b.Enabled = enabled
	b.UpdatedAt = time.Now().UTC()
	s.backends[id] = b
	return nil
}

func (s *Store) SetBackendPriority(id string, priority int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.backends[id]
	if !ok {
		return ErrNotFound
	}
	b.Priority = priority
	b.UpdatedAt = time.Now().UTC()
	s.backends[id] = b
	return nil
}

func (s *Store) SetBackendPreferred(id string, preferred bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.backends[id]
	if !ok {
		return ErrNotFound
	}
	if b.Config == nil {
		b.Config = map[string]any{}
	}
	b.Config["preferred"] = preferred
	b.UpdatedAt = time.Now().UTC()
	s.backends[id] = b
	return nil
}

func (s *Store) SetBackendReliability(id string, score float64, tier DispatchTier) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.backends[id]
	if !ok {
		return ErrNotFound
	}
	b.ReliabilityScore = score
	b.Tier = tier
	b.UpdatedAt = time.Now().UTC()
	s.backends[id] = b
	return nil
}

func (s *Store) UpsertMCPServer(rec MCPServerRecord) MCPServerRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	existing, ok := s.mcpServers[rec.ID]
	if !ok {
		rec.CreatedAt = now
	} else {
		rec.CreatedAt = existing.CreatedAt
		if rec.EnvMapping == nil {
			rec.EnvMapping = existing.EnvMapping
		}
	}
	if rec.EnvSchema == nil {
		rec.EnvSchema = map[string]string{}
	}
	if rec.EnvMapping == nil {
		rec.EnvMapping = map[string]string{}
	}
	rec.UpdatedAt = now
	s.mcpServers[rec.ID] = rec
	return rec
}

func (s *Store) ListMCPServers() []MCPServerRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MCPServerRecord, 0, len(s.mcpServers))
	for _, m := range s.mcpServers {
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) GetMCPServer(id string) (MCPServerRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.mcpServers[id]
	if !ok {
		return MCPServerRecord{}, ErrNotFound
	}
	return rec, nil
}

func (s *Store) SetMCPEnabled(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.mcpServers[id]
	if !ok {
		return ErrNotFound
	}
	m.Enabled = enabled
	m.UpdatedAt = time.Now().UTC()
	s.mcpServers[id] = m
	return nil
}

func (s *Store) SetMCPEnvMapping(id string, mapping map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.mcpServers[id]
	if !ok {
		return ErrNotFound
	}
	if m.EnvMapping == nil {
		m.EnvMapping = map[string]string{}
	}
	for k, v := range mapping {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		m.EnvMapping[k] = v
	}
	m.UpdatedAt = time.Now().UTC()
	s.mcpServers[id] = m
	return nil
}

func (s *Store) CreateOrGetSearchRequest(requestKey string, query QuerySpec, maxAttempts int) SearchRequestRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.requestKeyToID[requestKey]; ok {
		return s.searchRequests[id]
	}
	now := time.Now().UTC()
	rec := SearchRequestRecord{
		ID:           s.nextRequestID,
		RequestKey:   requestKey,
		EntityType:   query.EntityType,
		EntityID:     query.EntityID,
		Query:        query,
		Status:       "queued",
		AttemptCount: 0,
		MaxAttempts:  maxAttempts,
		NotBefore:    now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.nextRequestID++
	s.searchRequests[rec.ID] = rec
	s.requestKeyToID[requestKey] = rec.ID
	return rec
}

func (s *Store) GetSearchRequest(id int64) (SearchRequestRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.searchRequests[id]
	if !ok {
		return SearchRequestRecord{}, ErrNotFound
	}
	return rec, nil
}

func (s *Store) TryLockNextSearchRequest(workerID string, now time.Time) (SearchRequestRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var selected *SearchRequestRecord
	for id := range s.searchRequests {
		rec := s.searchRequests[id]
		if rec.Status != "queued" && rec.Status != "running" {
			continue
		}
		if rec.Status == "running" && rec.LockedAt != nil {
			continue
		}
		if rec.NotBefore.After(now) {
			continue
		}
		if selected == nil || rec.CreatedAt.Before(selected.CreatedAt) {
			tmp := rec
			selected = &tmp
		}
	}
	if selected == nil {
		return SearchRequestRecord{}, false, nil
	}
	rec := *selected
	rec.Status = "running"
	lockedAt := now.UTC()
	rec.LockedAt = &lockedAt
	rec.LockedBy = workerID
	leaseExpiresAt := lockedAt.Add(SearchRequestLeaseTTL)
	rec.LeaseExpiresAt = &leaseExpiresAt
	rec.AttemptCount++
	rec.UpdatedAt = lockedAt
	s.searchRequests[rec.ID] = rec
	return rec, true, nil
}

func (s *Store) RecoverExpiredSearchRequests(now time.Time, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	recovered := 0
	for id := range s.searchRequests {
		if recovered >= limit {
			break
		}
		rec := s.searchRequests[id]
		if rec.Status != "running" || rec.LeaseExpiresAt == nil || rec.LeaseExpiresAt.After(now) {
			continue
		}
		nextAttempt := rec.AttemptCount + 1
		rec.AttemptCount = nextAttempt
		rec.LastError = "lease expired; recovered"
		rec.LockedAt = nil
		rec.LockedBy = ""
		rec.LeaseExpiresAt = nil
		rec.UpdatedAt = now.UTC()
		if nextAttempt >= rec.MaxAttempts {
			rec.Status = "failed"
			rec.NotBefore = now.UTC()
		} else {
			rec.Status = "queued"
			rec.NotBefore = now.UTC().Add(backoffForAttempt(nextAttempt))
		}
		s.searchRequests[id] = rec
		recovered++
	}
	return recovered, nil
}

func (s *Store) RescheduleSearchRequest(id int64, lastErr string, notBefore time.Time, terminal bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.searchRequests[id]
	if !ok {
		return ErrNotFound
	}
	if terminal || rec.AttemptCount >= rec.MaxAttempts {
		rec.Status = "failed"
	} else {
		rec.Status = "queued"
	}
	rec.LastError = lastErr
	rec.NotBefore = notBefore.UTC()
	rec.LockedAt = nil
	rec.LockedBy = ""
	rec.LeaseExpiresAt = nil
	rec.UpdatedAt = time.Now().UTC()
	s.searchRequests[id] = rec
	return nil
}

func (s *Store) MarkSearchRequestSucceeded(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.searchRequests[id]
	if !ok {
		return ErrNotFound
	}
	rec.Status = "succeeded"
	rec.LastError = ""
	rec.LockedAt = nil
	rec.LockedBy = ""
	rec.LeaseExpiresAt = nil
	rec.UpdatedAt = time.Now().UTC()
	s.searchRequests[id] = rec
	return nil
}

func (s *Store) ReplaceCandidates(requestID int64, candidates []Candidate) ([]CandidateRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.searchRequests[requestID]; !ok {
		return nil, ErrNotFound
	}
	s.candidatesByReq[requestID] = nil
	out := make([]CandidateRecord, 0, len(candidates))
	now := time.Now().UTC()
	for _, c := range candidates {
		rec := CandidateRecord{
			ID:              s.nextCandidateID,
			SearchRequestID: requestID,
			Candidate:       c,
			CreatedAt:       now,
		}
		s.nextCandidateID++
		s.candidatesByReq[requestID] = append(s.candidatesByReq[requestID], rec)
		s.candidateByID[rec.ID] = rec
		out = append(out, rec)
	}
	return out, nil
}

func (s *Store) ListCandidates(requestID int64, limit int) ([]CandidateRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	recs, ok := s.candidatesByReq[requestID]
	if !ok {
		return nil, ErrNotFound
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if len(recs) < limit {
		limit = len(recs)
	}
	return append([]CandidateRecord(nil), recs[:limit]...), nil
}

func (s *Store) GetCandidateByID(id int64) (CandidateRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.candidateByID[id]
	if !ok {
		return CandidateRecord{}, ErrNotFound
	}
	return rec, nil
}

func (s *Store) CreateGrab(candidateID int64, entityType string, entityID string) (GrabRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.candidateByID[candidateID]; !ok {
		return GrabRecord{}, ErrNotFound
	}
	now := time.Now().UTC()
	rec := GrabRecord{
		ID:          s.nextGrabID,
		CandidateID: candidateID,
		EntityType:  entityType,
		EntityID:    entityID,
		Status:      "created",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.nextGrabID++
	s.grabsByID[rec.ID] = rec
	return rec, nil
}

func (s *Store) GetGrabByID(id int64) (GrabRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.grabsByID[id]
	if !ok {
		return GrabRecord{}, ErrNotFound
	}
	return rec, nil
}

func (s *Store) SetWantedWork(rec WantedWorkRecord) (WantedWorkRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(rec.WorkID) == "" {
		return WantedWorkRecord{}, ErrNotFound
	}
	now := time.Now().UTC()
	existing, ok := s.wantedWorks[rec.WorkID]
	if ok {
		rec.CreatedAt = existing.CreatedAt
		rec.LastEnqueuedAt = existing.LastEnqueuedAt
	} else {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	if rec.CadenceMinutes <= 0 {
		rec.CadenceMinutes = 60
	}
	s.wantedWorks[rec.WorkID] = rec
	return rec, nil
}

func (s *Store) ListWantedWorks() []WantedWorkRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]WantedWorkRecord, 0, len(s.wantedWorks))
	for _, rec := range s.wantedWorks {
		out = append(out, rec)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].WorkID < out[j].WorkID
		}
		return out[i].Priority < out[j].Priority
	})
	return out
}

func (s *Store) DeleteWantedWork(workID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.wantedWorks[workID]; !ok {
		return ErrNotFound
	}
	delete(s.wantedWorks, workID)
	return nil
}

func (s *Store) ListDueWantedWorks(now time.Time) []WantedWorkRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	due := make([]WantedWorkRecord, 0)
	for _, rec := range s.wantedWorks {
		if !rec.Enabled {
			continue
		}
		if rec.LastEnqueuedAt == nil || rec.LastEnqueuedAt.Add(time.Duration(rec.CadenceMinutes)*time.Minute).Before(now) || rec.LastEnqueuedAt.Add(time.Duration(rec.CadenceMinutes)*time.Minute).Equal(now) {
			due = append(due, rec)
		}
	}
	sort.SliceStable(due, func(i, j int) bool {
		if due[i].Priority == due[j].Priority {
			return due[i].WorkID < due[j].WorkID
		}
		return due[i].Priority < due[j].Priority
	})
	return due
}

func (s *Store) MarkWantedWorkEnqueued(workID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.wantedWorks[workID]
	if !ok {
		return ErrNotFound
	}
	ts := now.UTC()
	rec.LastEnqueuedAt = &ts
	rec.UpdatedAt = ts
	s.wantedWorks[workID] = rec
	return nil
}

func (s *Store) SetWantedAuthor(rec WantedAuthorRecord) (WantedAuthorRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(rec.AuthorID) == "" {
		return WantedAuthorRecord{}, ErrNotFound
	}
	now := time.Now().UTC()
	existing, ok := s.wantedAuthors[rec.AuthorID]
	if ok {
		rec.CreatedAt = existing.CreatedAt
		rec.LastEnqueuedAt = existing.LastEnqueuedAt
	} else {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	if rec.CadenceMinutes <= 0 {
		rec.CadenceMinutes = 60
	}
	s.wantedAuthors[rec.AuthorID] = rec
	return rec, nil
}

func (s *Store) ListWantedAuthors() []WantedAuthorRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]WantedAuthorRecord, 0, len(s.wantedAuthors))
	for _, rec := range s.wantedAuthors {
		out = append(out, rec)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority == out[j].Priority {
			return out[i].AuthorID < out[j].AuthorID
		}
		return out[i].Priority < out[j].Priority
	})
	return out
}

func (s *Store) DeleteWantedAuthor(authorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.wantedAuthors[authorID]; !ok {
		return ErrNotFound
	}
	delete(s.wantedAuthors, authorID)
	return nil
}

func (s *Store) ListDueWantedAuthors(now time.Time) []WantedAuthorRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	due := make([]WantedAuthorRecord, 0)
	for _, rec := range s.wantedAuthors {
		if !rec.Enabled {
			continue
		}
		if rec.LastEnqueuedAt == nil || rec.LastEnqueuedAt.Add(time.Duration(rec.CadenceMinutes)*time.Minute).Before(now) || rec.LastEnqueuedAt.Add(time.Duration(rec.CadenceMinutes)*time.Minute).Equal(now) {
			due = append(due, rec)
		}
	}
	sort.SliceStable(due, func(i, j int) bool {
		if due[i].Priority == due[j].Priority {
			return due[i].AuthorID < due[j].AuthorID
		}
		return due[i].Priority < due[j].Priority
	})
	return due
}

func (s *Store) MarkWantedAuthorEnqueued(authorID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.wantedAuthors[authorID]
	if !ok {
		return ErrNotFound
	}
	ts := now.UTC()
	rec.LastEnqueuedAt = &ts
	rec.UpdatedAt = ts
	s.wantedAuthors[authorID] = rec
	return nil
}

func (s *Store) PruneStaleCandidates(maxPerRequest int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if maxPerRequest <= 0 {
		maxPerRequest = 50
	}
	pruned := 0
	for requestID, recs := range s.candidatesByReq {
		if len(recs) <= maxPerRequest {
			continue
		}
		sort.SliceStable(recs, func(i, j int) bool {
			if recs[i].CreatedAt.Equal(recs[j].CreatedAt) {
				return recs[i].ID > recs[j].ID
			}
			return recs[i].CreatedAt.After(recs[j].CreatedAt)
		})
		for _, rec := range recs[maxPerRequest:] {
			delete(s.candidateByID, rec.ID)
			pruned++
		}
		s.candidatesByReq[requestID] = append([]CandidateRecord(nil), recs[:maxPerRequest]...)
	}
	return pruned, nil
}

func (s *Store) RecordBackendSearchResult(backendID string, success bool, latency time.Duration, yielded bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.backends[backendID]; !ok {
		return ErrNotFound
	}
	m := s.backendMetrics[backendID]
	m.SearchCount++
	m.TotalLatencyMS += latency.Milliseconds()
	if success {
		m.SuccessCount++
	} else {
		m.FailureCount++
	}
	if yielded {
		m.CandidateYieldCount++
	}
	s.backendMetrics[backendID] = m
	return nil
}

func (s *Store) RecomputeReliability() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for backendID, m := range s.backendMetrics {
		rec, ok := s.backends[backendID]
		if !ok {
			continue
		}
		availability := 0.70
		totalOutcomes := m.SuccessCount + m.FailureCount
		if totalOutcomes > 0 {
			availability = float64(m.SuccessCount) / float64(totalOutcomes)
		}

		latencyScore := 0.70
		if m.SearchCount > 0 {
			avgLatencyMS := float64(m.TotalLatencyMS) / float64(m.SearchCount)
			latencyScore = 1.0 - (avgLatencyMS / 5000.0)
			if latencyScore < 0.0 {
				latencyScore = 0.0
			}
		}

		yieldScore := 0.70
		if m.SearchCount > 0 {
			yieldScore = float64(m.CandidateYieldCount) / float64(m.SearchCount)
		}

		composite := (availability * 0.50) + (latencyScore * 0.30) + (yieldScore * 0.20)
		rec.ReliabilityScore = composite
		rec.Tier = tierForReliability(composite)
		rec.UpdatedAt = time.Now().UTC()
		s.backends[backendID] = rec
	}
	return nil
}

func tierForReliability(score float64) DispatchTier {
	switch {
	case score >= 0.85:
		return TierPrimary
	case score >= 0.70:
		return TierSecondary
	case score >= 0.50:
		return TierFallback
	default:
		return TierQuarantine
	}
}
