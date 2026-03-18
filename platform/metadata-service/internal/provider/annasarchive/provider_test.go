package annasarchive

import "testing"

func TestParseSearchResultsFindsMD5Records(t *testing.T) {
	html := `
	<div>
	  <a href="/md5/1234567890abcdef1234567890abcdef">Dune Messiah</a>
	  <a href="/md5/1234567890ABCDEF1234567890ABCDEF">Duplicate Case Variant</a>
	  <a href="/book/not-a-record">Ignore Me</a>
	</div>`

	results := parseSearchResults(html, "https://annas-archive.gd")
	if len(results) != 1 {
		t.Fatalf("expected 1 unique result, got %d", len(results))
	}
	if results[0].MD5 != "1234567890abcdef1234567890abcdef" {
		t.Fatalf("unexpected md5: %q", results[0].MD5)
	}
	if results[0].Title != "Dune Messiah" {
		t.Fatalf("unexpected title: %q", results[0].Title)
	}
	if results[0].DetailURL != "https://annas-archive.gd/md5/1234567890abcdef1234567890abcdef" {
		t.Fatalf("unexpected detail url: %q", results[0].DetailURL)
	}
}

func TestParseDetailRecordExtractsMetadata(t *testing.T) {
	html := `
	<html>
	  <head>
	    <title>Dune Messiah - Anna's Archive</title>
	  </head>
	  <body>
	    <h1>Dune Messiah</h1>
	    <a href="/author/frank-herbert">Frank Herbert</a>
	    <div>Publisher Penguin Books</div>
	    <div>Publication year 1969</div>
	    <div>File type epub</div>
	    <div>ISBN 9780441172696</div>
	  </body>
	</html>`

	record := parseDetailRecord(html, "https://annas-archive.gd", "annasarchive:md5:1234567890abcdef1234567890abcdef")
	if record.MD5 != "1234567890abcdef1234567890abcdef" {
		t.Fatalf("unexpected md5: %q", record.MD5)
	}
	if record.Title != "Dune Messiah" {
		t.Fatalf("unexpected title: %q", record.Title)
	}
	if len(record.Authors) != 1 || record.Authors[0] != "Frank Herbert" {
		t.Fatalf("unexpected authors: %#v", record.Authors)
	}
	if record.Publisher != "Penguin Books" {
		t.Fatalf("unexpected publisher: %q", record.Publisher)
	}
	if record.Year != 1969 {
		t.Fatalf("unexpected year: %d", record.Year)
	}
	if record.Format != "epub" {
		t.Fatalf("unexpected format: %q", record.Format)
	}
	if len(record.ISBNs) != 1 || record.ISBNs[0].Value != "9780441172696" {
		t.Fatalf("unexpected identifiers: %#v", record.ISBNs)
	}
}

func TestNormalizeProviderIDAcceptsKnownShapes(t *testing.T) {
	md5 := "1234567890abcdef1234567890abcdef"
	cases := []string{
		md5,
		"annasarchive:md5:" + md5,
		"wrk_aa_" + md5,
		"edn_aa_" + md5,
		"https://annas-archive.gd/md5/" + md5,
	}

	for _, input := range cases {
		if got := normalizeProviderID(input); got != md5 {
			t.Fatalf("normalizeProviderID(%q) = %q, want %q", input, got, md5)
		}
	}
}
