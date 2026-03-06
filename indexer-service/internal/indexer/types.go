package indexer

import "time"

const SearchRequestLeaseTTL = 2 * time.Minute

type MetadataSnapshot struct {
	WorkID          string   `json:"work_id"`
	EditionID       string   `json:"edition_id,omitempty"`
	EntityType      string   `json:"entity_type,omitempty"`
	EntityID        string   `json:"entity_id,omitempty"`
	ISBN10          string   `json:"isbn_10,omitempty"`
	ISBN13          string   `json:"isbn_13,omitempty"`
	Title           string   `json:"title"`
	Authors         []string `json:"authors,omitempty"`
	Language        string   `json:"language,omitempty"`
	PublicationYear int      `json:"publication_year,omitempty"`
}

type SearchRequest struct {
	Metadata              MetadataSnapshot `json:"metadata"`
	RequestedCapabilities []string         `json:"requested_capabilities,omitempty"`
	Priority              string           `json:"priority,omitempty"`
	PolicyProfile         string           `json:"policy_profile,omitempty"`
	BackendGroups         []string         `json:"backend_groups,omitempty"`
}

type Candidate struct {
	CandidateID     string         `json:"candidate_id"`
	Title           string         `json:"title"`
	NormalizedTitle string         `json:"normalized_title,omitempty"`
	Format          string         `json:"format,omitempty"`
	Protocol        string         `json:"protocol,omitempty"`
	MatchConfidence float64        `json:"match_confidence"`
	Score           float64        `json:"score,omitempty"`
	ProviderLink    string         `json:"provider_link,omitempty"`
	Provenance      string         `json:"provenance"`
	ReasonCodes     []string       `json:"reason_codes,omitempty"`
	Reasons         []Reason       `json:"reasons,omitempty"`
	Identifiers     map[string]any `json:"identifiers,omitempty"`
	Attributes      map[string]any `json:"attributes,omitempty"`
	GrabPayload     map[string]any `json:"grab_payload,omitempty"`
	SourcePipeline  string         `json:"source_pipeline,omitempty"`
	SourceBackendID string         `json:"source_backend_id,omitempty"`
	SizeBytes       *int64         `json:"size_bytes,omitempty"`
	Seeders         *int           `json:"seeders,omitempty"`
	Leechers        *int           `json:"leechers,omitempty"`
	PublishedAt     *time.Time     `json:"published_at,omitempty"`
}

type AdapterTrace struct {
	Adapter string `json:"adapter"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

type SearchResult struct {
	WorkID     string         `json:"work_id"`
	Source     string         `json:"source"`
	Found      bool           `json:"found"`
	Candidates []Candidate    `json:"candidates"`
	SearchedAt time.Time      `json:"searched_at"`
	Trace      []AdapterTrace `json:"trace,omitempty"`
}

type AdapterStatus struct {
	Name         string   `json:"name"`
	Group        string   `json:"group"`
	Capabilities []string `json:"capabilities"`
	Healthy      bool     `json:"healthy"`
}

type BackendCapabilities struct {
	Protocols []string `json:"protocols,omitempty"`
	Supports  []string `json:"supports,omitempty"`
}

type Reason struct {
	Code    string  `json:"code"`
	Weight  float64 `json:"weight,omitempty"`
	Message string  `json:"message,omitempty"`
}

type QuerySpec struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`

	Title  string `json:"title,omitempty"`
	Author string `json:"author,omitempty"`
	ISBN   string `json:"isbn,omitempty"`
	DOI    string `json:"doi,omitempty"`

	Preferences struct {
		Formats   []string `json:"formats,omitempty"`
		Languages []string `json:"languages,omitempty"`
	} `json:"preferences,omitempty"`

	Limits struct {
		MaxCandidates int `json:"max_candidates,omitempty"`
		TimeoutSec    int `json:"timeout_sec,omitempty"`
	} `json:"limits,omitempty"`
}

