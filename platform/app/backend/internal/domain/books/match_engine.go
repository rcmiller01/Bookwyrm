package books

import (
	"context"

	"app-backend/internal/domain/contract"
)

type matchEngine struct{}

func (m matchEngine) Match(_ context.Context, _ contract.MatchInput) contract.MatchResult {
	return contract.MatchResult{Candidates: []contract.EntityRef{}, Confidence: 0}
}
