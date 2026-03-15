package normalize

import (
	"strings"
	"unicode"
)

func normalizeGraphToken(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	var builder strings.Builder
	for _, runeValue := range lower {
		if unicode.IsLetter(runeValue) || unicode.IsDigit(runeValue) || unicode.IsSpace(runeValue) {
			builder.WriteRune(runeValue)
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

// NormalizeSubject canonicalizes a subject string for deterministic identity keys.
func NormalizeSubject(subject string) string {
	return normalizeGraphToken(subject)
}

// NormalizeSeriesName canonicalizes a series name for deterministic identity keys.
func NormalizeSeriesName(seriesName string) string {
	return normalizeGraphToken(seriesName)
}
