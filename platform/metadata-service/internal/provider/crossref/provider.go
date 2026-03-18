package crossref

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

const defaultBaseURL = "https://api.crossref.org"

type Provider struct {
	client  *http.Client
	mailto  string
	baseURL string
}

func New(timeoutSeconds int, mailto string, baseURL string) *Provider {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &Provider{
		client:  &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		mailto:  strings.TrimSpace(mailto),
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (p *Provider) Name() string { return "crossref" }

func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsSearch:       true,
		SupportsISBN:         false,
		SupportsDOI:          true,
		SupportsSeries:       false,
		SupportsSubjects:     true,
		SupportsAuthorSearch: false,
	}
}

type worksResponse struct {
	Message struct {
		Items []workItem `json:"items"`
	} `json:"message"`
}

type workResponse struct {
	Message workItem `json:"message"`
}

type workItem struct {
	DOI       string   `json:"DOI"`
	Title     []string `json:"title"`
	Publisher string   `json:"publisher"`
	Published struct {
		DateParts [][]int `json:"date-parts"`
	} `json:"published-print"`
	Issued struct {
		DateParts [][]int `json:"date-parts"`
	} `json:"issued"`
	Author []struct {
		Given  string `json:"given"`
		Family string `json:"family"`
		Name   string `json:"name"`
	} `json:"author"`
	Subject []string `json:"subject"`
}

func (p *Provider) SearchWorks(ctx context.Context, query string) ([]model.Work, error) {
	params := url.Values{}
	params.Set("rows", "10")
	params.Set("query.bibliographic", strings.TrimSpace(query))
	if p.mailto != "" {
		params.Set("mailto", p.mailto)
	}
	endpoint := p.baseURL + "/works?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", p.userAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crossref: unexpected status %d", resp.StatusCode)
	}

	var payload worksResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	works := make([]model.Work, 0, len(payload.Message.Items))
	for _, item := range payload.Message.Items {
		works = append(works, mapItem(item))
	}
	return works, nil
}

func (p *Provider) GetWork(ctx context.Context, providerID string) (*model.Work, error) {
	doi := strings.TrimSpace(providerID)
	if doi == "" {
		return nil, fmt.Errorf("crossref: missing doi")
	}
	params := url.Values{}
	if p.mailto != "" {
		params.Set("mailto", p.mailto)
	}
	endpoint := p.baseURL + "/works/" + url.PathEscape(doi)
	if encoded := params.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", p.userAgent())
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("crossref: work not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crossref: unexpected status %d", resp.StatusCode)
	}
	var payload workResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	w := mapItem(payload.Message)
	return &w, nil
}

func (p *Provider) GetEditions(_ context.Context, _ string) ([]model.Edition, error) {
	return nil, nil
}

func (p *Provider) ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error) {
	kind := strings.ToUpper(strings.TrimSpace(idType))
	if kind != "DOI" {
		return nil, fmt.Errorf("crossref: unsupported identifier type %s", idType)
	}
	work, err := p.GetWork(ctx, value)
	if err != nil {
		return nil, err
	}
	if len(work.Editions) == 0 {
		return nil, fmt.Errorf("crossref: no edition data for doi")
	}
	return &work.Editions[0], nil
}

func mapItem(item workItem) model.Work {
	title := ""
	if len(item.Title) > 0 {
		title = strings.TrimSpace(item.Title[0])
	}
	w := model.Work{
		ID:           "wrk_" + uuid.NewString(),
		Title:        title,
		FirstPubYear: publishedYear(item),
	}
	for _, author := range item.Author {
		name := strings.TrimSpace(strings.TrimSpace(author.Given + " " + author.Family))
		if name == "" {
			name = strings.TrimSpace(author.Name)
		}
		if name == "" {
			continue
		}
		w.Authors = append(w.Authors, model.Author{
			ID:   "ath_" + uuid.NewString(),
			Name: name,
		})
	}
	if len(item.Subject) > 0 {
		subjectSet := map[string]struct{}{}
		for _, subject := range item.Subject {
			trimmed := strings.TrimSpace(subject)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, ok := subjectSet[key]; ok {
				continue
			}
			subjectSet[key] = struct{}{}
			w.Subjects = append(w.Subjects, trimmed)
			if len(w.Subjects) >= 25 {
				break
			}
		}
	}
	e := model.Edition{
		ID:              "edn_" + uuid.NewString(),
		WorkID:          w.ID,
		Title:           title,
		Publisher:       strings.TrimSpace(item.Publisher),
		PublicationYear: publishedYear(item),
	}
	if doi := strings.TrimSpace(item.DOI); doi != "" {
		e.Identifiers = append(e.Identifiers, model.Identifier{
			Type:  "DOI",
			Value: doi,
		})
	}
	w.Editions = []model.Edition{e}
	return w
}

func publishedYear(item workItem) int {
	for _, part := range [][]int{
		firstPart(item.Issued.DateParts),
		firstPart(item.Published.DateParts),
	} {
		if len(part) > 0 && part[0] > 0 {
			return part[0]
		}
	}
	return 0
}

func firstPart(parts [][]int) []int {
	if len(parts) == 0 {
		return nil
	}
	return parts[0]
}

func (p *Provider) userAgent() string {
	if p.mailto == "" {
		return "Bookwyrm metadata-service/1.0"
	}
	return "Bookwyrm metadata-service/1.0 (mailto:" + p.mailto + ")"
}
