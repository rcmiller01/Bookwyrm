package download

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SABnzbdConfig struct {
	BaseURL  string
	APIKey   string
	Category string
	Timeout  time.Duration
}

type SABnzbdClient struct {
	baseURL         string
	apiKey          string
	defaultCategory string
	httpClient      *http.Client
}

func NewSABnzbdClient(cfg SABnzbdConfig) *SABnzbdClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &SABnzbdClient{
		baseURL:         strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		apiKey:          strings.TrimSpace(cfg.APIKey),
		defaultCategory: strings.TrimSpace(cfg.Category),
		httpClient:      &http.Client{Timeout: timeout},
	}
}

func (c *SABnzbdClient) Name() string { return "sabnzbd" }

func (c *SABnzbdClient) AddDownload(ctx context.Context, req AddRequest) (string, error) {
	if strings.TrimSpace(req.URI) == "" {
		return "", fmt.Errorf("download uri is required")
	}
	params := c.baseParams()
	params.Set("mode", "addurl")
	params.Set("name", req.URI)
	category := req.Category
	if category == "" {
		category = c.defaultCategory
	}
	if category != "" {
		params.Set("cat", category)
	}
	if len(req.Tags) > 0 {
		params.Set("script", strings.Join(req.Tags, ","))
	}
	endpoint := c.baseURL + "/api?" + params.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("sabnzbd add status %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	ids := extractNzoIDs(payload)
	if len(ids) > 0 {
		return ids[0], nil
	}
	return deriveSyntheticID("sab", req.URI), nil
}

func (c *SABnzbdClient) GetStatus(ctx context.Context, downloadID string) (DownloadStatus, error) {
	params := c.baseParams()
	params.Set("mode", "queue")
	params.Set("search", downloadID)
	endpoint := c.baseURL + "/api?" + params.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return DownloadStatus{}, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return DownloadStatus{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return DownloadStatus{}, fmt.Errorf("sabnzbd queue status %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return DownloadStatus{}, err
	}
	slot, ok := findSABSlot(payload, downloadID)
	if !ok {
		return DownloadStatus{}, ErrDownloadNotFound
	}
	state := normalizeSABState(toString(slot["status"]), toString(slot["mbleft"]))
	progress := parsePercent(slot["percentage"])
	return DownloadStatus{
		Client:   c.Name(),
		ID:       downloadID,
		State:    state,
		Progress: progress,
		Raw:      slot,
	}, nil
}

func (c *SABnzbdClient) Remove(ctx context.Context, downloadID string, _ bool) error {
	params := c.baseParams()
	params.Set("mode", "queue")
	params.Set("name", "delete")
	params.Set("value", downloadID)
	endpoint := c.baseURL + "/api?" + params.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("sabnzbd delete status %d", resp.StatusCode)
	}
	return nil
}

func (c *SABnzbdClient) baseParams() url.Values {
	values := url.Values{}
	values.Set("apikey", c.apiKey)
	values.Set("output", "json")
	return values
}

func extractNzoIDs(payload map[string]any) []string {
	ids := []string{}
	idList, ok := payload["nzo_ids"].([]any)
	if !ok {
		return ids
	}
	for _, item := range idList {
		if id, ok := item.(string); ok && strings.TrimSpace(id) != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func findSABSlot(payload map[string]any, downloadID string) (map[string]any, bool) {
	queue, ok := payload["queue"].(map[string]any)
	if !ok {
		return nil, false
	}
	slots, ok := queue["slots"].([]any)
	if !ok {
		return nil, false
	}
	for _, item := range slots {
		slot, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(toString(slot["nzo_id"])) == strings.TrimSpace(downloadID) {
			return slot, true
		}
	}
	return nil, false
}

func normalizeSABState(status string, mbLeft string) string {
	if strings.EqualFold(strings.TrimSpace(status), "Paused") {
		return "paused"
	}
	if strings.TrimSpace(mbLeft) == "0.00" || strings.TrimSpace(mbLeft) == "0" {
		return "completed"
	}
	return "downloading"
}

func parsePercent(v any) float64 {
	switch value := v.(type) {
	case string:
		trimmed := strings.TrimSpace(strings.TrimSuffix(value, "%"))
		var out float64
		_, _ = fmt.Sscanf(trimmed, "%f", &out)
		return out
	case float64:
		return value
	default:
		return 0
	}
}
