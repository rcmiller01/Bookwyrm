package resolver

import (
	"testing"

	"metadata-service/internal/model"
)

func TestMergeWorksWeighted_PrefersReliableSeriesAndUnionsSubjects(t *testing.T) {
	seriesA := "Dune Chronicles"
	seriesB := "Wrong Series"
	idxA := 1.0
	idxB := 9.0

	resultA := ProviderResult{
		Provider: "hardcover",
		Works: []model.Work{
			{
				ID:           "w-a",
				Title:        "Dune",
				Fingerprint:  "fp-dune",
				FirstPubYear: 1965,
				SeriesName:   &seriesA,
				SeriesIndex:  &idxA,
				Subjects:     []string{"Science Fiction", "Classics"},
				Editions: []model.Edition{
					{
						ID:     "ed-1",
						WorkID: "w-a",
						Identifiers: []model.Identifier{
							{Type: "ISBN_13", Value: "9780441172719"},
						},
					},
				},
			},
		},
	}
	resultB := ProviderResult{
		Provider: "low_source",
		Works: []model.Work{
			{
				ID:           "w-b",
				Title:        "Dune",
				Fingerprint:  "fp-dune",
				FirstPubYear: 1965,
				SeriesName:   &seriesB,
				SeriesIndex:  &idxB,
				Subjects:     []string{"science fiction", "Adventure"},
				Editions: []model.Edition{
					{
						ID:     "ed-2",
						WorkID: "w-b",
						Identifiers: []model.Identifier{
							{Type: "isbn_13", Value: "9780441172719"},
							{Type: "ISBN_13", Value: "9780441172719"},
						},
					},
				},
			},
		},
	}

	merged, err := NewMerger().MergeWorksWeighted(
		[]ProviderResult{resultA, resultB},
		map[string]float64{"hardcover": 0.95, "low_source": 0.40},
	)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if len(merged) != 1 {
		t.Fatalf("expected one merged work, got %d", len(merged))
	}
	w := merged[0]
	if w.SeriesName == nil || *w.SeriesName != seriesA {
		t.Fatalf("expected reliable series %q, got %+v", seriesA, w.SeriesName)
	}
	if len(w.Subjects) != 3 {
		t.Fatalf("expected 3 merged subjects, got %d (%v)", len(w.Subjects), w.Subjects)
	}
	if len(w.Editions) != 2 {
		t.Fatalf("expected both editions to be preserved, got %d", len(w.Editions))
	}
	if len(w.Editions[1].Identifiers) != 1 {
		t.Fatalf("expected duplicate identifiers to be deduped within edition, got %d", len(w.Editions[1].Identifiers))
	}
}

func TestMergeWorksWeighted_ChoosesConsensusYear(t *testing.T) {
	merged, err := NewMerger().MergeWorksWeighted([]ProviderResult{
		{
			Provider: "p1",
			Works: []model.Work{
				{ID: "a", Title: "Book", Fingerprint: "fp-book", FirstPubYear: 2001},
			},
		},
		{
			Provider: "p2",
			Works: []model.Work{
				{ID: "b", Title: "Book", Fingerprint: "fp-book", FirstPubYear: 2001},
			},
		},
		{
			Provider: "p3",
			Works: []model.Work{
				{ID: "c", Title: "Book", Fingerprint: "fp-book", FirstPubYear: 2002},
			},
		},
	}, map[string]float64{
		"p1": 0.60,
		"p2": 0.61,
		"p3": 0.99,
	})
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}
	if len(merged) != 1 {
		t.Fatalf("expected one merged work, got %d", len(merged))
	}
	if merged[0].FirstPubYear != 2001 {
		t.Fatalf("expected consensus year 2001, got %d", merged[0].FirstPubYear)
	}
}
