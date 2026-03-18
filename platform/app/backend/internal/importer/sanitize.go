package importer

import (
	"path"
	"strings"
)

var invalidPathChars = []string{`<`, `>`, `"`, `|`, `?`, `*`}

func SanitizeRelativePath(rel string, replaceColon bool, maxPathLen int) string {
	parts := strings.Split(strings.ReplaceAll(rel, "\\", "/"), "/")
	outParts := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if replaceColon {
			p = strings.ReplaceAll(p, ":", " -")
		}
		for _, ch := range invalidPathChars {
			p = strings.ReplaceAll(p, ch, "")
		}
		p = strings.ReplaceAll(p, "/", "")
		p = strings.ReplaceAll(p, `\`, "")
		p = strings.Join(strings.Fields(p), " ")
		p = strings.TrimRight(p, ". ")
		if p == "" {
			continue
		}
		if len(p) > 120 {
			p = p[:120]
		}
		outParts = append(outParts, p)
	}
	clean := path.Clean(strings.Join(outParts, "/"))
	if clean == "." {
		clean = ""
	}
	clean = strings.ReplaceAll(clean, "/", "\\")
	if maxPathLen > 0 && len(clean) > maxPathLen {
		clean = clean[:maxPathLen]
	}
	return clean
}
