package metadata

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

type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

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

func (c *Client) Search(ctx context.Context, query string) (map[string]any, error) {
	endpoint := c.baseURL + "/v1/search?q=" + url.QueryEscape(strings.TrimSpace(query))
	return c.get(ctx, endpoint)
}

func (c *Client) GetWork(ctx context.Context, id string) (map[string]any, error) {
	endpoint := c.baseURL + "/v1/work/" + url.PathEscape(strings.TrimSpace(id))
	return c.get(ctx, endpoint)
}

func (c *Client) GetGraph(ctx context.Context, id string) (map[string]any, error) {
	endpoint := c.baseURL + "/v1/work/" + url.PathEscape(strings.TrimSpace(id)) + "/graph"
	return c.get(ctx, endpoint)
}

func (c *Client) GetRecommendations(ctx context.Context, id string, limit int) (map[string]any, error) {
	endpoint := c.baseURL + "/v1/work/" + url.PathEscape(strings.TrimSpace(id)) + "/recommendations"
	if limit > 0 {
		endpoint += "?limit=" + fmt.Sprintf("%d", limit)
	}
	return c.get(ctx, endpoint)
}

func (c *Client) GetQualityReport(ctx context.Context, limit int) (map[string]any, error) {
	endpoint := c.baseURL + "/v1/quality/report"
	if limit > 0 {
		endpoint += "?limit=" + fmt.Sprintf("%d", limit)
	}
	return c.get(ctx, endpoint)
}

func (c *Client) RepairQuality(ctx context.Context, payload any) (map[string]any, error) {
	endpoint := c.baseURL + "/v1/quality/repair"
	return c.post(ctx, endpoint, payload)
}

func (c *Client) BuildSnapshotFromWork(work map[string]any) MetadataSnapshot {
	snapshot := MetadataSnapshot{}
	if id, ok := work["id"].(string); ok {
		snapshot.WorkID = id
	}
	if title, ok := work["title"].(string); ok {
		snapshot.Title = title
	}
	if year, ok := work["first_pub_year"].(float64); ok {
		snapshot.PublicationYear = int(year)
	}

	if authors, ok := work["authors"].([]any); ok {
		for _, author := range authors {
			if aMap, mapOK := author.(map[string]any); mapOK {
				if name, nameOK := aMap["name"].(string); nameOK && strings.TrimSpace(name) != "" {
					snapshot.Authors = append(snapshot.Authors, name)
				}
			}
		}
	}

	if editions, ok := work["editions"].([]any); ok && len(editions) > 0 {
		if first, mapOK := editions[0].(map[string]any); mapOK {
			if editionID, idOK := first["id"].(string); idOK {
				snapshot.EditionID = editionID
			}
			if ids, idsOK := first["identifiers"].([]any); idsOK {
				for _, raw := range ids {
					idMap, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					typeValue, _ := idMap["type"].(string)
					value, _ := idMap["value"].(string)
					switch strings.ToUpper(strings.TrimSpace(typeValue)) {
					case "ISBN_10", "ISBN10":
						snapshot.ISBN10 = value
					case "ISBN_13", "ISBN13":
						snapshot.ISBN13 = value
					}
				}
			}
		}
	}

	return snapshot
}

func (c *Client) get(ctx context.Context, endpoint string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *Client) post(ctx context.Context, endpoint string, payload any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

func (c *Client) do(req *http.Request) (map[string]any, error) {
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
			return nil, fmt.Errorf("metadata-service error (%d): %s", resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("metadata-service error (%d)", resp.StatusCode)
	}
	return parsed, nil
}
