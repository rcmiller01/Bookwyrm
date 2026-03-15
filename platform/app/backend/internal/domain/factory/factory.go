package factory

import (
	"fmt"
	"strings"

	"app-backend/internal/domain/books"
	"app-backend/internal/domain/contract"
)

func Resolve(name string) (contract.Domain, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "books":
		return books.NewDomain(), nil
	default:
		return nil, fmt.Errorf("unsupported domain: %s", name)
	}
}
