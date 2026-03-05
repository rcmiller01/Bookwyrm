package download

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrClientNotConfigured = errors.New("download client not configured")
	ErrDownloadNotFound    = errors.New("download not found")
)

type AddRequest struct {
	URI      string
	Category string
	Tags     []string
}

type DownloadStatus struct {
	Client   string         `json:"client"`
	ID       string         `json:"id"`
	State    string         `json:"state"`
	Progress float64        `json:"progress"`
	Raw      map[string]any `json:"raw,omitempty"`
}

type Client interface {
	Name() string
	AddDownload(ctx context.Context, req AddRequest) (string, error)
	GetStatus(ctx context.Context, downloadID string) (DownloadStatus, error)
	Remove(ctx context.Context, downloadID string, deleteFiles bool) error
}

type Service struct {
	clients map[string]Client
}

func NewService(clients ...Client) *Service {
	s := &Service{clients: map[string]Client{}}
	for _, c := range clients {
		s.Register(c)
	}
	return s
}

func (s *Service) Register(client Client) {
	if client == nil {
		return
	}
	s.clients[strings.ToLower(strings.TrimSpace(client.Name()))] = client
}

func (s *Service) AddDownload(ctx context.Context, clientName string, req AddRequest) (string, string, error) {
	client, resolvedName, err := s.resolveClient(clientName)
	if err != nil {
		return "", "", err
	}
	downloadID, err := client.AddDownload(ctx, req)
	if err != nil {
		return "", resolvedName, err
	}
	return downloadID, resolvedName, nil
}

func (s *Service) GetStatus(ctx context.Context, clientName string, downloadID string) (DownloadStatus, string, error) {
	client, resolvedName, err := s.resolveClient(clientName)
	if err != nil {
		return DownloadStatus{}, "", err
	}
	status, err := client.GetStatus(ctx, downloadID)
	if err != nil {
		return DownloadStatus{}, resolvedName, err
	}
	status.Client = resolvedName
	return status, resolvedName, nil
}

func (s *Service) Remove(ctx context.Context, clientName string, downloadID string, deleteFiles bool) (string, error) {
	client, resolvedName, err := s.resolveClient(clientName)
	if err != nil {
		return "", err
	}
	if err := client.Remove(ctx, downloadID, deleteFiles); err != nil {
		return resolvedName, err
	}
	return resolvedName, nil
}

func (s *Service) resolveClient(clientName string) (Client, string, error) {
	if len(s.clients) == 0 {
		return nil, "", ErrClientNotConfigured
	}
	key := strings.ToLower(strings.TrimSpace(clientName))
	if key != "" {
		client, ok := s.clients[key]
		if !ok {
			return nil, "", fmt.Errorf("%w: %s", ErrClientNotConfigured, clientName)
		}
		return client, key, nil
	}
	if len(s.clients) == 1 {
		for name, client := range s.clients {
			return client, name, nil
		}
	}
	return nil, "", fmt.Errorf("%w: client required when multiple configured", ErrClientNotConfigured)
}
