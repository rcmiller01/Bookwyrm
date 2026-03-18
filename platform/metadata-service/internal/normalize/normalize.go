package normalize

import platformnormalize "bookwyrm/platform/normalize"

// NormalizeSubject canonicalizes a subject string for deterministic identity keys.
func NormalizeSubject(subject string) string {
	return platformnormalize.NormalizeSubject(subject)
}

// NormalizeSeriesName canonicalizes a series name for deterministic identity keys.
func NormalizeSeriesName(seriesName string) string {
	return platformnormalize.NormalizeSeriesName(seriesName)
}
