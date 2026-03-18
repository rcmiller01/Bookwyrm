package importer

import "strings"

type TemplateValues struct {
	Author      string
	Title       string
	Year        string
	Series      string
	SeriesIndex string
	Ext         string
	WorkID      string
	EditionID   string
}

func ApplyTemplate(tpl string, v TemplateValues) string {
	repl := map[string]string{
		"{Author}":      strings.TrimSpace(v.Author),
		"{Title}":       strings.TrimSpace(v.Title),
		"{Year}":        strings.TrimSpace(v.Year),
		"{Series}":      strings.TrimSpace(v.Series),
		"{SeriesIndex}": strings.TrimSpace(v.SeriesIndex),
		"{Ext}":         strings.TrimSpace(v.Ext),
		"{WorkID}":      strings.TrimSpace(v.WorkID),
		"{EditionID}":   strings.TrimSpace(v.EditionID),
	}
	out := tpl
	for k, val := range repl {
		out = strings.ReplaceAll(out, k, val)
	}
	// Clean up optional Year group and empty segments.
	out = strings.ReplaceAll(out, " ()", "")
	out = strings.ReplaceAll(out, "()", "")
	out = strings.ReplaceAll(out, "  ", " ")
	out = strings.ReplaceAll(out, "//", "/")
	out = strings.ReplaceAll(out, `\\`, `\`)
	out = strings.TrimSpace(out)
	return out
}
