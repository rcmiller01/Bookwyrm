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

type ProwlarrAdapter struct {
	name    string
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewProwlarrAdapter(name string, baseURL string, apiKey string, timeout time.Duration) *ProwlarrAdapter {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &ProwlarrAdapter{
		name:    name,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		client:  &http.Client{Timeout: timeout},
	}
}

func (a *ProwlarrAdapter) Name() string { return a.name }

func (a *ProwlarrAdapter) Capabilities() []string {
	return []string{"availability", "files"}
}

func (a *ProwlarrAdapter) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/api/v1/health", nil)
	if err != nil {
		return err
	}
	a.setHeaders(req)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("prowlarr health status %d", resp.StatusCode)
	}
	return nil
}

func (a *ProwlarrAdapter) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	query := buildQuery(req)
	endpoint := a.baseURL + "/api/v1/search?query=" + url.QueryEscape(query) + "&type=search&limit=50"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return SearchResult{}, err
	}
	a.setHeaders(httpReq)
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return SearchResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return SearchResult{}, fmt.Errorf("prowlarr search status %d", resp.StatusCode)
	}

	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return SearchResult{}, err
	}
	candidates := make([]Candidate, 0, len(raw))
	for i, item := range raw {
		candidate := mapProwlarrCandidate(req.Metadata, item, i)
		if candidate.CandidateID == "" {
			continue
		}
		candidates = append(candidates, candidate)
	}

	return SearchResult{
		WorkID:     req.Metadata.WorkID,
		Source:     a.name,
		Found:      len(candidates) > 0,
		Candidates: candidates,
		SearchedAt: time.Now().UTC(),
		Trace: []AdapterTrace{{
			Adapter: a.name,
			Status:  "ok",
		}},
	}, nil
}

func (a *ProwlarrAdapter) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	if a.apiKey != "" {
		req.Header.Set("X-Api-Key", a.apiKey)
	}
}

func buildQuery(req SearchRequest) string {
	m := req.Metadata
	if strings.TrimSpace(m.ISBN13) != "" {
		return strings.TrimSpace(m.ISBN13)
	}
	if strings.TrimSpace(m.ISBN10) != "" {
		return strings.TrimSpace(m.ISBN10)
	}
	title := strings.TrimSpace(m.Title)
	if title == "" {
		title = strings.TrimSpace(m.WorkID)
	}
	parts := []string{}
	if title != "" {
		parts = append(parts, quoteIfNeeded(title))
	}
	if len(m.Authors) > 0 && strings.TrimSpace(m.Authors[0]) != "" {
		parts = append(parts, strings.TrimSpace(m.Authors[0]))
	}
	if format := preferredFormatToken(req.PreferredFormats); format != "" {
		parts = append(parts, format)
	}
	query := strings.Join(parts, " ")
	if strings.TrimSpace(query) != "" {
		return query
	}
	return title
}

func preferredFormatToken(formats []string) string {
	preferredOrder := []string{"epub", "azw3", "mobi", "pdf", "m4b", "mp3"}
	set := map[string]struct{}{}
	for _, format := range formats {
		format = strings.ToLower(strings.TrimSpace(format))
		if format == "" {
			continue
		}
		set[format] = struct{}{}
	}
	for _, format := range preferredOrder {
		if _, ok := set[format]; ok {
			return format
		}
	}
	for _, format := range formats {
		format = strings.ToLower(strings.TrimSpace(format))
		if format != "" {
			return format
		}
	}
	return ""
}

func quoteIfNeeded(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, " \t") {
		return fmt.Sprintf("\"%s\"", value)
	}
	return fmt.Sprintf("\"%s\"", value)
}

