package annasarchive
package annasarchive

import "testing"

func TestParseSearchTitles(t *testing.T) {
	html := `
	<div>
	  <a href="/md5/abc123">Dune Messiah</a>
	  <a href="/book/xyz">Hyperion</a>
	  <a href="/about">About</a>
	</div>`
	got := parseSearchTitles(html)
	if len(got) != 2 {
		t.Fatalf("expected 2 titles, got %d (%v)", len(got), got)
	}
	if got[0] != "Dune Messiah" || got[1] != "Hyperion" {
		t.Fatalf("unexpected titles: %v", got)
	}
}
