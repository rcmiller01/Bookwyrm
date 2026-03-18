package annasarchive

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"metadata-service/internal/model"
	"metadata-service/internal/provider"
)

const (
	defaultBaseURL = "https://annas-archive.org"
	providerPrefix = "annasarchive:md5:"
)

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

var (
	md5LinkRe          = regexp.MustCompile(`(?is)<a[^>]*href="([^"]*/md5/([a-f0-9]{32})[^"]*)"[^>]*>(.*?)</a>`)
	h1Re               = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	titleRe            = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	metaTitleRe        = regexp.MustCompile(`(?is)<meta[^>]+property="og:title"[^>]+content="([^"]+)"`)
	authorLinkRe       = regexp.MustCompile(`(?is)<a[^>]*href="[^"]*(?:/author/|index=author)[^"]*"[^>]*>(.*?)</a>`)
	isbn13Re           = regexp.MustCompile(`\b97[89][0-9]{10}\b`)
	isbn10Re           = regexp.MustCompile(`\b[0-9]{9}[0-9Xx]\b`)
	yearLabelRe        = regexp.MustCompile(`(?is)(?:year|published|publication year)[^0-9]{0,20}([12][0-9]{3})`)
	formatLabelRe      = regexp.MustCompile(`(?is)(?:format|extension|file type)[^a-z0-9]{0,20}(epub|pdf|mobi|azw3|djvu|fb2|mp3|m4b|m4a)`)
	publisherLabelRe   = regexp.MustCompile(`(?is)(?:publisher)[^a-z0-9]{0,20}([a-z0-9][^<\n]{1,120})`)
	tagRe              = regexp.MustCompile(`(?is)<[^>]+>`)
	spaceRe            = regexp.MustCompile(`\s+`)
	annasTitleSuffixRe = regexp.MustCompile(`(?i)\s+-\s+anna'?s archive.*$`)
)

var publisherStopMarkers = []string{
	" Publication year ",
	" Published ",
	" Year ",
	" File type ",
	" Format ",
	" Extension ",
	" ISBN ",
	" Language ",
}

type searchResult struct {
	MD5       string
	Title     string
	DetailURL string
}

type detailRecord struct {
	MD5       string
	Title     string
	Authors   []string
	Publisher string
	Year      int
	Format    string
	ISBNs     []model.Identifier
	DetailURL string
}

func cleanText(input string) string {
	stripped := tagRe.ReplaceAllString(input, " ")
	decoded := html.UnescapeString(stripped)
	return strings.TrimSpace(spaceRe.ReplaceAllString(decoded, " "))
}

func normalizeTitle(title string) string {
	title = cleanText(title)
	title = annasTitleSuffixRe.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}

func buildWorkID(md5 string) string {
	return "wrk_aa_" + strings.ToLower(strings.TrimSpace(md5))
}

func buildEditionID(md5 string) string {
	return "edn_aa_" + strings.ToLower(strings.TrimSpace(md5))
}

func buildProviderRef(md5 string) string {
	return providerPrefix + strings.ToLower(strings.TrimSpace(md5))
}

func normalizeProviderID(providerID string) string {
	trimmed := strings.ToLower(strings.TrimSpace(providerID))
	trimmed = strings.TrimPrefix(trimmed, providerPrefix)
	trimmed = strings.TrimPrefix(trimmed, "wrk_aa_")
	trimmed = strings.TrimPrefix(trimmed, "edn_aa_")
	if idx := strings.Index(trimmed, "/md5/"); idx >= 0 {
		trimmed = trimmed[idx+5:]
	}
	trimmed = strings.Trim(trimmed, "/")
	if len(trimmed) > 32 {
		trimmed = trimmed[:32]
	}
	if len(trimmed) != 32 {
		return ""
	}
	for _, ch := range trimmed {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return ""
		}
	}
	return trimmed
}

func resolveURL(baseURL string, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	parsed, err := url.Parse(href)
	if err == nil && parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

func parseSearchResults(htmlBody string, baseURL string) []searchResult {
	matches := md5LinkRe.FindAllStringSubmatch(htmlBody, 200)
	seen := map[string]struct{}{}
	results := make([]searchResult, 0, 20)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		md5 := normalizeProviderID(match[2])
		title := normalizeTitle(match[3])
		if md5 == "" || title == "" {
			continue
		}
		if _, ok := seen[md5]; ok {
			continue
		}
		seen[md5] = struct{}{}
		results = append(results, searchResult{
			MD5:       md5,
			Title:     title,
			DetailURL: resolveURL(baseURL, match[1]),
		})
		if len(results) >= 10 {
			break
		}
	}
	return results
}

func firstMatch(body string, patterns ...*regexp.Regexp) string {
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(body)
		if len(match) < 2 {
			continue
		}
		value := normalizeTitle(match[1])
		if value != "" {
			return value
		}
	}
	return ""
}

func parseAuthors(htmlBody string) []string {
	matches := authorLinkRe.FindAllStringSubmatch(htmlBody, 20)
	seen := map[string]struct{}{}
	authors := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := cleanText(match[1])
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		authors = append(authors, name)
	}
	return authors
}

func parseIdentifiers(htmlBody string) []model.Identifier {
	cleaned := cleanText(htmlBody)
	seen := map[string]struct{}{}
	ids := make([]model.Identifier, 0)
	for _, match := range isbn13Re.FindAllString(cleaned, -1) {
		key := "ISBN_13|" + match
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ids = append(ids, model.Identifier{Type: "ISBN_13", Value: match})
	}
	for _, match := range isbn10Re.FindAllString(cleaned, -1) {
		key := "ISBN_10|" + strings.ToUpper(match)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ids = append(ids, model.Identifier{Type: "ISBN_10", Value: strings.ToUpper(match)})
	}
	return ids
}

