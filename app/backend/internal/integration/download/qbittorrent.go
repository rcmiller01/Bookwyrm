package download

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
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
	httpReq, err := c.buildAddRequest(ctx, req)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("qbittorrent add status %d", resp.StatusCode)
	}
	if hash := hashFromMagnet(req.URI); hash != "" {
		return hash, nil
	}
	if len(req.Tags) > 0 && strings.TrimSpace(req.Tags[0]) != "" {
		return "tag:" + strings.TrimSpace(req.Tags[0]), nil
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
	endpoint := c.baseURL + "/api/v2/torrents/info"
	if strings.HasPrefix(downloadID, "tag:") {
		tag := strings.TrimSpace(strings.TrimPrefix(downloadID, "tag:"))
		endpoint += "?tag=" + url.QueryEscape(tag)
	} else {
		endpoint += "?hashes=" + url.QueryEscape(downloadID)
	}
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
	if strings.HasPrefix(downloadID, "tag:") {
		// Prefer the most recently active torrent when looking up by tag.
		row = pickNewest(rows)
	}
	state := strings.TrimSpace(toString(row["state"]))
	progress := toFloat(row["progress"])
	outputPath := firstNonEmpty(
		toString(row["content_path"]),
		joinPath(toString(row["save_path"]), toString(row["name"])),
	)
	return DownloadStatus{
		Client:     c.Name(),
		ID:         downloadID,
		State:      normalizeQBitState(state, progress),
		Progress:   progress * 100.0,
		OutputPath: outputPath,
		Raw:        row,
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
		return "submitted"
	default:
		return "downloading"
	}
}

func (c *QBitTorrentClient) buildAddRequest(ctx context.Context, req AddRequest) (*http.Request, error) {
	uri := strings.TrimSpace(req.URI)
	if looksLikeTorrentURL(uri) {
		resp, err := c.httpClient.Get(uri)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("fetch torrent url status %d", resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		if req.Category != "" {
			_ = writer.WriteField("category", req.Category)
		}
		if len(req.Tags) > 0 {
			_ = writer.WriteField("tags", strings.Join(req.Tags, ","))
		}
		part, err := writer.CreateFormFile("torrents", "download.torrent")
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, err := part.Write(data); err != nil {
			_ = writer.Close()
			return nil, err
		}
		_ = writer.Close()
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/add", bytes.NewReader(body.Bytes()))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", writer.FormDataContentType())
		return httpReq, nil
	}

	form := url.Values{}
	form.Set("urls", uri)
	if req.Category != "" {
		form.Set("category", req.Category)
	}
	if len(req.Tags) > 0 {
		form.Set("tags", strings.Join(req.Tags, ","))
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/torrents/add", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return httpReq, nil
}

func looksLikeTorrentURL(v string) bool {
	lower := strings.ToLower(strings.TrimSpace(v))
	return (strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")) && strings.Contains(lower, ".torrent")
}

func hashFromMagnet(uri string) string {
	lower := strings.ToLower(strings.TrimSpace(uri))
	if !strings.HasPrefix(lower, "magnet:") {
		return ""
	}
	idx := strings.Index(lower, "xt=urn:btih:")
	if idx < 0 {
		return ""
	}
	hash := lower[idx+12:]
	if amp := strings.Index(hash, "&"); amp >= 0 {
		hash = hash[:amp]
	}
	return strings.TrimSpace(hash)
}

func pickNewest(rows []map[string]any) map[string]any {
	if len(rows) == 0 {
		return map[string]any{}
	}
	best := rows[0]
	bestAdded := toFloat(best["added_on"])
	for i := 1; i < len(rows); i++ {
		added := toFloat(rows[i]["added_on"])
		if added > bestAdded {
			best = rows[i]
			bestAdded = added
		}
	}
	return best
}

func joinPath(base string, child string) string {
	if strings.TrimSpace(base) == "" || strings.TrimSpace(child) == "" {
		return ""
	}
	return filepath.Clean(base + string(os.PathSeparator) + child)
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
