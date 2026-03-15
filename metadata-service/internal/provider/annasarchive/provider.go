package annasarchive

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"metadata-service/internal/model"
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
	linkRe             = regexp.MustCompile(`(?is)<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	tagRe              = regexp.MustCompile(`(?is)<[^>]+>`)
	md5PathRe          = regexp.MustCompile(`(?i)/md5/([a-f0-9]{32})`)
	metaTitleDoubleRe  = regexp.MustCompile(`(?is)<meta[^>]+property=["']og:title["'][^>]+content="([^"]+)"[^>]*>`)
	metaTitleSingleRe  = regexp.MustCompile(`(?is)<meta[^>]+property=["']og:title["'][^>]+content='([^']+)'[^>]*>`)
	htmlTitleRe        = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	isbnRe             = regexp.MustCompile(`(?i)\b[0-9x][0-9x\s-]{8,20}[0-9x]\b`)
	yearRe             = regexp.MustCompile(`\b(1[5-9]\d{2}|20\d{2}|2100)\b`)
	labeledFieldLineRe = regexp.MustCompile(`(?i)^(title|author|authors|publisher|published|publication year|year|format|extension|file type|isbn|isbn-10|isbn-13)\s*:\s*(.+)$`)
)

func cleanText(input string) string {
	stripped := tagRe.ReplaceAllString(input, " ")
	decoded := html.UnescapeString(stripped)
	return strings.Join(strings.Fields(decoded), " ")
}

func normalizeMD5(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "wrk_aa_") || strings.HasPrefix(trimmed, "edn_aa_") {
		trimmed = trimmed[7:]
	}
	if m := md5PathRe.FindStringSubmatch(trimmed); len(m) == 2 {
		return strings.ToLower(m[1])
	}
	cleaned := strings.ToLower(strings.Trim(trimmed, "/"))
	if len(cleaned) == 32 {
		for _, ch := range cleaned {
			if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
				return ""
			}
		}
		return cleaned
	}
	return ""
}

func workIDForMD5(md5 string) string {
	return "wrk_aa_" + md5
}

func editionIDForMD5(md5 string) string {
	return "edn_aa_" + md5
}

type searchHit struct {
	MD5   string
	Title string
}

func parseSearchHits(htmlBody string) []searchHit {
	matches := linkRe.FindAllStringSubmatch(htmlBody, 200)
	seen := map[string]struct{}{}
	results := make([]searchHit, 0, 20)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		md5 := normalizeMD5(match[1])
		if md5 == "" {
			continue
		}
		title := cleanText(match[2])
		if len(title) < 3 {
			continue
		}
		if _, ok := seen[md5]; ok {
			continue
		}
		seen[md5] = struct{}{}
		results = append(results, searchHit{MD5: md5, Title: title})
		if len(results) >= 10 {
			break
		}
	}
	return results
}

type detailMetadata struct {
	Title     string
	Authors   []string
	Year      int
	Publisher string
	Format    string
	ISBNs     []string
}