func parseYear(htmlBody string) int {
	match := yearLabelRe.FindStringSubmatch(cleanText(htmlBody))
	if len(match) < 2 {
		return 0
	}
	year, _ := strconv.Atoi(match[1])
	return year
}

func parseFormat(htmlBody string) string {
	match := formatLabelRe.FindStringSubmatch(strings.ToLower(cleanText(htmlBody)))
	if len(match) < 2 {
		return "ebook"
	}
	return strings.ToLower(strings.TrimSpace(match[1]))
}

func parsePublisher(htmlBody string) string {
	match := publisherLabelRe.FindStringSubmatch(cleanText(htmlBody))
	if len(match) < 2 {
		return "Anna's Archive"
	}
	publisher := strings.TrimSpace(match[1])
	for _, marker := range publisherStopMarkers {
		if idx := strings.Index(publisher, marker); idx >= 0 {
			publisher = strings.TrimSpace(publisher[:idx])
			break
		}
	}
	if publisher == "" {
		return "Anna's Archive"
	}
	return publisher
}

func parseDetailRecord(htmlBody string, baseURL string, providerID string) detailRecord {
	md5 := normalizeProviderID(providerID)
	return detailRecord{
		MD5:       md5,
		Title:     firstMatch(htmlBody, h1Re, metaTitleRe, titleRe),
		Authors:   parseAuthors(htmlBody),
		Publisher: parsePublisher(htmlBody),
		Year:      parseYear(htmlBody),
		Format:    parseFormat(htmlBody),
		ISBNs:     parseIdentifiers(htmlBody),
		DetailURL: resolveURL(baseURL, "/md5/"+md5),
	}
}

func (p *Provider) fetchPage(ctx context.Context, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("annasarchive: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (p *Provider) SearchWorks(ctx context.Context, query string) ([]model.Work, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []model.Work{}, nil
	}
	endpoint := fmt.Sprintf("%s/search?q=%s", p.baseURL, url.QueryEscape(trimmed))
	htmlBody, err := p.fetchPage(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	results := parseSearchResults(htmlBody, p.baseURL)
	works := make([]model.Work, 0, len(results))
	for _, result := range results {
		edition := model.Edition{
			ID:        buildEditionID(result.MD5),
			WorkID:    buildWorkID(result.MD5),
			Title:     result.Title,
			Format:    "ebook",
			Publisher: "Anna's Archive",
			Identifiers: []model.Identifier{
				{Type: "ANNASARCHIVE_MD5", Value: result.MD5},
			},
		}
		works = append(works, model.Work{
			ID:                 buildWorkID(result.MD5),
			Title:              result.Title,
			RelatedProviderIDs: []string{buildProviderRef(result.MD5)},
			Editions:           []model.Edition{edition},
		})
	}
	return works, nil
}

func (p *Provider) GetWork(ctx context.Context, providerID string) (*model.Work, error) {
	md5 := normalizeProviderID(providerID)
	if md5 == "" {
		return nil, fmt.Errorf("annasarchive: invalid provider id %q", providerID)
	}
	htmlBody, err := p.fetchPage(ctx, fmt.Sprintf("%s/md5/%s", p.baseURL, md5))
	if err != nil {
		return nil, err
	}
	detail := parseDetailRecord(htmlBody, p.baseURL, md5)
	if detail.Title == "" {
		return nil, fmt.Errorf("annasarchive: detail page missing title for %s", md5)
	}

	work := &model.Work{
		ID:                 buildWorkID(md5),
		Title:              detail.Title,
		FirstPubYear:       detail.Year,
		RelatedProviderIDs: []string{buildProviderRef(md5)},
		Editions:           []model.Edition{buildEditionFromDetail(detail)},
	}
	for _, author := range detail.Authors {
		work.Authors = append(work.Authors, model.Author{ID: "ath_aa_" + strings.ToLower(strings.ReplaceAll(author, " ", "-")), Name: author})
	}
	return work, nil
}

func buildEditionFromDetail(detail detailRecord) model.Edition {
	identifiers := append([]model.Identifier{{Type: "ANNASARCHIVE_MD5", Value: detail.MD5}}, detail.ISBNs...)
	sort.SliceStable(identifiers, func(i, j int) bool {
		if identifiers[i].Type == identifiers[j].Type {
			return identifiers[i].Value < identifiers[j].Value
		}
		return identifiers[i].Type < identifiers[j].Type
	})
	return model.Edition{
		ID:              buildEditionID(detail.MD5),
		WorkID:          buildWorkID(detail.MD5),
		Title:           detail.Title,
		Format:          detail.Format,
		Publisher:       detail.Publisher,
		PublicationYear: detail.Year,
		Identifiers:     identifiers,
	}
}

func (p *Provider) GetEditions(ctx context.Context, providerWorkID string) ([]model.Edition, error) {
	md5 := normalizeProviderID(providerWorkID)
	if md5 == "" {
		return nil, fmt.Errorf("annasarchive: invalid provider id %q", providerWorkID)
	}
	htmlBody, err := p.fetchPage(ctx, fmt.Sprintf("%s/md5/%s", p.baseURL, md5))
	if err != nil {
		return nil, err
	}
	detail := parseDetailRecord(htmlBody, p.baseURL, md5)
	if detail.Title == "" {
		return nil, fmt.Errorf("annasarchive: detail page missing title for %s", md5)
	}
	return []model.Edition{buildEditionFromDetail(detail)}, nil
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
			ed.Identifiers = append(ed.Identifiers, model.Identifier{Type: strings.ToUpper(strings.TrimSpace(idType)), Value: strings.TrimSpace(value)})
			return &ed, nil
		}
	}
	return nil, fmt.Errorf("annasarchive: identifier not found: %s %s", idType, value)
}
