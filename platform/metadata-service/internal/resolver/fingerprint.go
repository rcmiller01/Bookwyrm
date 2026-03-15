package resolver

import (
	"fmt"
	"strings"
)

// GenerateFingerprint produces a deterministic key for a work.
// Format: normalizedtitle|normalizedauthor|year
func GenerateFingerprint(title string, authorName string, year int) string {
	t := strings.ReplaceAll(NormalizeQuery(title), " ", "")
	a := strings.ReplaceAll(NormalizeQuery(authorName), " ", "")
	return fmt.Sprintf("%s|%s|%d", t, a, year)
}
