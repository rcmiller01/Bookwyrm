package books

import (
	"context"

	"app-backend/internal/domain/contract"
)

type namingEngine struct{}

func (n namingEngine) Plan(_ context.Context, _ contract.NamingInput) (contract.NamingPlan, error) {
	return contract.NamingPlan{Variables: map[string]string{}, Renames: map[string]string{}}, nil
}
