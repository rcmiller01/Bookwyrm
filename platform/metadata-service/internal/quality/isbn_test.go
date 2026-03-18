package quality

import "testing"

func TestVerifyIdentifierISBN10(t *testing.T) {
	valid, reason := VerifyIdentifier("ISBN_10", "0-306-40615-2")
	if !valid || reason != "" {
		t.Fatalf("expected valid ISBN-10, got valid=%v reason=%q", valid, reason)
	}

	valid, reason = VerifyIdentifier("ISBN_10", "0-306-40615-3")
	if valid || reason == "" {
		t.Fatalf("expected invalid ISBN-10, got valid=%v reason=%q", valid, reason)
	}
}

func TestVerifyIdentifierISBN13(t *testing.T) {
	valid, reason := VerifyIdentifier("ISBN_13", "978-0-306-40615-7")
	if !valid || reason != "" {
		t.Fatalf("expected valid ISBN-13, got valid=%v reason=%q", valid, reason)
	}

	valid, reason = VerifyIdentifier("ISBN_13", "978-0-306-40615-8")
	if valid || reason == "" {
		t.Fatalf("expected invalid ISBN-13, got valid=%v reason=%q", valid, reason)
	}
}

func TestVerifyIdentifierUnknownType(t *testing.T) {
	valid, reason := VerifyIdentifier("ASIN", "B00TEST123")
	if !valid || reason != "" {
		t.Fatalf("expected unknown identifier type to pass through, got valid=%v reason=%q", valid, reason)
	}
}
