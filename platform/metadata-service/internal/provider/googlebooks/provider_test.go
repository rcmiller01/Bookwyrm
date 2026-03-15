package googlebooks

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSearchWorks_MapsFieldsAndNormalizesSubjects(t *testing.T) {
	p := New(2, "test-key")
	p.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/books/v1/volumes" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		query := req.URL.Query().Get("q")
		if !strings.Contains(query, "isbn:9780441172719") {
			t.Fatalf("unexpected query: %s", query)
		}
		body := `{
			"items": [{
				"id": "vol-1",
				"volumeInfo": {
					"title": "Dune",
					"authors": ["Frank Herbert"],
					"publisher": "Ace",
					"publishedDate": "1965-08-01",
					"categories": ["Science Fiction", "science fiction", "Classics"],
					"industryIdentifiers": [
						{"type":"ISBN_13","identifier":"9780441172719"},
						{"type":"ISBN_10","identifier":"0441172717"}
					]
				}
			}]
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	works, err := p.SearchWorks(context.Background(), "isbn:9780441172719")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(works) != 1 {
		t.Fatalf("expected 1 work, got %d", len(works))
	}
	w := works[0]
	if w.Title != "Dune" {
		t.Fatalf("unexpected title: %q", w.Title)
	}
	if w.FirstPubYear != 1965 {
		t.Fatalf("expected year 1965, got %d", w.FirstPubYear)
	}
	if len(w.Authors) != 1 || w.Authors[0].Name != "Frank Herbert" {
		t.Fatalf("unexpected authors: %+v", w.Authors)
	}
	if len(w.Subjects) != 2 {
		t.Fatalf("expected deduped subjects, got %d (%v)", len(w.Subjects), w.Subjects)
	}
	if len(w.Editions) != 1 || len(w.Editions[0].Identifiers) != 2 {
		t.Fatalf("expected mapped isbn identifiers, got %+v", w.Editions)
	}
}

func TestSearchWorks_ContextTimeout(t *testing.T) {
	p := New(2, "")
	p.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := p.SearchWorks(ctx, "dune")
	if err == nil {
		t.Fatalf("expected timeout error")
	}
}
