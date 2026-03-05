package indexer

import "time"

type MetadataSnapshot struct {
	WorkID          string   `json:"work_id"`
	EditionID       string   `json:"edition_id,omitempty"`
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
	CandidateID     string   `json:"candidate_id"`
	Title           string   `json:"title"`
	Format          string   `json:"format,omitempty"`
	MatchConfidence float64  `json:"match_confidence"`
	ProviderLink    string   `json:"provider_link,omitempty"`
	Provenance      string   `json:"provenance"`
	ReasonCodes     []string `json:"reason_codes,omitempty"`
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
