package annasarchive

import (
	"testing"
)

func TestParseSearchHits_MD5Results(t *testing.T) {
	html := `
	<div>
	  <a href="/md5/ABCDEF0123456789ABCDEF0123456789">The Left Hand of Darkness</a>
	  <a href="/book/not-a-stable-key">Ignore this non-md5 record</a>
	  <a href="/md5/11111111111111111111111111111111"><span>Dune Messiah</span></a>
	</div>`

	hits := parseSearchHits(html)
	if len(hits) != 2 {
		t.Fatalf("expected 2 md5 hits, got %d (%+v)", len(hits), hits)
	}

	if hits[0].MD5 != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("unexpected first md5: %s", hits[0].MD5)
	}
	if hits[0].Title != "The Left Hand of Darkness" {
		t.Fatalf("unexpected first title: %q", hits[0].Title)
	}
	if hits[1].MD5 != "11111111111111111111111111111111" {
		t.Fatalf("unexpected second md5: %s", hits[1].MD5)
	}
	if hits[1].Title != "Dune Messiah" {
		t.Fatalf("unexpected second title: %q", hits[1].Title)
	}
}

func TestParseDetailMetadata_ExtractsCoreFields(t *testing.T) {
	html := `
	<html>
	  <head>
	    <meta property="og:title" content="The Hobbit - Anna's Archive">
	  </head>
	  <body>
	    <div>Authors: J.R.R. Tolkien</div>
	    <div>Publication Year: 1937</div>
	    <div>Publisher: George Allen &amp; Unwin</div>
	    <div>Format: EPUB</div>
	    <div>ISBN: 978-0-261-10300-3, 0261103001</div>
	  </body>
	</html>`

	meta := parseDetailMetadata(html)
	if meta.Title != "The Hobbit" {
		t.Fatalf("unexpected title: %q", meta.Title)
	}
	if len(meta.Authors) != 1 || meta.Authors[0] != "J.R.R. Tolkien" {
		t.Fatalf("unexpected authors: %+v", meta.Authors)
	}
	if meta.Year != 1937 {
		t.Fatalf("unexpected year: %d", meta.Year)
	}
	if meta.Publisher != "George Allen & Unwin" {
		t.Fatalf("unexpected publisher: %q", meta.Publisher)
	}
	if meta.Format != "epub" {
		t.Fatalf("unexpected format: %q", meta.Format)
	}
	if len(meta.ISBNs) != 2 {
		t.Fatalf("expected 2 isbn values, got %d (%+v)", len(meta.ISBNs), meta.ISBNs)
	}
	if meta.ISBNs[0] != "9780261103003" && meta.ISBNs[1] != "9780261103003" {
		t.Fatalf("missing isbn13 in %+v", meta.ISBNs)
	}
	if meta.ISBNs[0] != "0261103001" && meta.ISBNs[1] != "0261103001" {
		t.Fatalf("missing isbn10 in %+v", meta.ISBNs)
	}
}
