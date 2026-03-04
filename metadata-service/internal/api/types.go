package api

import (
	"metadata-service/internal/model"
	"metadata-service/internal/store"
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
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Priority      int    `json:"priority"`
	TimeoutSec    int    `json:"timeout_sec"`
	RateLimit     int    `json:"rate_limit"`
	Status        string `json:"status"`
	FailureCount  int    `json:"failure_count"`
	AvgLatencyMs  int64  `json:"avg_latency_ms"`
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
