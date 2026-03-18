package bookwyrm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type Options struct {
	APIKey  string
	Timeout time.Duration
}

type SearchResponse struct {
	Works []map[string]any `json:"works"`
}

type WorkResponse struct {
	Work map[string]any `json:"work"`
}

type QualityReportResponse struct {
	Report map[string]any `json:"report"`
}

type QualityRepairRequest struct {
	Limit                    int   `json:"limit,omitempty"`
	DryRun                   bool  `json:"dry_run"`
	RemoveInvalidIdentifiers *bool `json:"remove_invalid_identifiers,omitempty"`
}

type QualityRepairResponse struct {
	Result map[string]any `json:"result"`
}

func NewClient(baseURL string, opts Options) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		baseURL:    baseURL,
		apiKey:     strings.TrimSpace(opts.APIKey),
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Search(ctx context.Context, query string) (*SearchResponse, error) {
	endpoint := c.baseURL + "/v1/search?q=" + url.QueryEscape(query)
	var out SearchResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetWork(ctx context.Context, id string) (*WorkResponse, error) {
	endpoint := c.baseURL + "/v1/work/" + url.PathEscape(id)
	var out WorkResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetQualityReport(ctx context.Context, limit int) (*QualityReportResponse, error) {
	endpoint := c.baseURL + "/v1/quality/report"
	if limit > 0 {
		endpoint += "?limit=" + fmt.Sprintf("%d", limit)
	}
	var out QualityReportResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RepairQuality(ctx context.Context, req QualityRepairRequest) (*QualityRepairResponse, error) {
	endpoint := c.baseURL + "/v1/quality/repair"
	var out QualityRepairResponse
	if err := c.doJSON(ctx, http.MethodPost, endpoint, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) doJSON(ctx context.Context, method string, endpoint string, body any, out any) error {
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = encoded
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		if errBody.Error != "" {
			return fmt.Errorf("api error (%d): %s", resp.StatusCode, errBody.Error)
		}
		return fmt.Errorf("api error (%d)", resp.StatusCode)
	}

	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
