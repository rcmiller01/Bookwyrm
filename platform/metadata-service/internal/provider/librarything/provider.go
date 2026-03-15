package librarything

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

const defaultBaseURL = "https://www.librarything.com"

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

func (p *Provider) Name() string { return "librarything" }

var (
	resultLinkRe = regexp.MustCompile(`(?is)<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	removeTagRe  = regexp.MustCompile(`(?is)<[^>]+>`)
)

func normalizeText(input string) string {
	stripped := removeTagRe.ReplaceAllString(input, " ")
	return strings.Join(strings.Fields(html.UnescapeString(stripped)), " ")
}

func parseLibraryThingTitles(htmlBody string) []string {
	matches := resultLinkRe.FindAllStringSubmatch(htmlBody, 250)
	seen := map[string]struct{}{}
	out := make([]string, 0, 20)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		href := strings.ToLower(strings.TrimSpace(match[1]))
		if !strings.Contains(href, "/work/") && !strings.Contains(href, "/book/") {
			continue
		}
		title := normalizeText(match[2])
		if len(title) < 3 {
			continue
		}
		if _, ok := seen[title]; ok {
			continue
		}
		seen[title] = struct{}{}
		out = append(out, title)
		if len(out) >= 10 {
			break
		}
	}
	return out
}

func (p *Provider) SearchWorks(ctx context.Context, query string) ([]model.Work, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []model.Work{}, nil
	}
	endpoint := fmt.Sprintf("%s/search.php?search=%s", p.baseURL, url.QueryEscape(trimmed))
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
		return nil, fmt.Errorf("librarything: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	titles := parseLibraryThingTitles(string(body))
	works := make([]model.Work, 0, len(titles))
	for _, title := range titles {
		workID := "wrk_" + uuid.NewString()
		works = append(works, model.Work{
			ID:    workID,
			Title: title,
			Editions: []model.Edition{{
				ID:        "edn_" + uuid.NewString(),
				WorkID:    workID,
				Title:     title,
				Publisher: "LibraryThing",
			}},
		})
	}
	return works, nil
}

func (p *Provider) GetWork(_ context.Context, providerID string) (*model.Work, error) {
	return nil, fmt.Errorf("librarything: GetWork not implemented for provider id %s", providerID)
}

func (p *Provider) GetEditions(_ context.Context, providerWorkID string) ([]model.Edition, error) {
	return nil, fmt.Errorf("librarything: GetEditions not implemented for provider id %s", providerWorkID)
}

func (p *Provider) ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("librarything: empty identifier")
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
	return nil, fmt.Errorf("librarything: identifier not found: %s %s", idType, value)
}
