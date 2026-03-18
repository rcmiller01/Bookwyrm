package download

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ConnectionTester is an optional interface that download clients can implement
// to provide a lightweight connectivity check. The TestConnection method should
// hit the cheapest possible API endpoint (e.g. version/status) and return nil
// on success or an error describing the failure.
type ConnectionTester interface {
	TestConnection(ctx context.Context) error
}

// TestConnection checks whether the named client is reachable. If the client
// does not implement ConnectionTester, it returns an error explaining that.
func (s *Service) TestConnection(ctx context.Context, clientName string) error {
	client, _, err := s.resolveClient(clientName)
	if err != nil {
		return err
	}
	tester, ok := client.(ConnectionTester)
	if !ok {
		return fmt.Errorf("client %q does not support connection testing", clientName)
	}
	return tester.TestConnection(ctx)
}

// --- SABnzbd ---

func (c *SABnzbdClient) TestConnection(ctx context.Context) error {
	params := c.baseParams()
	params.Set("mode", "version")
	endpoint := c.baseURL + "/api?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("sabnzbd: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sabnzbd unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("sabnzbd returned status %d", resp.StatusCode)
	}
	return nil
}

// --- qBittorrent ---

func (c *QBitTorrentClient) TestConnection(ctx context.Context) error {
	if err := c.login(ctx); err != nil {
		return fmt.Errorf("qbittorrent login failed: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/app/version", nil)
	if err != nil {
		return fmt.Errorf("qbittorrent: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qbittorrent unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qbittorrent returned status %d", resp.StatusCode)
	}
	return nil
}

// --- NZBGet ---

func (c *NZBGetClient) TestConnection(ctx context.Context) error {
	var result map[string]any
	if err := c.rpc(ctx, "status", []any{}, &result); err != nil {
		return fmt.Errorf("nzbget unreachable: %w", err)
	}
	// Verify we got a valid response with at least one field
	if _, err := json.Marshal(result); err != nil {
		return fmt.Errorf("nzbget: unexpected response: %w", err)
	}
	return nil
}
