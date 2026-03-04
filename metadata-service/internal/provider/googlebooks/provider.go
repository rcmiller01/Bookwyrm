package googlebooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"metadata-service/internal/model"
)

const baseURL = "https://www.googleapis.com/books/v1"

type Provider struct {
	client *http.Client
	apiKey string
}

func New(timeoutSeconds int, apiKey string) *Provider {
	return &Provider{
		client: &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		apiKey: apiKey,
	}
}

func (p *Provider) Name() string { return "googlebooks" }

// --- internal response types ---

type volumesResponse struct {
	Items []volumeItem `json:"items"`
}

type volumeItem struct {
	ID         string     `json:"id"`
	VolumeInfo volumeInfo `json:"volumeInfo"`
}

type volumeInfo struct {
	Title               string   `json:"title"`
	Authors             []string `json:"authors"`
	Publisher           string   `json:"publisher"`
	PublishedDate       string   `json:"publishedDate"`
	IndustryIdentifiers []struct {
		Type       string `json:"type"`
		Identifier string `json:"identifier"`
	} `json:"industryIdentifiers"`
}

// --- helpers ---

func (p *Provider) buildURL(path string, params map[string]string) string {
	u := fmt.Sprintf("%s%s?", baseURL, path)
	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	if p.apiKey != "" {
		vals.Set("key", p.apiKey)
	}
	return u + vals.Encode()
}

func pubYear(date string) int {
	if len(date) >= 4 {
		var y int
		fmt.Sscanf(date[:4], "%d", &y)
		return y
	}
	return 0
}

func mapItem(item volumeItem) model.Work {
	vi := item.VolumeInfo
	w := model.Work{
		ID:           "wrk_" + uuid.NewString(),
		Title:        vi.Title,
		FirstPubYear: pubYear(vi.PublishedDate),
	}
	for _, name := range vi.Authors {
		w.Authors = append(w.Authors, model.Author{ID: "ath_" + uuid.NewString(), Name: name})
	}
	if len(vi.IndustryIdentifiers) > 0 {
		e := model.Edition{
			ID:        "edn_" + uuid.NewString(),
			WorkID:    w.ID,
			Publisher: vi.Publisher,
		}
		for _, id := range vi.IndustryIdentifiers {
			t := id.Type
			if t == "ISBN_13" || t == "ISBN_10" {
				e.Identifiers = append(e.Identifiers, model.Identifier{Type: t, Value: id.Identifier})
			}
		}
		w.Editions = []model.Edition{e}
	}
	return w
}

// --- Provider interface ---

func (p *Provider) SearchWorks(ctx context.Context, query string) ([]model.Work, error) {
	u := p.buildURL("/volumes", map[string]string{"q": query, "maxResults": "10"})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("googlebooks: unexpected status %d", resp.StatusCode)
	}

	var vr volumesResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return nil, err
	}
	var works []model.Work
	for _, item := range vr.Items {
		works = append(works, mapItem(item))
	}
	return works, nil
}

func (p *Provider) GetWork(ctx context.Context, providerID string) (*model.Work, error) {
	u := p.buildURL("/volumes/"+providerID, nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var item volumeItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}
	w := mapItem(item)
	return &w, nil
}

func (p *Provider) GetEditions(ctx context.Context, providerWorkID string) ([]model.Edition, error) {
	w, err := p.GetWork(ctx, providerWorkID)
	if err != nil || len(w.Editions) == 0 {
		return nil, err
	}
	return w.Editions, nil
}

func (p *Provider) ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error) {
	query := fmt.Sprintf("isbn:%s", value)
	works, err := p.SearchWorks(ctx, query)
	if err != nil {
		return nil, err
	}
	for _, w := range works {
		for _, e := range w.Editions {
			for _, id := range e.Identifiers {
				if id.Value == value {
					return &e, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("googlebooks: identifier not found: %s %s", idType, value)
}
