package factory

import "testing"

func TestResolveBooksDefault(t *testing.T) {
	d, err := Resolve("")
	if err != nil {
		t.Fatalf("expected default domain resolve, got error: %v", err)
	}
	if d == nil || d.Name() != "books" {
		t.Fatalf("expected books domain, got %#v", d)
	}
}

func TestResolveUnsupported(t *testing.T) {
	d, err := Resolve("tv")
	if err == nil {
		t.Fatalf("expected unsupported-domain error, got nil with domain %#v", d)
	}
}
