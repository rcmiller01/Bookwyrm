package download

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type NZBGetConfig struct {
	BaseURL  string
	Username string
	Password string
	Category string
	Timeout  time.Duration
}

type NZBGetClient struct {
	baseURL         string
	username        string
	password        string
	defaultCategory string
	httpClient      *http.Client
}

func NewNZBGetClient(cfg NZBGetConfig) *NZBGetClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &NZBGetClient{
		baseURL:         strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		username:        strings.TrimSpace(cfg.Username),
		password:        strings.TrimSpace(cfg.Password),
		defaultCategory: strings.TrimSpace(cfg.Category),
		httpClient:      &http.Client{Timeout: timeout},
	}
}

func (c *NZBGetClient) Name() string { return "nzbget" }

func (c *NZBGetClient) AddDownload(ctx context.Context, req AddRequest) (string, error) {
	uri := strings.TrimSpace(req.URI)
	if uri == "" {
		return "", fmt.Errorf("download uri is required")
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = c.defaultCategory
	}
	params := []any{
		uri,      // URL
		"",       // NZB content (unused for URL mode)
		category, // category
		0,        // priority
		false,    // add paused
		"",       // dupe key
		0,        // dupe score
		"SCORE",  // dupe mode
	}
	var result int
	if err := c.rpc(ctx, "appendurl", params, &result); err != nil {
		return "", err
	}
	if result <= 0 {
		return deriveSyntheticID("nzbget", uri), nil
	}
	return fmt.Sprintf("%d", result), nil
}

func (c *NZBGetClient) GetStatus(ctx context.Context, downloadID string) (DownloadStatus, error) {
	var groups []map[string]any
	if err := c.rpc(ctx, "listgroups", []any{}, &groups); err != nil {
		return DownloadStatus{}, err
	}
	target := strings.TrimSpace(downloadID)
	for _, group := range groups {
		id := fmt.Sprintf("%v", group["NZBID"])
		if strings.TrimSpace(id) != target {
			continue
		}
		progress := 0.0
		remaining := toFloat(group["RemainingSizeMB"])
		downloaded := toFloat(group["DownloadedSizeMB"])
		total := remaining + downloaded
		if total > 0 {
			progress = (downloaded / total) * 100.0
		}
		state := normalizeNZBGetState(toString(group["Status"]), progress)
		outputPath := firstNonEmpty(toString(group["DestDir"]), toString(group["FinalDir"]), toString(group["Directory"]))
		return DownloadStatus{
			Client:     c.Name(),
			ID:         id,
			State:      state,
			Progress:   progress,
			OutputPath: outputPath,
			Raw:        group,
		}, nil
	}
	return DownloadStatus{}, ErrDownloadNotFound
}

func (c *NZBGetClient) Remove(ctx context.Context, downloadID string, deleteFiles bool) error {
	action := "GroupDelete"
	if deleteFiles {
		action = "GroupDelete"
	}
	params := []any{
		action,
		"",
		[]int{atoiSafe(downloadID)},
	}
	var result bool
	if err := c.rpc(ctx, "editqueue", params, &result); err != nil {
		return err
	}
	if !result {
		return fmt.Errorf("nzbget editqueue returned false")
	}
	return nil
}

func (c *NZBGetClient) rpc(ctx context.Context, method string, params []any, out any) error {
	body := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/jsonrpc", strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("nzbget rpc status %d", resp.StatusCode)
	}
	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  any             `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("nzbget rpc %s error", method)
	}
	if out != nil {
		if err := json.Unmarshal(rpcResp.Result, out); err != nil {
			return err
		}
	}
	return nil
}

func normalizeNZBGetState(status string, progress float64) string {
	lower := strings.ToLower(strings.TrimSpace(status))
	switch {
	case strings.Contains(lower, "failure"):
		return "failed"
	case strings.Contains(lower, "repair"):
		return "repairing"
	case strings.Contains(lower, "unpack"):
		return "unpacking"
	case strings.Contains(lower, "paused"):
		return "submitted"
	case strings.Contains(lower, "success") || progress >= 100:
		return "completed"
	default:
		return "downloading"
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func atoiSafe(v string) int {
	var out int
	_, _ = fmt.Sscanf(strings.TrimSpace(v), "%d", &out)
	return out
}