type BackendType string

const (
	BackendTypeProwlarr BackendType = "prowlarr"
	BackendTypeMCP      BackendType = "mcp"
)

type DispatchTier string

const (
	TierPrimary      DispatchTier = "primary"
	TierSecondary    DispatchTier = "secondary"
	TierFallback     DispatchTier = "fallback"
	TierQuarantine   DispatchTier = "quarantine"
	TierUnclassified DispatchTier = "unclassified"
)

type BackendRecord struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	BackendType      BackendType    `json:"backend_type"`
	Enabled          bool           `json:"enabled"`
	Tier             DispatchTier   `json:"tier"`
	ReliabilityScore float64        `json:"reliability_score"`
	Priority         int            `json:"priority"`
	Config           map[string]any `json:"config_json,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type MCPServerRecord struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Source     string            `json:"source"`
	SourceRef  string            `json:"source_ref"`
	Enabled    bool              `json:"enabled"`
	BaseURL    string            `json:"base_url,omitempty"`
	EnvSchema  map[string]string `json:"env_schema,omitempty"`
	EnvMapping map[string]string `json:"env_mapping,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type SearchRequestRecord struct {
	ID             int64      `json:"id"`
	RequestKey     string     `json:"request_key"`
	EntityType     string     `json:"entity_type"`
	EntityID       string     `json:"entity_id"`
	Query          QuerySpec  `json:"query_json"`
	Status         string     `json:"status"`
	AttemptCount   int        `json:"attempt_count"`
	MaxAttempts    int        `json:"max_attempts"`
	LastError      string     `json:"last_error,omitempty"`
	NotBefore      time.Time  `json:"not_before"`
	LockedAt       *time.Time `json:"locked_at,omitempty"`
	LockedBy       string     `json:"locked_by,omitempty"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type CandidateRecord struct {
	ID              int64     `json:"id"`
	SearchRequestID int64     `json:"search_request_id"`
	Candidate       Candidate `json:"candidate"`
	CreatedAt       time.Time `json:"created_at"`
}

type GrabRecord struct {
	ID            int64     `json:"id"`
	CandidateID   int64     `json:"candidate_id"`
	EntityType    string    `json:"entity_type"`
	EntityID      string    `json:"entity_id"`
	Status        string    `json:"status"`
	DownstreamRef string    `json:"downstream_ref,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type WantedWorkRecord struct {
	WorkID         string     `json:"work_id"`
	Enabled        bool       `json:"enabled"`
	Priority       int        `json:"priority"`
	CadenceMinutes int        `json:"cadence_minutes"`
	ProfileID      string     `json:"profile_id,omitempty"`
	IgnoreUpgrades bool       `json:"ignore_upgrades,omitempty"`
	Formats        []string   `json:"formats,omitempty"`
	Languages      []string   `json:"languages,omitempty"`
	LastEnqueuedAt *time.Time `json:"last_enqueued_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type WantedAuthorRecord struct {
	AuthorID       string     `json:"author_id"`
	Enabled        bool       `json:"enabled"`
	Priority       int        `json:"priority"`
	CadenceMinutes int        `json:"cadence_minutes"`
	ProfileID      string     `json:"profile_id,omitempty"`
	Formats        []string   `json:"formats,omitempty"`
	Languages      []string   `json:"languages,omitempty"`
	LastEnqueuedAt *time.Time `json:"last_enqueued_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type ProfileRecord struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	CutoffQuality  string    `json:"cutoff_quality"`
	DefaultProfile bool      `json:"default_profile"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ProfileQualityRecord struct {
	ProfileID string `json:"profile_id"`
	Quality   string `json:"quality"`
	Rank      int    `json:"rank"`
}

type ProfileWithQualities struct {
	Profile   ProfileRecord          `json:"profile"`
	Qualities []ProfileQualityRecord `json:"qualities"`
}
