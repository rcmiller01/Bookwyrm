package librarything

import "testing"

func TestParseLibraryThingTitles(t *testing.T) {
	html := `
	<div>
	  <a href="/work/1234">The Left Hand of Darkness</a>
	  <a href="/book/5678">The Dispossessed</a>
	  <a href="/about">About</a>
	</div>`
	got := parseLibraryThingTitles(html)
	if len(got) != 2 {
		t.Fatalf("expected 2 titles, got %d (%v)", len(got), got)
	}
}
