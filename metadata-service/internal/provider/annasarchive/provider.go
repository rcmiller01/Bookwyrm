package annasarchive

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"metadata-service/internal/model"

	"github.com/google/uuid"
)

const defaultBaseURL = "https://annas-archive.org"

type Provider struct {
	client  *http.Client
	baseURL string
}

func New(timeoutSeconds int, baseURL string) *Provider {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 15
	}
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = defaultBaseURL
	}
	return &Provider{
		client:  &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		baseURL: strings.TrimRight(base, "/"),
	}
}

func (p *Provider) Name() string { return "annasarchive" }

var (
	linkRe = regexp.MustCompile(`(?is)<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	tagRe  = regexp.MustCompile(`(?is)<[^>]+>`)
)

func cleanText(input string) string {
	stripped := tagRe.ReplaceAllString(input, " ")
	decoded := html.UnescapeString(stripped)
	return strings.Join(strings.Fields(decoded), " ")
}

func parseSearchTitles(htmlBody string) []string {
	matches := linkRe.FindAllStringSubmatch(htmlBody, 200)
	seen := map[string]struct{}{}
	results := make([]string, 0, 20)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		href := strings.ToLower(strings.TrimSpace(match[1]))
		if !strings.Contains(href, "/md5/") && !strings.Contains(href, "/book/") {
			continue
		}
		title := cleanText(match[2])
		if len(title) < 3 {
			continue
		}
		if _, ok := seen[title]; ok {
			continue
		}
		seen[title] = struct{}{}
		results = append(results, title)
		if len(results) >= 10 {
			break
		}
	}
	return results
}

func (p *Provider) SearchWorks(ctx context.Context, query string) ([]model.Work, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []model.Work{}, nil
	}
	endpoint := fmt.Sprintf("%s/search?q=%s", p.baseURL, url.QueryEscape(trimmed))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("annasarchive: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	titles := parseSearchTitles(string(body))
	works := make([]model.Work, 0, len(titles))
	for _, title := range titles {
		workID := "wrk_" + uuid.NewString()
		edition := model.Edition{
			ID:        "edn_" + uuid.NewString(),
			WorkID:    workID,
			Title:     title,
			Format:    "ebook",
			Publisher: "Anna's Archive",
		}
		works = append(works, model.Work{
			ID:       workID,
			Title:    title,
			Editions: []model.Edition{edition},
		})
	}
	return works, nil
}

func (p *Provider) GetWork(_ context.Context, providerID string) (*model.Work, error) {
	return nil, fmt.Errorf("annasarchive: GetWork not implemented for provider id %s", providerID)
}

func (p *Provider) GetEditions(_ context.Context, providerWorkID string) ([]model.Edition, error) {
	return nil, fmt.Errorf("annasarchive: GetEditions not implemented for provider id %s", providerWorkID)
}

func (p *Provider) ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("annasarchive: empty identifier")
	}
	works, err := p.SearchWorks(ctx, value)
	if err != nil {
		return nil, err
	}
	for _, work := range works {
		for _, edition := range work.Editions {
			ed := edition
			ed.Identifiers = []model.Identifier{{Type: strings.ToUpper(strings.TrimSpace(idType)), Value: value}}
			return &ed, nil
		}
	}
	return nil, fmt.Errorf("annasarchive: identifier not found: %s %s", idType, value)
}
