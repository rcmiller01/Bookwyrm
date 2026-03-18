package pipeline

import (
	"context"
	"testing"

	"app-backend/internal/domain/books"
	"app-backend/internal/domain/contract"
	"app-backend/internal/integration/metadata"
)

func TestImporterBuildSearchSpec(t *testing.T) {
	p := NewImporterPipeline(books.NewDomain())
	spec := p.BuildSearchSpec(metadata.MetadataSnapshot{
		WorkID:          "work-1",
		EditionID:       "ed-1",
		ISBN13:          "9780441172719",
		Title:           "Dune",
		Authors:         []string{"Frank Herbert"},
		PublicationYear: 1965,
	}, []string{"availability"}, "high", "default", []string{"prowlarr"})

	if spec.Metadata["work_id"] != "work-1" {
		t.Fatalf("expected work_id to be set, got %#v", spec.Metadata["work_id"])
	}
	if spec.Metadata["entity_type"] != "work" {
		t.Fatalf("expected entity_type=work, got %#v", spec.Metadata["entity_type"])
	}
	if spec.Metadata["entity_id"] != "work-1" {
		t.Fatalf("expected entity_id=work-1, got %#v", spec.Metadata["entity_id"])
	}
}

func TestRenamerPlanDelegatesToDomain(t *testing.T) {
	p := NewRenamerPipeline(books.NewDomain())
	plan, err := p.Plan(context.Background(), contract.NamingInput{
		Entity: contract.EntityRef{Type: "work", ID: "work-1"},
		Files:  []string{"Dune.epub"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Variables == nil || plan.Renames == nil {
		t.Fatalf("expected non-nil plan maps, got %+v", plan)
	}
}
