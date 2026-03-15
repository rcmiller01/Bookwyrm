package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type MetadataClient struct {
	baseURL string
	client  *http.Client
}

func NewMetadataClient(baseURL string, timeout time.Duration) *MetadataClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &MetadataClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *MetadataClient) GetWork(ctx context.Context, workID string) (MetadataSnapshot, error) {
	if c == nil {
		return MetadataSnapshot{}, fmt.Errorf("metadata client not configured")
	}
	workID = strings.TrimSpace(workID)
	if workID == "" {
		return MetadataSnapshot{}, fmt.Errorf("missing work id")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/work/"+url.PathEscape(workID), nil)
	if err != nil {
		return MetadataSnapshot{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return MetadataSnapshot{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return MetadataSnapshot{}, fmt.Errorf("metadata work status %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return MetadataSnapshot{}, err
	}
	work := payload
	if wrapped, ok := payload["work"].(map[string]any); ok {
		work = wrapped
	}

	snapshot := MetadataSnapshot{
		WorkID: workID,
		Title:  strings.TrimSpace(stringValue(work["title"])),
	}

	if authors, ok := work["authors"].([]any); ok {
		for _, raw := range authors {
			authorMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name := strings.TrimSpace(stringValue(authorMap["name"]))
			if name != "" {
				snapshot.Authors = append(snapshot.Authors, name)
			}
		}
	}

	if editions, ok := work["editions"].([]any); ok {
		for _, rawEdition := range editions {
			editionMap, ok := rawEdition.(map[string]any)
			if !ok {
				continue
			}
			if lang := strings.TrimSpace(stringValue(editionMap["language"])); lang != "" && snapshot.Language == "" {
				snapshot.Language = lang
			}
			if ids, ok := editionMap["identifiers"].([]any); ok {
				for _, rawID := range ids {
					idMap, ok := rawID.(map[string]any)
					if !ok {
						continue
					}
					typ := strings.ToUpper(strings.TrimSpace(stringValue(idMap["type"])))
					val := strings.TrimSpace(stringValue(idMap["value"]))
					switch typ {
					case "ISBN13", "ISBN_13":
						if snapshot.ISBN13 == "" {
							snapshot.ISBN13 = val
						}
					case "ISBN10", "ISBN_10":
						if snapshot.ISBN10 == "" {
							snapshot.ISBN10 = val
						}
					}
				}
			}
			if snapshot.Language != "" && (snapshot.ISBN10 != "" || snapshot.ISBN13 != "") {
				break
			}
		}
	}

	return snapshot, nil
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}
