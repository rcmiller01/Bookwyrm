package pipeline

import (
	"context"

	"app-backend/internal/domain/contract"
)

type RenamerPipeline struct {
	domain contract.Domain
}

func NewRenamerPipeline(domain contract.Domain) *RenamerPipeline {
	return &RenamerPipeline{domain: domain}
}

func (p *RenamerPipeline) Plan(ctx context.Context, input contract.NamingInput) (contract.NamingPlan, error) {
	if p == nil || p.domain == nil {
		return contract.NamingPlan{Variables: map[string]string{}, Renames: map[string]string{}}, nil
	}
	return p.domain.NamingEngine().Plan(ctx, input)
}
