package api

import (
	"metadata-service/internal/model"
	"metadata-service/internal/store"
	"time"
)

type SearchResponse struct {
	Works []model.Work `json:"works"`
}

type WorkResponse struct {
	Work model.Work `json:"work"`
}

type EditionResponse struct {
	Edition model.Edition `json:"edition"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// Provider management types

type ProviderInfo struct {
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	Priority     int    `json:"priority"`
	TimeoutSec   int    `json:"timeout_sec"`
	RateLimit    int    `json:"rate_limit"`
	Status       string `json:"status"`
	FailureCount int    `json:"failure_count"`
	AvgLatencyMs int64  `json:"avg_latency_ms"`
}

type ProvidersResponse struct {
	Providers []ProviderInfo `json:"providers"`
}

type UpsertProviderRequest struct {
	Enabled    *bool   `json:"enabled"`
	Priority   *int    `json:"priority"`
	TimeoutSec *int    `json:"timeout_sec"`
	RateLimit  *int    `json:"rate_limit"`
	APIKey     *string `json:"api_key"`
}

type ProviderTestResponse struct {
	Provider string       `json:"provider"`
	Success  bool         `json:"success"`
	Works    []model.Work `json:"works,omitempty"`
	Error    string       `json:"error,omitempty"`
}

type ProviderPolicyResponse struct {
	QuarantineDisableDispatch bool   `json:"quarantine_disable_dispatch"`
	Source                    string `json:"source"`
	Mode                      string `json:"mode"`
}

type EnrichmentJobsResponse struct {
	Jobs []model.EnrichmentJob `json:"jobs"`
}

type EnqueueEnrichmentJobRequest struct {
	JobType    string `json:"job_type"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Priority   int    `json:"priority,omitempty"`
}

type EnqueueEnrichmentJobResponse struct {
	JobID int64 `json:"job_id"`
}

type EnrichmentJobResponse struct {
	Job model.EnrichmentJob `json:"job"`
}

type EnrichmentStatsResponse struct {
	Enabled        bool             `json:"enabled"`
	WorkerCount    int              `json:"worker_count"`
	QueueDepth     map[string]int64 `json:"queue_depth"`
	NextRunnableAt *time.Time       `json:"next_runnable_at"`
}

// Reliability types

// ReliabilityInfo is the wire representation of a provider's reliability score.
type ReliabilityInfo struct {
	Name              string  `json:"name"`
	Score             float64 `json:"score"`
	Availability      float64 `json:"availability"`
	LatencyScore      float64 `json:"latency_score"`
	AgreementScore    float64 `json:"agreement_score"`
	IdentifierQuality float64 `json:"identifier_quality"`
	Status            string  `json:"status"`
}

// ReliabilityListResponse is returned by GET /v1/providers/reliability.
type ReliabilityListResponse struct {
	Providers []ReliabilityInfo `json:"providers"`
}

// ReliabilityDetailResponse is returned by GET /v1/providers/{name}/reliability.
type ReliabilityDetailResponse struct {
	Provider ReliabilityInfo `json:"provider"`
}

func mergeProviderInfo(cfgs []store.ProviderConfig, statuses []store.ProviderStatus) []ProviderInfo {
	statusMap := make(map[string]store.ProviderStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s
	}
	var out []ProviderInfo
	for _, c := range cfgs {
		info := ProviderInfo{
			Name:       c.Name,
			Enabled:    c.Enabled,
			Priority:   c.Priority,
			TimeoutSec: c.TimeoutSec,
			RateLimit:  c.RateLimit,
			Status:     "unknown",
		}
		if s, ok := statusMap[c.Name]; ok {
			info.Status = s.Status
			info.FailureCount = s.FailureCount
			info.AvgLatencyMs = s.AvgLatencyMs
		}
		out = append(out, info)
	}
	return out
}
