package importer

import (
	"path"
	"path/filepath"
	"strings"
)

type NamingPlan struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	Format     string `json:"format"`
}

func BuildNamingPlan(cfg Config, job Job, sourcePath string, audiobookTrackMode bool) NamingPlan {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(sourcePath)), ".")
	title := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	values := TemplateValues{
		Author:      "Unknown Author",
		Title:       title,
		Year:        "",
		Series:      "",
		SeriesIndex: "",
		Ext:         ext,
		WorkID:      fallback(job.WorkID, "unknown-work"),
		EditionID:   job.EditionID,
	}
	var rel string
	if isAudiobookExt(ext) && audiobookTrackMode {
		folderRel := ApplyTemplate(cfg.TemplateAudiobookFolder, values)
		folderRel = SanitizeRelativePath(folderRel, cfg.ReplaceColon, cfg.MaxPathLen)
		name := SanitizeRelativePath(filepath.Base(sourcePath), cfg.ReplaceColon, cfg.MaxPathLen)
		rel = cleanRelativeForStyle(joinWithStyle(folderRel, name, pathStyleForRoot(cfg.LibraryRoot)), pathStyleForRoot(cfg.LibraryRoot))
	} else {
		tpl := cfg.TemplateEbook
		if isAudiobookExt(ext) {
			tpl = cfg.TemplateAudiobookSingle
		}
		rel = ApplyTemplate(tpl, values)
		rel = SanitizeRelativePath(rel, cfg.ReplaceColon, cfg.MaxPathLen)
	}
	target := joinWithStyle(cfg.LibraryRoot, rel, pathStyleForRoot(cfg.LibraryRoot))
	return NamingPlan{
		SourcePath: sourcePath,
		TargetPath: target,
		Format:     ext,
	}
}

func pathStyleForRoot(root string) string {
	if strings.Contains(root, "\\") {
		return "\\"
	}
	return "/"
}

func joinWithStyle(base string, rel string, separator string) string {
	if separator == "\\" {
		baseClean := cleanRelativeForStyle(base, separator)
		relClean := cleanRelativeForStyle(rel, separator)
		if baseClean == "" {
			return relClean
		}
		if relClean == "" {
			return baseClean
		}
		return baseClean + separator + relClean
	}
	return filepath.Clean(filepath.Join(base, filepath.FromSlash(strings.ReplaceAll(rel, "\\", "/"))))
}

func cleanRelativeForStyle(value string, separator string) string {
	v := strings.TrimSpace(value)
	v = strings.ReplaceAll(v, "\\", "/")
	v = strings.Trim(v, "/")
	if v == "" {
		return ""
	}
	v = path.Clean(v)
	if v == "." {
		return ""
	}
	if separator == "\\" {
		return strings.ReplaceAll(v, "/", "\\")
	}
	return v
}

func isAudiobookExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "m4b", "mp3", "m4a", "flac":
		return true
	default:
		return false
	}
}

func fallback(v string, fb string) string {
	if strings.TrimSpace(v) == "" {
		return fb
	}
	return strings.TrimSpace(v)
}
