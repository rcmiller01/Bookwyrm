package launcher

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func WaitForHealthy(ctx context.Context, client *http.Client, endpoints []string, pollInterval time.Duration) error {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	if pollInterval <= 0 {
		pollInterval = 1 * time.Second
	}
	pending := make(map[string]struct{})
	for _, endpoint := range endpoints {
		trimmed := strings.TrimSpace(endpoint)
		if trimmed != "" {
			pending[trimmed] = struct{}{}
		}
	}
	if len(pending) == 0 {
		return nil
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		for endpoint := range pending {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
			if err != nil {
				return err
			}
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			_ = resp.Body.Close()
			if resp.StatusCode < 300 {
				delete(pending, endpoint)
			}
		}
		if len(pending) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("health wait timeout: pending %v", keys(pending))
		case <-ticker.C:
		}
	}
}

func keys(in map[string]struct{}) []string {
	out := make([]string, 0, len(in))
	for k := range in {
		out = append(out, k)
	}
	return out
}
