package hardcover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"metadata-service/internal/model"
	"metadata-service/internal/provider"
)

const graphqlEndpoint = "https://api.hardcover.app/v1/graphql"

type Provider struct {
	client *http.Client
	token  string
}

func New(timeoutSeconds int, token string) *Provider {
	return &Provider{
		client: &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		token:  token,
	}
}

func (p *Provider) Name() string { return "hardcover" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsSearch:       true,
		SupportsISBN:         true,
		SupportsDOI:          false,
		SupportsSeries:       true,
		SupportsSubjects:     true,
		SupportsAuthorSearch: true,
	}
}

// --- GraphQL request/response helpers ---

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (p *Provider) doQuery(ctx context.Context, query string, variables map[string]any, out interface{}) error {
	body, _ := json.Marshal(gqlRequest{Query: query, Variables: variables})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlEndpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", p.token)
	req.Header.Set("User-Agent", "Bookwyrm metadata-service/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("hardcover: unauthorized — check API token")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("hardcover: rate limited")
	}

	var gqlResp gqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return err
	}
	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("hardcover: graphql error: %s", gqlResp.Errors[0].Message)
	}
	return json.Unmarshal(gqlResp.Data, out)
}

// --- Search ---

const searchQuery = `
query SearchBooks($query: String!) {
  search(query: $query, query_type: "Book", per_page: 10) {
    results
  }
}`

type searchResult struct {
	Search struct {
		Results json.RawMessage `json:"results"`
	} `json:"search"`
}

type hardcoverBook struct {
	Title      string   `json:"title"`
	Author     string   `json:"author"`
	ISBN13     string   `json:"isbn_13"`
	ISBN10     string   `json:"isbn_10"`
	Year       int      `json:"year_published"`
	Series     string   `json:"series_name"`
	SeriesPart string   `json:"series_position"`
	Tags       []string `json:"tags"`
	Genres     []string `json:"genres"`
}

func (p *Provider) SearchWorks(ctx context.Context, query string) ([]model.Work, error) {
	var data searchResult
	err := p.doQuery(ctx, searchQuery, map[string]any{"query": query}, &data)
	if err != nil {
		return nil, err
	}

	var books []hardcoverBook
	if err := json.Unmarshal(data.Search.Results, &books); err != nil {
		// Results may be nested under "hits" depending on API version
		var wrapped struct {
			Hits []struct {
				Document hardcoverBook `json:"document"`
			} `json:"hits"`
		}
		if err2 := json.Unmarshal(data.Search.Results, &wrapped); err2 != nil {
			return nil, err
		}
		for _, h := range wrapped.Hits {
			books = append(books, h.Document)
		}
	}

	var works []model.Work
	for _, b := range books {
		w := model.Work{
			ID:           "wrk_" + uuid.NewString(),
			Title:        b.Title,
			FirstPubYear: b.Year,
		}
		if series := strings.TrimSpace(b.Series); series != "" {
			w.SeriesName = &series
			if idx, err := strconv.ParseFloat(strings.TrimSpace(b.SeriesPart), 64); err == nil {
				w.SeriesIndex = &idx
			}
		}
		subjectSet := map[string]struct{}{}
		for _, tag := range append(b.Tags, b.Genres...) {
			subject := strings.TrimSpace(tag)
			if subject == "" {
				continue
			}
			key := strings.ToLower(subject)
			if _, ok := subjectSet[key]; ok {
				continue
			}
			subjectSet[key] = struct{}{}
			w.Subjects = append(w.Subjects, subject)
			if len(w.Subjects) >= 25 {
				break
			}
		}
		if b.Author != "" {
			w.Authors = []model.Author{{ID: "ath_" + uuid.NewString(), Name: b.Author}}
		}
		e := model.Edition{ID: "edn_" + uuid.NewString(), WorkID: w.ID}
		if b.ISBN13 != "" {
			e.Identifiers = append(e.Identifiers, model.Identifier{Type: "ISBN_13", Value: b.ISBN13})
		}
		if b.ISBN10 != "" {
			e.Identifiers = append(e.Identifiers, model.Identifier{Type: "ISBN_10", Value: b.ISBN10})
		}
		if len(e.Identifiers) > 0 {
			w.Editions = []model.Edition{e}
		}
		works = append(works, w)
	}
	return works, nil
}

func (p *Provider) GetWork(ctx context.Context, providerID string) (*model.Work, error) {
	return nil, fmt.Errorf("hardcover: GetWork not implemented")
}

func (p *Provider) GetEditions(ctx context.Context, providerWorkID string) ([]model.Edition, error) {
	return nil, fmt.Errorf("hardcover: GetEditions not implemented")
}

func (p *Provider) ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error) {
	works, err := p.SearchWorks(ctx, value)
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
	return nil, fmt.Errorf("hardcover: identifier not found: %s %s", idType, value)
}