func splitAuthors(value string) []string {
	raw := strings.Split(strings.ReplaceAll(value, ";", ","), ",")
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, item := range raw {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	return out
}

func normalizeISBN(input string) string {
	var b strings.Builder
	for _, ch := range input {
		if ch >= '0' && ch <= '9' {
			b.WriteRune(ch)
			continue
		}
		if ch == 'x' || ch == 'X' {
			b.WriteRune('X')
		}
	}
	out := b.String()
	if len(out) == 10 || len(out) == 13 {
		return out
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func parseDetailMetadata(htmlBody string) detailMetadata {
	metadata := detailMetadata{}

	if m := metaTitleDoubleRe.FindStringSubmatch(htmlBody); len(m) == 2 {
		title := cleanText(m[1])
		title = strings.TrimSpace(strings.TrimSuffix(title, "- Anna's Archive"))
		title = strings.TrimSpace(strings.TrimSuffix(title, "- Anna’s Archive"))
		metadata.Title = title
	} else if m := metaTitleSingleRe.FindStringSubmatch(htmlBody); len(m) == 2 {
		title := cleanText(m[1])
		title = strings.TrimSpace(strings.TrimSuffix(title, "- Anna's Archive"))
		title = strings.TrimSpace(strings.TrimSuffix(title, "- Anna’s Archive"))
		metadata.Title = title
	} else if m := htmlTitleRe.FindStringSubmatch(htmlBody); len(m) == 2 {
		title := cleanText(m[1])
		title = strings.TrimSpace(strings.TrimSuffix(title, "- Anna's Archive"))
		title = strings.TrimSpace(strings.TrimSuffix(title, "- Anna’s Archive"))
		metadata.Title = title
	}

	decoded := html.UnescapeString(htmlBody)
	plain := tagRe.ReplaceAllString(decoded, "\n")
	lines := strings.Split(plain, "\n")
	for _, line := range lines {
		clean := strings.TrimSpace(strings.Join(strings.Fields(line), " "))
		if clean == "" {
			continue
		}
		m := labeledFieldLineRe.FindStringSubmatch(clean)
		if len(m) != 3 {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(m[1]))
		value := strings.TrimSpace(m[2])
		switch label {
		case "title":
			if metadata.Title == "" {
				metadata.Title = value
			}
		case "author", "authors":
			metadata.Authors = append(metadata.Authors, splitAuthors(value)...)
		case "publisher":
			if metadata.Publisher == "" {
				metadata.Publisher = value
			}
		case "publication year", "year", "published":
			if metadata.Year == 0 {
				if y := yearRe.FindString(value); y != "" {
					if parsed, err := strconv.Atoi(y); err == nil {
						metadata.Year = parsed
					}
				}
			}
		case "format", "extension", "file type":
			if metadata.Format == "" {
				metadata.Format = strings.ToLower(value)
			}
		case "isbn", "isbn-10", "isbn-13":
			for _, raw := range isbnRe.FindAllString(value, -1) {
				if normalized := normalizeISBN(raw); normalized != "" {
					metadata.ISBNs = append(metadata.ISBNs, normalized)
				}
			}
		}
	}

	if metadata.Year == 0 {
		if y := yearRe.FindString(plain); y != "" {
			if parsed, err := strconv.Atoi(y); err == nil {
				metadata.Year = parsed
			}
		}
	}
	for _, raw := range isbnRe.FindAllString(plain, -1) {
		if normalized := normalizeISBN(raw); normalized != "" {
			metadata.ISBNs = append(metadata.ISBNs, normalized)
		}
	}
	metadata.Authors = uniqueStrings(metadata.Authors)
	metadata.ISBNs = uniqueStrings(metadata.ISBNs)
	if metadata.Format == "" {
		metadata.Format = "ebook"
	}
	return metadata
}

func fetchPage(ctx context.Context, client *http.Client, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
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

func editionFromMetadata(md5 string, workID string, meta detailMetadata) model.Edition {
	edition := model.Edition{
		ID:              editionIDForMD5(md5),
		WorkID:          workID,
		Title:           meta.Title,
		Format:          meta.Format,
		Publisher:       meta.Publisher,
		PublicationYear: meta.Year,
	}
	for _, isbn := range meta.ISBNs {
		idType := "ISBN_13"
		if len(isbn) == 10 {
			idType = "ISBN_10"
		}
		edition.Identifiers = append(edition.Identifiers, model.Identifier{
			Type:  idType,
			Value: isbn,
		})
	}
	return edition
}

func (p *Provider) SearchWorks(ctx context.Context, query string) ([]model.Work, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []model.Work{}, nil
	}
	endpoint := fmt.Sprintf("%s/search?q=%s", p.baseURL, url.QueryEscape(trimmed))
	body, err := fetchPage(ctx, p.client, endpoint)
	if err != nil {
		return nil, err
	}

	hits := parseSearchHits(body)
	works := make([]model.Work, 0, len(hits))
	for _, hit := range hits {
		workID := workIDForMD5(hit.MD5)
		edition := model.Edition{
			ID:        editionIDForMD5(hit.MD5),
			WorkID:    workID,
			Title:     hit.Title,
			Format:    "ebook",
			Publisher: "Anna's Archive",
		}
		works = append(works, model.Work{
			ID:       workID,
			Title:    hit.Title,
			Editions: []model.Edition{edition},
		})
	}
	return works, nil
}

func (p *Provider) GetWork(ctx context.Context, providerID string) (*model.Work, error) {
	md5 := normalizeMD5(providerID)
	if md5 == "" {
		return nil, fmt.Errorf("annasarchive: invalid provider id %s", providerID)
	}
	endpoint := fmt.Sprintf("%s/md5/%s", p.baseURL, md5)
	body, err := fetchPage(ctx, p.client, endpoint)
	if err != nil {
		return nil, err
	}
	meta := parseDetailMetadata(body)
	if meta.Title == "" {
		meta.Title = md5
	}

	work := &model.Work{
		ID:    workIDForMD5(md5),
		Title: meta.Title,
	}
	for idx, author := range meta.Authors {
		work.Authors = append(work.Authors, model.Author{
			ID:   fmt.Sprintf("ath_aa_%s_%d", md5, idx+1),
			Name: author,
		})
	}
	work.Editions = []model.Edition{editionFromMetadata(md5, work.ID, meta)}
	return work, nil
}

func (p *Provider) GetEditions(ctx context.Context, providerWorkID string) ([]model.Edition, error) {
	md5 := normalizeMD5(providerWorkID)
	if md5 == "" {
		return nil, fmt.Errorf("annasarchive: invalid provider work id %s", providerWorkID)
	}
	endpoint := fmt.Sprintf("%s/md5/%s", p.baseURL, md5)
	body, err := fetchPage(ctx, p.client, endpoint)
	if err != nil {
		return nil, err
	}
	meta := parseDetailMetadata(body)
	if meta.Title == "" {
		meta.Title = md5
	}
	workID := workIDForMD5(md5)
	return []model.Edition{editionFromMetadata(md5, workID, meta)}, nil
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
			enriched, editionErr := p.GetEditions(ctx, work.ID)
			if editionErr == nil && len(enriched) > 0 {
				ed := enriched[0]
				ed.Identifiers = append(ed.Identifiers, model.Identifier{
					Type:  strings.ToUpper(strings.TrimSpace(idType)),
					Value: value,
				})
				return &ed, nil
			}
			fallback := edition
			fallback.Identifiers = append(fallback.Identifiers, model.Identifier{
				Type:  strings.ToUpper(strings.TrimSpace(idType)),
				Value: value,
			})
			return &fallback, nil
		}
	}
	return nil, fmt.Errorf("annasarchive: identifier not found: %s %s", idType, value)
}
