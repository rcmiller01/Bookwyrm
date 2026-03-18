package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"indexer-service/internal/indexer"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type toolRequest struct {
	Tool  string `json:"tool"`
	Input any    `json:"input,omitempty"`
}

type toolResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error,omitempty"`
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Search(ctx context.Context, query indexer.QuerySpec, headers map[string]string) ([]indexer.Candidate, error) {
	respBody, status, err := c.callTool(ctx, "indexer.search", query, headers)
	if err == nil && status < 300 {
		var candidates []indexer.Candidate
		if err := decodeToolResult(respBody, &candidates); err != nil {
			return nil, err
		}
		if err := validateCandidates(candidates); err != nil {
			return nil, err
		}
		return candidates, nil
	}
	// Backward-compatible fallback for early HTTP endpoint-based MCP adapters.
	payload, marshalErr := json.Marshal(query)
	if marshalErr != nil {
		return nil, marshalErr
	}
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/indexer/search", bytes.NewReader(payload))
	if reqErr != nil {
		return nil, reqErr
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	legacyResp, legacyErr := c.httpClient.Do(req)
	if legacyErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, legacyErr
	}
	defer legacyResp.Body.Close()
	if legacyResp.StatusCode >= 300 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("mcp search status %d", legacyResp.StatusCode)
	}
	var candidates []indexer.Candidate
	if err := json.NewDecoder(legacyResp.Body).Decode(&candidates); err != nil {
		return nil, err
	}
	if err := validateCandidates(candidates); err != nil {
		return nil, err
	}
	return candidates, nil
}

func (c *Client) Health(ctx context.Context, headers map[string]string) error {
	respBody, status, err := c.callTool(ctx, "indexer.health", map[string]any{}, headers)
	if err == nil && status < 300 {
		var result map[string]any
		if decodeErr := decodeToolResult(respBody, &result); decodeErr == nil {
			if okValue, ok := result["ok"].(bool); ok && okValue {
				return nil
			}
			return errors.New("mcp health tool reported not ok")
		}
		return nil
	}
	// Backward-compatible fallback for early HTTP endpoint-based MCP adapters.
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/indexer/health", nil)
	if reqErr != nil {
		if err != nil {
			return err
		}
		return reqErr
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	legacyResp, legacyErr := c.httpClient.Do(req)
	if legacyErr != nil {
		if err != nil {
			return err
		}
		return legacyErr
	}
	defer legacyResp.Body.Close()
	if legacyResp.StatusCode >= 300 {
		if err != nil {
			return err
		}
		return fmt.Errorf("mcp health status %d", legacyResp.StatusCode)
	}
	return nil
}

func (c *Client) callTool(ctx context.Context, toolName string, input any, headers map[string]string) ([]byte, int, error) {
	payload, err := json.Marshal(toolRequest{
		Tool:  toolName,
		Input: input,
	})
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/mcp/tool", bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp.StatusCode, readErr
	}
	if resp.StatusCode >= 300 {
		return body, resp.StatusCode, fmt.Errorf("mcp tool call status %d", resp.StatusCode)
	}
	return body, resp.StatusCode, nil
}

func decodeToolResult(raw []byte, out any) error {
	var tr toolResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return err
	}
	if !tr.OK {
		if tr.Error == "" {
			return errors.New("mcp tool call failed")
		}
		return errors.New(tr.Error)
	}
	if len(tr.Result) == 0 {
		return errors.New("mcp tool response missing result")
	}
	if err := json.Unmarshal(tr.Result, out); err != nil {
		return err
	}
	return nil
}

func validateCandidates(candidates []indexer.Candidate) error {
	for i := range candidates {
		c := candidates[i]
		if strings.TrimSpace(c.Title) == "" {
			return fmt.Errorf("candidate[%d] missing title", i)
		}
		if strings.TrimSpace(c.Protocol) == "" {
			return fmt.Errorf("candidate[%d] missing protocol", i)
		}
		if c.GrabPayload == nil {
			return fmt.Errorf("candidate[%d] missing grab_payload", i)
		}
	}
	return nil
}
