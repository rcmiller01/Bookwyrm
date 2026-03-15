package hardcover

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

func TestSearchWorks_MapsSeriesSubjectsAndIdentifiers(t *testing.T) {
	p := New(2, "token-123")
	p.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.String() != graphqlEndpoint {
			t.Fatalf("unexpected endpoint: %s", req.URL.String())
		}
		if req.Header.Get("Authorization") != "token-123" {
			t.Fatalf("expected authorization token header")
		}
		body := `{
			"data": {
				"search": {
					"results": [{
						"title": "Dune",
						"author": "Frank Herbert",
						"isbn_13": "9780441172719",
						"isbn_10": "0441172717",
						"year_published": 1965,
						"series_name": "Dune Chronicles",
						"series_position": "1",
						"tags": ["Science Fiction", "space opera"],
						"genres": ["science fiction", "Classic"]
					}]
				}
			}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	works, err := p.SearchWorks(context.Background(), "dune")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(works) != 1 {
		t.Fatalf("expected 1 work, got %d", len(works))
	}
	w := works[0]
	if w.SeriesName == nil || *w.SeriesName != "Dune Chronicles" {
		t.Fatalf("expected mapped series, got %+v", w.SeriesName)
	}
	if w.SeriesIndex == nil || *w.SeriesIndex != 1 {
		t.Fatalf("expected series index 1, got %+v", w.SeriesIndex)
	}
	if len(w.Subjects) != 3 {
		t.Fatalf("expected deduped tag/genre subjects, got %d (%v)", len(w.Subjects), w.Subjects)
	}
	if len(w.Editions) != 1 || len(w.Editions[0].Identifiers) != 2 {
		t.Fatalf("expected mapped edition identifiers, got %+v", w.Editions)
	}
}

func TestSearchWorks_ContextTimeout(t *testing.T) {
	p := New(2, "token")
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
