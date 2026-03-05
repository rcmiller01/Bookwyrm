package download

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

type QBitTorrentConfig struct {
	BaseURL  string
	Username string
	Password string
	Timeout  time.Duration
}

type QBitTorrentClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	mu         sync.Mutex
	loggedIn   bool
}

func NewQBitTorrentClient(cfg QBitTorrentConfig) *QBitTorrentClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	jar, _ := cookiejar.New(nil)
	return &QBitTorrentClient{
		baseURL:  strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		username: cfg.Username,
		password: cfg.Password,
		httpClient: &http.Client{
			Timeout: timeout,
			Jar:     jar,
		},
	}
}

func (c *QBitTorrentClient) Name() string { return "qbittorrent" }

func (c *QBitTorrentClient) AddDownload(ctx context.Context, req AddRequest) (string, error) {
	if strings.TrimSpace(req.URI) == "" {
		return "", fmt.Errorf("download uri is required")
	}
	if err := c.login(ctx); err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("urls", req.URI)
	if req.Category != "" {
		form.Set("category", req.Category)
	}
	if len(req.Tags) > 0 {
		form.Set("tags", strings.Join(req.Tags, ","))
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/add", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("qbittorrent add status %d", resp.StatusCode)
	}
	return deriveSyntheticID("qb", req.URI), nil
}

func (c *QBitTorrentClient) GetStatus(ctx context.Context, downloadID string) (DownloadStatus, error) {
	if strings.TrimSpace(downloadID) == "" {
		return DownloadStatus{}, fmt.Errorf("download_id required")
	}
	if err := c.login(ctx); err != nil {
		return DownloadStatus{}, err
	}
	endpoint := c.baseURL + "/api/v2/torrents/info?hashes=" + url.QueryEscape(downloadID)
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
		return DownloadStatus{}, fmt.Errorf("qbittorrent status %d", resp.StatusCode)
	}
	var rows []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return DownloadStatus{}, err
	}
	if len(rows) == 0 {
		return DownloadStatus{}, ErrDownloadNotFound
	}
	row := rows[0]
	state := strings.TrimSpace(toString(row["state"]))
	progress := toFloat(row["progress"])
	return DownloadStatus{
		Client:   c.Name(),
		ID:       downloadID,
		State:    normalizeQBitState(state, progress),
		Progress: progress * 100.0,
		Raw:      row,
	}, nil
}

func (c *QBitTorrentClient) Remove(ctx context.Context, downloadID string, deleteFiles bool) error {
	if err := c.login(ctx); err != nil {
		return err
	}
	form := url.Values{}
	form.Set("hashes", downloadID)
	if deleteFiles {
		form.Set("deleteFiles", "true")
	} else {
		form.Set("deleteFiles", "false")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/delete", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qbittorrent remove status %d", resp.StatusCode)
	}
	return nil
}

func (c *QBitTorrentClient) login(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loggedIn {
		return nil
	}
	if c.baseURL == "" {
		return fmt.Errorf("qbittorrent base url missing")
	}
	form := url.Values{}
	form.Set("username", c.username)
	form.Set("password", c.password)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qbittorrent login status %d", resp.StatusCode)
	}
	c.loggedIn = true
	return nil
}

func normalizeQBitState(state string, progress float64) string {
	lower := strings.ToLower(state)
	switch {
	case strings.Contains(lower, "error"):
		return "failed"
	case progress >= 1.0 || strings.Contains(lower, "upload"):
		return "completed"
	case strings.Contains(lower, "paused"):
		return "paused"
	default:
		return "downloading"
	}
}

func deriveSyntheticID(prefix string, value string) string {
	hash := sha1.Sum([]byte(strings.TrimSpace(value)))
	return prefix + "-" + hex.EncodeToString(hash[:8])
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}

func toFloat(v any) float64 {
	switch value := v.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	default:
		return 0
	}
}
