package download

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
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
	filename := nzbFilenameFromURI(uri)
	content, err := c.fetchNZB(ctx, uri)
	if err != nil {
		return "", err
	}
	params := []any{
		filename,
		base64.StdEncoding.EncodeToString(content),
		category,
		0,
		false,
		false,
		"",
		0,
		"SCORE",
		[]any{},
	}
	var result int
	if err := c.rpc(ctx, "append", params, &result); err != nil {
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
		if strings.TrimSpace(fmt.Sprintf("%v", group["NZBID"])) != target {
			continue
		}
		return statusFromNZBGetRecord(group), nil
	}

	var historyResp struct {
		Result []map[string]any `json:"result"`
	}
	if err := c.rpc(ctx, "history", []any{false}, &historyResp.Result); err != nil {
		return DownloadStatus{}, err
	}
	for _, item := range historyResp.Result {
		if strings.TrimSpace(fmt.Sprintf("%v", item["NZBID"])) != target && strings.TrimSpace(fmt.Sprintf("%v", item["ID"])) != target {
			continue
		}
		return statusFromNZBGetRecord(item), nil
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

func (c *NZBGetClient) fetchNZB(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("nzb fetch status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nzb fetch returned empty body")
	}
	return body, nil
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

func statusFromNZBGetRecord(record map[string]any) DownloadStatus {
	progress := 0.0
	remaining := toFloat(record["RemainingSizeMB"])
	downloaded := toFloat(record["DownloadedSizeMB"])
	total := remaining + downloaded
	if total > 0 {
		progress = (downloaded / total) * 100.0
	}
	outputPath := firstNonEmpty(toString(record["FinalDir"]), toString(record["DestDir"]), toString(record["Directory"]))
	return DownloadStatus{
		Client:     "nzbget",
		ID:         strings.TrimSpace(fmt.Sprintf("%v", firstNonNil(record["NZBID"], record["ID"]))),
		State:      normalizeNZBGetState(toString(record["Status"]), progress),
		Progress:   progress,
		OutputPath: outputPath,
		Raw:        record,
	}
}

func normalizeNZBGetState(status string, progress float64) string {
	lower := strings.ToLower(strings.TrimSpace(status))
	switch {
	case strings.Contains(lower, "dupe") || strings.Contains(lower, "deleted"):
		return "canceled"
	case strings.Contains(lower, "failure"):
		return "failed"
	case strings.Contains(lower, "repair") || strings.Contains(lower, "par"):
		return "repairing"
	case strings.Contains(lower, "unpack") || strings.Contains(lower, "move") || strings.Contains(lower, "pp"):
		return "unpacking"
	case strings.Contains(lower, "paused"):
		return "submitted"
	case strings.Contains(lower, "success"):
		return "completed"
	case progress >= 100:
		return "downloading"
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

func firstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return ""
}

func atoiSafe(v string) int {
	var out int
	_, _ = fmt.Sscanf(strings.TrimSpace(v), "%d", &out)
	return out
}

func nzbFilenameFromURI(raw string) string {
	fallback := "download.nzb"
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	if base := strings.TrimSpace(path.Base(u.Path)); strings.HasSuffix(strings.ToLower(base), ".nzb") {
		return base
	}
	if file := strings.TrimSpace(u.Query().Get("file")); file != "" {
		clean := strings.NewReplacer("/", "_", "\\", "_", ":", "-", "*", "_", "?", "", "\"", "", "<", "", ">", "", "|", "_").Replace(file)
		if !strings.HasSuffix(strings.ToLower(clean), ".nzb") {
			clean += ".nzb"
		}
		return clean
	}
	return fallback
}
