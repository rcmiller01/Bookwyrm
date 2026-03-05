package worldcat
package worldcat

import "testing"

func TestParseWorldCatTitles(t *testing.T) {
	html := `
	<div>
	  <a href="/title/dune-messiah/oclc/123">Dune Messiah</a>
	  <a href="/title/foundation/oclc/987">Foundation</a>
	  <a href="/help">Help</a>
	</div>`
	got := parseWorldCatTitles(html)
	if len(got) != 2 {
		t.Fatalf("expected 2 titles, got %d (%v)", len(got), got)
	}
}
