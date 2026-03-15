package openlibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"metadata-service/internal/model"
	"metadata-service/internal/provider"
)

const baseURL = "https://openlibrary.org"

type Provider struct {
	client *http.Client
}

func New(timeoutSeconds int) *Provider {
	return &Provider{
		client: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}
}

func (p *Provider) Name() string {
	return "openlibrary"
}

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsSearch:       true,
		SupportsISBN:         true,
		SupportsDOI:          false,
		SupportsSeries:       false,
		SupportsSubjects:     false,
		SupportsAuthorSearch: true,
	}
}

// --- Search ---

type searchResponse struct {
	Docs []searchDoc `json:"docs"`
}

type searchDoc struct {
	Key          string   `json:"key"`
	Title        string   `json:"title"`
	AuthorNames  []string `json:"author_name"`
	FirstPublish int      `json:"first_publish_year"`
	ISBN         []string `json:"isbn"`
}

func (p *Provider) SearchWorks(ctx context.Context, query string) ([]model.Work, error) {
	u := fmt.Sprintf("%s/search.json?q=%s&limit=10", baseURL, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	var works []model.Work
	for _, doc := range sr.Docs {
		w := model.Work{
			ID:           "wrk_" + uuid.NewString(),
			Title:        doc.Title,
			FirstPubYear: doc.FirstPublish,
		}
		for _, name := range doc.AuthorNames {
			w.Authors = append(w.Authors, model.Author{
				ID:   "ath_" + uuid.NewString(),
				Name: name,
			})
		}
		if len(doc.ISBN) > 0 {
			e := model.Edition{
				ID:     "edn_" + uuid.NewString(),
				WorkID: w.ID,
			}
			for _, isbn := range doc.ISBN {
				idType := "ISBN_13"
				if len(strings.ReplaceAll(isbn, "-", "")) == 10 {
					idType = "ISBN_10"
				}
				e.Identifiers = append(e.Identifiers, model.Identifier{Type: idType, Value: isbn})
			}
			w.Editions = []model.Edition{e}
		}
		works = append(works, w)
	}
	return works, nil
}

// --- GetWork ---

func (p *Provider) GetWork(ctx context.Context, providerID string) (*model.Work, error) {
	u := fmt.Sprintf("%s/works/%s.json", baseURL, providerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	return &model.Work{
		ID:    "wrk_" + uuid.NewString(),
		Title: raw.Title,
	}, nil
}

// --- GetEditions ---

func (p *Provider) GetEditions(ctx context.Context, providerWorkID string) ([]model.Edition, error) {
	u := fmt.Sprintf("%s/works/%s/editions.json?limit=10", baseURL, providerWorkID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw struct {
		Entries []struct {
			Title      string   `json:"title"`
			Publishers []string `json:"publishers"`
			ISBN13     []string `json:"isbn_13"`
			ISBN10     []string `json:"isbn_10"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	var editions []model.Edition
	for _, entry := range raw.Entries {
		e := model.Edition{
			ID:    "edn_" + uuid.NewString(),
			Title: entry.Title,
		}
		if len(entry.Publishers) > 0 {
			e.Publisher = entry.Publishers[0]
		}
		for _, isbn := range entry.ISBN13 {
			e.Identifiers = append(e.Identifiers, model.Identifier{Type: "ISBN_13", Value: isbn})
		}
		for _, isbn := range entry.ISBN10 {
			e.Identifiers = append(e.Identifiers, model.Identifier{Type: "ISBN_10", Value: isbn})
		}
		editions = append(editions, e)
	}
	return editions, nil
}

// --- ResolveIdentifier ---

func (p *Provider) ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error) {
	u := fmt.Sprintf("%s/isbn/%s.json", baseURL, value)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("identifier not found: %s %s", idType, value)
	}

	var raw struct {
		Title      string   `json:"title"`
		Publishers []string `json:"publishers"`
		ISBN13     []string `json:"isbn_13"`
		ISBN10     []string `json:"isbn_10"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	e := model.Edition{
		ID:    "edn_" + uuid.NewString(),
		Title: raw.Title,
	}
	if len(raw.Publishers) > 0 {
		e.Publisher = raw.Publishers[0]
	}
	for _, isbn := range raw.ISBN13 {
		e.Identifiers = append(e.Identifiers, model.Identifier{Type: "ISBN_13", Value: isbn})
	}
	for _, isbn := range raw.ISBN10 {
		e.Identifiers = append(e.Identifiers, model.Identifier{Type: "ISBN_10", Value: isbn})
	}
	return &e, nil
}
