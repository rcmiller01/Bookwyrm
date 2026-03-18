package jobs

import (
	"context"
	"fmt"
	"strings"

	"app-backend/internal/domain"
	"app-backend/internal/integration/download"
	"app-backend/internal/integration/indexer"
)

type NoopHandler struct {
	jobType domain.JobType
}

func NewNoopHandler(jobType domain.JobType) *NoopHandler {
	return &NoopHandler{jobType: jobType}
}

func (h *NoopHandler) Type() domain.JobType { return h.jobType }

func (h *NoopHandler) Handle(_ context.Context, _ domain.Job) (map[string]any, error) {
	return map[string]any{"status": "noop"}, nil
}

type IndexerSearchHandler struct {
	client *indexer.Client
}

func NewIndexerSearchHandler(client *indexer.Client) *IndexerSearchHandler {
	return &IndexerSearchHandler{client: client}
}

func (h *IndexerSearchHandler) Type() domain.JobType { return domain.JobTypeSearchMissing }

func (h *IndexerSearchHandler) Handle(ctx context.Context, job domain.Job) (map[string]any, error) {
	if h.client == nil {
		return nil, fmt.Errorf("indexer client unavailable")
	}
	metadata, ok := job.Payload["metadata"].(map[string]any)
	if !ok {
		return nil, ErrInvalidPayload
	}
	req := indexer.SearchRequest{
		Metadata:              metadata,
		RequestedCapabilities: toStringSlice(job.Payload["requested_capabilities"]),
		Priority:              toString(job.Payload["priority"]),
		PolicyProfile:         toString(job.Payload["policy_profile"]),
		BackendGroups:         toStringSlice(job.Payload["backend_groups"]),
	}
	result, err := h.client.Search(ctx, req)
	if err != nil {
		return nil, err
	}
	candidates := extractCandidates(result)
	return map[string]any{
		"source":          result["source"],
		"found":           result["found"],
		"candidate_count": len(candidates),
		"top_candidate":   firstCandidate(candidates),
		"availability":    result,
	}, nil
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch vv := v.(type) {
	case []string:
		return vv
	case []any:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func extractCandidates(result map[string]any) []any {
	raw, ok := result["candidates"].([]any)
	if !ok {
		return nil
	}
	return raw
}

func firstCandidate(candidates []any) any {
	if len(candidates) == 0 {
		return nil
	}
	return candidates[0]
}

type DownloadEnqueueHandler struct {
	service *download.Service
}

func NewDownloadEnqueueHandler(service *download.Service) *DownloadEnqueueHandler {
	return &DownloadEnqueueHandler{service: service}
}

func (h *DownloadEnqueueHandler) Type() domain.JobType { return domain.JobTypeEnqueueDownload }

func (h *DownloadEnqueueHandler) Handle(ctx context.Context, job domain.Job) (map[string]any, error) {
	if h.service == nil {
		return nil, fmt.Errorf("download service unavailable")
	}
	uri := strings.TrimSpace(toString(job.Payload["uri"]))
	if uri == "" {
		return nil, ErrInvalidPayload
	}
	clientName := strings.TrimSpace(toString(job.Payload["client"]))
	downloadID, resolvedClient, err := h.service.AddDownload(ctx, clientName, download.AddRequest{
		URI:      uri,
		Category: strings.TrimSpace(toString(job.Payload["category"])),
		Tags:     toStringSlice(job.Payload["tags"]),
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"client":      resolvedClient,
		"download_id": downloadID,
		"state":       "queued",
	}, nil
}

type DownloadPollHandler struct {
	service *download.Service
}

func NewDownloadPollHandler(service *download.Service) *DownloadPollHandler {
	return &DownloadPollHandler{service: service}
}

func (h *DownloadPollHandler) Type() domain.JobType { return domain.JobTypePollDownload }

func (h *DownloadPollHandler) Handle(ctx context.Context, job domain.Job) (map[string]any, error) {
	if h.service == nil {
		return nil, fmt.Errorf("download service unavailable")
	}
	downloadID := strings.TrimSpace(toString(job.Payload["download_id"]))
	if downloadID == "" {
		return nil, ErrInvalidPayload
	}
	clientName := strings.TrimSpace(toString(job.Payload["client"]))
	status, resolvedClient, err := h.service.GetStatus(ctx, clientName, downloadID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"client":      resolvedClient,
		"download_id": status.ID,
		"state":       status.State,
		"progress":    status.Progress,
		"status":      status,
	}, nil
}
