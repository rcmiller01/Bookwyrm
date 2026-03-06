package importer

import (
	"strings"
	"testing"
)

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

func TestBuildNamingPlan_PathLengthDeterministic(t *testing.T) {
	longTitle := strings.Repeat("VeryLongTitleSegment", 20)
	cfg := Config{
		LibraryRoot:             "C:\\library",
		TemplateEbook:           "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
		MaxPathLen:              120,
		ReplaceColon:            true,
	}
	job := Job{WorkID: "work-1"}
	source := "C:\\downloads\\" + longTitle + ".epub"

	plan1 := BuildNamingPlan(cfg, job, source, false)
	plan2 := BuildNamingPlan(cfg, job, source, false)
	if plan1.TargetPath != plan2.TargetPath {
		t.Fatalf("expected deterministic truncation; got %q vs %q", plan1.TargetPath, plan2.TargetPath)
	}
	relative := strings.TrimPrefix(plan1.TargetPath, cfg.LibraryRoot+"\\")
	if len(relative) > cfg.MaxPathLen {
		t.Fatalf("expected relative path <= %d, got %d", cfg.MaxPathLen, len(relative))
	}
}
