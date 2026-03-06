package resolver

import (
	"context"
	"testing"

	"metadata-service/internal/model"
	"metadata-service/internal/provider"
)

type routerProvider struct {
	name string
	caps provider.Capabilities
}

func (p routerProvider) Name() string { return p.name }
func (p routerProvider) SearchWorks(context.Context, string) ([]model.Work, error) {
	return nil, nil
}
func (p routerProvider) GetWork(context.Context, string) (*model.Work, error) { return nil, nil }
func (p routerProvider) GetEditions(context.Context, string) ([]model.Edition, error) {
	return nil, nil
}
func (p routerProvider) ResolveIdentifier(context.Context, string, string) (*model.Edition, error) {
	return nil, nil
}
func (p routerProvider) Capabilities() provider.Capabilities { return p.caps }

func TestApplyRoutingBias_DOI(t *testing.T) {
	providers := []provider.Provider{
		routerProvider{name: "openlibrary", caps: provider.Capabilities{SupportsSearch: true, SupportsISBN: true}},
		routerProvider{name: "crossref", caps: provider.Capabilities{SupportsSearch: true, SupportsDOI: true}},
	}
	out := ApplyRoutingBias(ClassifiedQuery{Type: QueryTypeDOI, IdentifierType: "DOI"}, providers)
	if out[0].Name() != "crossref" {
		t.Fatalf("expected crossref first for DOI query, got %s", out[0].Name())
	}
}

func TestApplyRoutingBias_ISBN(t *testing.T) {
	providers := []provider.Provider{
		routerProvider{name: "hardcover", caps: provider.Capabilities{SupportsSearch: true, SupportsISBN: true}},
		routerProvider{name: "googlebooks", caps: provider.Capabilities{SupportsSearch: true, SupportsISBN: true}},
		routerProvider{name: "openlibrary", caps: provider.Capabilities{SupportsSearch: true, SupportsISBN: true}},
	}
	out := ApplyRoutingBias(ClassifiedQuery{Type: QueryTypeISBN13, IdentifierType: "ISBN_13"}, providers)
	if out[0].Name() != "googlebooks" && out[0].Name() != "openlibrary" {
		t.Fatalf("expected openlibrary/googlebooks first for ISBN query, got %s", out[0].Name())
	}
}

func TestApplyRoutingBias_PlainTitleAuthor(t *testing.T) {
	providers := []provider.Provider{
		routerProvider{name: "crossref", caps: provider.Capabilities{SupportsSearch: true, SupportsDOI: true}},
		routerProvider{name: "openlibrary", caps: provider.Capabilities{SupportsSearch: true, SupportsISBN: true}},
		routerProvider{name: "hardcover", caps: provider.Capabilities{SupportsSearch: true, SupportsAuthorSearch: true}},
		routerProvider{name: "googlebooks", caps: provider.Capabilities{SupportsSearch: true, SupportsAuthorSearch: true}},
	}
	out := ApplyRoutingBias(ClassifiedQuery{
		Type:       QueryTypeText,
		Normalized: "frank herbert dune",
	}, providers)
	if len(out) < 3 {
		t.Fatalf("expected at least 3 providers, got %d", len(out))
	}
	top := map[string]struct{}{
		out[0].Name(): {},
		out[1].Name(): {},
		out[2].Name(): {},
	}
	for _, expected := range []string{"openlibrary", "hardcover", "googlebooks"} {
		if _, ok := top[expected]; !ok {
			t.Fatalf("expected %s in top 3 for plain query; order=%s,%s,%s", expected, out[0].Name(), out[1].Name(), out[2].Name())
		}
	}
}
