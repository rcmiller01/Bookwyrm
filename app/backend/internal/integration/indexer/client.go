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
