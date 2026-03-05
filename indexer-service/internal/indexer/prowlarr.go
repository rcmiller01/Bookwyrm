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
	query := buildQuery(req.Metadata)
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

func buildQuery(m MetadataSnapshot) string {
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
	if len(m.Authors) > 0 && strings.TrimSpace(m.Authors[0]) != "" {
		return title + " " + strings.TrimSpace(m.Authors[0])
	}
	return title
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
	return Candidate{
		CandidateID:     candidateID,
		Title:           title,
		Format:          inferFormat(title),
		MatchConfidence: scoreProwlarrCandidate(m, title),
		ProviderLink:    link,
		Provenance:      provenance,
		ReasonCodes:     scoreReasons(m, title),
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