func mapProwlarrCandidate(m MetadataSnapshot, item map[string]any, idx int) Candidate {
	title := toStringValue(item["title"])
	if title == "" {
		title = m.Title
	}
	candidateID := toStringValue(item["guid"])
	if candidateID == "" {
		candidateID = toStringValue(item["downloadUrl"])
	}
	if candidateID == "" {
		candidateID = fmt.Sprintf("prowlarr-%s-%d", m.WorkID, idx)
	}
	link := toStringValue(item["downloadUrl"])
	if link == "" {
		link = toStringValue(item["guid"])
	}
	provenance := "prowlarr"
	if idxName := toStringValue(item["indexer"]); idxName != "" {
		provenance = "prowlarr:" + idxName
	}
	protocol := inferProtocol(item, link)
	grabPayload := buildGrabPayload(protocol, item, link)
	return Candidate{
		CandidateID:     candidateID,
		Title:           title,
		Format:          inferFormat(title),
		Protocol:        protocol,
		MatchConfidence: scoreProwlarrCandidate(m, title),
		ProviderLink:    link,
		Provenance:      provenance,
		ReasonCodes:     scoreReasons(m, title),
		GrabPayload:     grabPayload,
		Attributes: map[string]any{
			"indexer": toStringValue(item["indexer"]),
		},
	}
}

func inferFormat(title string) string {
	lower := strings.ToLower(title)
	switch {
	case strings.Contains(lower, "audiobook"):
		return "audiobook"
	case strings.Contains(lower, "epub"):
		return "epub"
	case strings.Contains(lower, "pdf"):
		return "pdf"
	default:
		return ""
	}
}

func scoreProwlarrCandidate(m MetadataSnapshot, title string) float64 {
	score := 0.55
	lowerTitle := strings.ToLower(strings.TrimSpace(title))
	if m.ISBN10 != "" || m.ISBN13 != "" {
		score += 0.30
	}
	if strings.Contains(lowerTitle, strings.ToLower(strings.TrimSpace(m.Title))) {
		score += 0.10
	}
	if len(m.Authors) > 0 && strings.Contains(lowerTitle, strings.ToLower(strings.TrimSpace(m.Authors[0]))) {
		score += 0.05
	}
	if score > 0.99 {
		return 0.99
	}
	return score
}

func scoreReasons(m MetadataSnapshot, title string) []string {
	reasons := []string{"title_fuzzy"}
	if m.ISBN10 != "" || m.ISBN13 != "" {
		reasons = append([]string{"identifier_exact"}, reasons...)
	}
	if len(m.Authors) > 0 && strings.Contains(strings.ToLower(title), strings.ToLower(strings.TrimSpace(m.Authors[0]))) {
		reasons = append(reasons, "author_overlap")
	}
	return reasons
}

func toStringValue(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func inferProtocol(item map[string]any, fallbackLink string) string {
	if protocol := strings.ToLower(toStringValue(item["protocol"])); protocol != "" {
		if strings.Contains(protocol, "usenet") {
			return "usenet"
		}
		if strings.Contains(protocol, "torrent") {
			return "torrent"
		}
	}
	if strings.HasPrefix(strings.ToLower(toStringValue(item["magnetUrl"])), "magnet:") {
		return "torrent"
	}
	lower := strings.ToLower(strings.TrimSpace(fallbackLink))
	if strings.HasPrefix(lower, "magnet:") || strings.Contains(lower, ".torrent") {
		return "torrent"
	}
	return "usenet"
}

func buildGrabPayload(protocol string, item map[string]any, fallbackLink string) map[string]any {
	guid := toStringValue(item["guid"])
	downloadURL := toStringValue(item["downloadUrl"])
	magnet := toStringValue(item["magnetUrl"])
	torrentURL := toStringValue(item["torrentUrl"])

	payload := map[string]any{
		"protocol": protocol,
	}

	switch protocol {
	case "torrent":
		if magnet == "" && strings.HasPrefix(strings.ToLower(fallbackLink), "magnet:") {
			magnet = fallbackLink
		}
		if torrentURL == "" && strings.HasPrefix(strings.ToLower(downloadURL), "http") {
			torrentURL = downloadURL
		}
		if magnet != "" {
			payload["magnet"] = magnet
		}
		if torrentURL != "" {
			payload["torrent_url"] = torrentURL
		}
	default:
		nzbURL := downloadURL
		if nzbURL == "" && strings.HasPrefix(strings.ToLower(fallbackLink), "http") {
			nzbURL = fallbackLink
		}
		if nzbURL != "" {
			payload["nzb_url"] = nzbURL
			payload["downloadUrl"] = nzbURL
		}
		if guid != "" {
			payload["guid"] = guid
		}
	}
	return payload
}
