package quality

import "strings"

func VerifyIdentifier(idType string, value string) (bool, string) {
	t := strings.ToUpper(strings.TrimSpace(idType))
	v := strings.TrimSpace(value)
	if v == "" {
		return false, "empty identifier value"
	}

	switch t {
	case "ISBN_10", "ISBN10":
		if isValidISBN10(v) {
			return true, ""
		}
		return false, "invalid ISBN-10 checksum or format"
	case "ISBN_13", "ISBN13":
		if isValidISBN13(v) {
			return true, ""
		}
		return false, "invalid ISBN-13 checksum or format"
	default:
		return true, ""
	}
}

func cleanISBN(value string) string {
	replacer := strings.NewReplacer("-", "", " ", "")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(value)))
}

func isValidISBN10(value string) bool {
	clean := cleanISBN(value)
	if len(clean) != 10 {
		return false
	}
	sum := 0
	for i := 0; i < 10; i++ {
		ch := clean[i]
		var digit int
		if i == 9 && ch == 'X' {
			digit = 10
		} else if ch >= '0' && ch <= '9' {
			digit = int(ch - '0')
		} else {
			return false
		}
		sum += digit * (10 - i)
	}
	return sum%11 == 0
}

func isValidISBN13(value string) bool {
	clean := cleanISBN(value)
	if len(clean) != 13 {
		return false
	}
	sum := 0
	for i := 0; i < 13; i++ {
		ch := clean[i]
		if ch < '0' || ch > '9' {
			return false
		}
		digit := int(ch - '0')
		if i == 12 {
			check := (10 - (sum % 10)) % 10
			return digit == check
		}
		if i%2 == 0 {
			sum += digit
		} else {
			sum += digit * 3
		}
	}
	return true
}
