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

func TestBuildNamingPlanWithValues_UsesMetadataLayout(t *testing.T) {
	cfg := Config{
		LibraryRoot:             "H:\\Books",
		TemplateEbook:           "{Author}/{Title}/{Title} - {Author}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title} - {Author}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
		MaxPathLen:              240,
		ReplaceColon:            true,
	}
	job := Job{WorkID: "wrk-1"}
	values := TemplateValues{Author: "Andy Weir", Title: "Project Hail Mary", Year: "2021"}
	plan := BuildNamingPlanWithValues(cfg, job, "H:\\Books\\_incoming\\16\\Project Hail Mary.epub", false, values)
	want := "H:\\Books\\Andy Weir\\Project Hail Mary\\Project Hail Mary - Andy Weir.epub"
	if plan.TargetPath != want {
		t.Fatalf("unexpected metadata naming path\n got: %s\nwant: %s", plan.TargetPath, want)
	}
}

func TestBuildNamingPlanWithValues_AudiobookTrackModeKeepsCommonFolder(t *testing.T) {
	cfg := Config{
		LibraryRoot:             "H:\\Books",
		TemplateEbook:           "{Author}/{Title}/{Title} - {Author}.{Ext}",
		TemplateAudiobookSingle: "{Author}/{Title}/{Title} - {Author}.{Ext}",
		TemplateAudiobookFolder: "{Author}/{Title}",
		MaxPathLen:              240,
		ReplaceColon:            true,
	}
	job := Job{WorkID: "wrk-2"}
	values := TemplateValues{Author: "Andy Weir", Title: "Project Hail Mary", Year: "2021"}
	plan := BuildNamingPlanWithValues(cfg, job, "H:\\Books\\_incoming\\16\\Andy Weir - 2021 - Project Hail Mary\\Project Hail Mary (02).mp3", true, values)
	want := "H:\\Books\\Andy Weir\\Project Hail Mary\\Project Hail Mary (02).mp3"
	if plan.TargetPath != want {
		t.Fatalf("unexpected audiobook track naming path\n got: %s\nwant: %s", plan.TargetPath, want)
	}
}
