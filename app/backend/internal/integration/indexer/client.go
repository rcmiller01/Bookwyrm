package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

type SearchRequest struct {
	Metadata              map[string]any `json:"metadata"`
	RequestedCapabilities []string       `json:"requested_capabilities,omitempty"`
	Priority              string         `json:"priority,omitempty"`
	PolicyProfile         string         `json:"policy_profile,omitempty"`
	BackendGroups         []string       `json:"backend_groups,omitempty"`
}

type GrabRecord struct {
	ID          int64  `json:"id"`
	CandidateID int64  `json:"candidate_id"`
	EntityType  string `json:"entity_type"`
	EntityID    string `json:"entity_id"`
	Status      string `json:"status"`
}

type CandidateRecord struct {
	ID        int64 `json:"id"`
	Candidate struct {
		Protocol    string         `json:"protocol"`
		GrabPayload map[string]any `json:"grab_payload"`
	} `json:"candidate"`
}

type DiagnosticsStats struct {
	SearchesExecuted    int64 `json:"searches_executed"`
	CandidatesEvaluated int64 `json:"candidates_evaluated"`
	GrabsPerformed      int64 `json:"grabs_performed"`
}

func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		apiKey:     strings.TrimSpace(cfg.APIKey),
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Search(ctx context.Context, reqBody SearchRequest) (map[string]any, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/indexer/search", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		if msg, ok := parsed["error"].(string); ok && msg != "" {
			return nil, fmt.Errorf("indexer-service error (%d): %s", resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("indexer-service error (%d)", resp.StatusCode)
	}
	return parsed, nil
}

func (c *Client) GetGrab(ctx context.Context, grabID int64) (GrabRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/indexer/grabs/%d", c.baseURL, grabID), nil)
	if err != nil {
		return GrabRecord{}, err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return GrabRecord{}, err
	}
	defer resp.Body.Close()
	var parsed map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return GrabRecord{}, err
	}
	if resp.StatusCode >= 400 {
		return GrabRecord{}, fmt.Errorf("indexer-service error (%d)", resp.StatusCode)
	}
	raw, ok := parsed["grab"].(map[string]any)
	if !ok {
		return GrabRecord{}, fmt.Errorf("invalid indexer grab response")
	}
	return GrabRecord{
		ID:          toInt64(raw["id"]),
		CandidateID: toInt64(raw["candidate_id"]),
		EntityType:  toString(raw["entity_type"]),
		EntityID:    toString(raw["entity_id"]),
		Status:      toString(raw["status"]),
	}, nil
}

func (c *Client) GetCandidate(ctx context.Context, candidateID int64) (CandidateRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/indexer/candidates/id/%d", c.baseURL, candidateID), nil)
	if err != nil {
		return CandidateRecord{}, err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return CandidateRecord{}, err
	}
	defer resp.Body.Close()
	var parsed map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return CandidateRecord{}, err
	}
	if resp.StatusCode >= 400 {
		return CandidateRecord{}, fmt.Errorf("indexer-service error (%d)", resp.StatusCode)
	}
	raw, ok := parsed["candidate"].(map[string]any)
	if !ok {
		return CandidateRecord{}, fmt.Errorf("invalid indexer candidate response")
	}
	record := CandidateRecord{
		ID: toInt64(raw["id"]),
	}
	cand, _ := raw["candidate"].(map[string]any)
	record.Candidate.Protocol = toString(cand["protocol"])
	record.Candidate.GrabPayload, _ = cand["grab_payload"].(map[string]any)
	if record.Candidate.GrabPayload == nil {
		record.Candidate.GrabPayload = map[string]any{}
	}
	return record, nil
}

func (c *Client) GetStats(ctx context.Context) (DiagnosticsStats, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/indexer/stats", nil)
	if err != nil {
		return DiagnosticsStats{}, err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DiagnosticsStats{}, err
	}
	defer resp.Body.Close()

	var parsed struct {
		Stats DiagnosticsStats `json:"stats"`
		Error string           `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return DiagnosticsStats{}, err
	}
	if resp.StatusCode >= 400 {
		if parsed.Error != "" {
			return DiagnosticsStats{}, fmt.Errorf("indexer-service error (%d): %s", resp.StatusCode, parsed.Error)
		}
		return DiagnosticsStats{}, fmt.Errorf("indexer-service error (%d)", resp.StatusCode)
	}
	return parsed.Stats, nil
}

func toInt64(v any) int64 {
	switch vv := v.(type) {
	case float64:
		return int64(vv)
	case int64:
		return vv
	case int:
		return int64(vv)
	default:
		return 0
	}
}

func toString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
