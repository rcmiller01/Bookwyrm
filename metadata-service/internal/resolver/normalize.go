package resolver

import (
	"strings"
	"unicode"
)

// NormalizeQuery lowercases, strips punctuation, and collapses whitespace.
func NormalizeQuery(q string) string {
	q = strings.ToLower(q)
	var b strings.Builder
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
