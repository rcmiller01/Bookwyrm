package indexer

import "time"

type Storage interface {
	UpsertBackend(rec BackendRecord) BackendRecord
	ListBackends() []BackendRecord
	SetBackendEnabled(id string, enabled bool) error
	SetBackendPriority(id string, priority int) error
	SetBackendReliability(id string, score float64, tier DispatchTier) error

	UpsertMCPServer(rec MCPServerRecord) MCPServerRecord
	ListMCPServers() []MCPServerRecord
	GetMCPServer(id string) (MCPServerRecord, error)
	SetMCPEnabled(id string, enabled bool) error
	SetMCPEnvMapping(id string, mapping map[string]string) error

	CreateOrGetSearchRequest(requestKey string, query QuerySpec, maxAttempts int) SearchRequestRecord
	GetSearchRequest(id int64) (SearchRequestRecord, error)
	TryLockNextSearchRequest(workerID string, now time.Time) (SearchRequestRecord, bool, error)
	RecoverExpiredSearchRequests(now time.Time, limit int) (int, error)
	RescheduleSearchRequest(id int64, lastErr string, notBefore time.Time, terminal bool) error
	MarkSearchRequestSucceeded(id int64) error

	ReplaceCandidates(requestID int64, candidates []Candidate) ([]CandidateRecord, error)
	ListCandidates(requestID int64, limit int) ([]CandidateRecord, error)
	GetCandidateByID(id int64) (CandidateRecord, error)
	CreateGrab(candidateID int64, entityType string, entityID string) (GrabRecord, error)
	GetGrabByID(id int64) (GrabRecord, error)
	SetWantedWork(rec WantedWorkRecord) (WantedWorkRecord, error)
	ListWantedWorks() []WantedWorkRecord
	DeleteWantedWork(workID string) error
	ListDueWantedWorks(now time.Time) []WantedWorkRecord
	MarkWantedWorkEnqueued(workID string, now time.Time) error
	SetWantedAuthor(rec WantedAuthorRecord) (WantedAuthorRecord, error)
	ListWantedAuthors() []WantedAuthorRecord
	DeleteWantedAuthor(authorID string) error
	ListDueWantedAuthors(now time.Time) []WantedAuthorRecord
	MarkWantedAuthorEnqueued(authorID string, now time.Time) error
	PruneStaleCandidates(maxPerRequest int) (int, error)

	RecordBackendSearchResult(backendID string, success bool, latency time.Duration, yielded bool) error
	RecomputeReliability() error
}
