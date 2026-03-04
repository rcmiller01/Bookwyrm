package provider

import (
	"context"
	"metadata-service/internal/model"
)

type Provider interface {
	Name() string
	SearchWorks(ctx context.Context, query string) ([]model.Work, error)
	GetWork(ctx context.Context, providerID string) (*model.Work, error)
	GetEditions(ctx context.Context, providerWorkID string) ([]model.Edition, error)
	ResolveIdentifier(ctx context.Context, idType string, value string) (*model.Edition, error)
}
