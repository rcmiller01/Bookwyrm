package resolver

import (
	"regexp"
	"strings"
)

type QueryType int

const (
	QueryTypeText QueryType = iota
	QueryTypeISBN13
	QueryTypeISBN10
	QueryTypeASIN
	QueryTypeDOI
)

type ClassifiedQuery struct {
	Original        string
	Normalized      string
	Type            QueryType
	IdentifierType  string
	IdentifierValue string
}

var (
	reISBN13 = regexp.MustCompile(`^\d{13}$`)
	reISBN10 = regexp.MustCompile(`^\d{9}[\dXx]$`)
	reASIN   = regexp.MustCompile(`^[A-Z0-9]{10}$`)
	reDOI    = regexp.MustCompile(`(?i)^10\.\d{4,9}/\S+$`)
)

func ClassifyQuery(q string) ClassifiedQuery {
	norm := NormalizeQuery(q)
	stripped := strings.ReplaceAll(strings.ReplaceAll(q, "-", ""), " ", "")

	switch {
	case reDOI.MatchString(strings.TrimSpace(q)):
		return ClassifiedQuery{
			Original:        q,
			Normalized:      norm,
			Type:            QueryTypeDOI,
			IdentifierType:  "DOI",
			IdentifierValue: strings.TrimSpace(q),
		}
	case reISBN13.MatchString(stripped):
		return ClassifiedQuery{Original: q, Normalized: norm, Type: QueryTypeISBN13, IdentifierType: "ISBN_13", IdentifierValue: stripped}
	case reISBN10.MatchString(stripped):
		return ClassifiedQuery{Original: q, Normalized: norm, Type: QueryTypeISBN10, IdentifierType: "ISBN_10", IdentifierValue: stripped}
	case reASIN.MatchString(stripped):
		return ClassifiedQuery{Original: q, Normalized: norm, Type: QueryTypeASIN, IdentifierType: "ASIN", IdentifierValue: stripped}
	default:
		return ClassifiedQuery{Original: q, Normalized: norm, Type: QueryTypeText}
	}
}
