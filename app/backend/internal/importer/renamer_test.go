package importer

import "testing"

func TestApplyTemplateAndSanitize(t *testing.T) {
	rel := ApplyTemplate("{Author}/{Title} ({Year})/{Title}:{Author}.{Ext}", TemplateValues{
		Author: "Frank Herbert",
		Title:  "Dune",
		Year:   "",
		Ext:    "epub",
	})
	sanitized := SanitizeRelativePath(rel, true, 240)
	expected := "Frank Herbert\\Dune\\Dune -Frank Herbert.epub"
	if sanitized != expected {
		t.Fatalf("unexpected sanitized path\n got: %s\nwant: %s", sanitized, expected)
	}
}
