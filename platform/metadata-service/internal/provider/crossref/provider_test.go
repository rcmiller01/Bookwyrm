package crossref

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchWorks_MapsMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/works" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		resp := map[string]any{
			"message": map[string]any{
				"items": []any{
					map[string]any{
						"DOI":       "10.1000/xyz123",
						"title":     []string{"Test Driven Metadata"},
						"publisher": "Example Press",
						"issued":    map[string]any{"date-parts": [][]int{{2021, 1, 1}}},
						"author": []any{
							map[string]any{"given": "Ada", "family": "Lovelace"},
						},
						"subject": []string{"Computer Science", "Metadata"},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := New(2, "ops@example.com", srv.URL)
	works, err := p.SearchWorks(context.Background(), "test driven")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(works) != 1 {
		t.Fatalf("expected 1 work, got %d", len(works))
	}
	if works[0].Title != "Test Driven Metadata" {
		t.Fatalf("unexpected title: %q", works[0].Title)
	}
	if works[0].FirstPubYear != 2021 {
		t.Fatalf("unexpected year: %d", works[0].FirstPubYear)
	}
	if len(works[0].Editions) == 0 || len(works[0].Editions[0].Identifiers) == 0 {
		t.Fatalf("expected DOI identifier on mapped edition")
	}
}

func TestResolveIdentifier_DOI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"message": map[string]any{
				"DOI":       "10.1000/xyz123",
				"title":     []string{"One DOI Book"},
				"publisher": "Example Press",
				"issued":    map[string]any{"date-parts": [][]int{{2020}}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := New(2, "", srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	edition, err := p.ResolveIdentifier(ctx, "DOI", "10.1000/xyz123")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if edition == nil {
		t.Fatalf("expected edition")
	}
	if edition.Identifiers[0].Type != "DOI" {
		t.Fatalf("expected DOI identifier, got %s", edition.Identifiers[0].Type)
	}
}
