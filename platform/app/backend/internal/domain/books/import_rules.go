package books

import (
	"path/filepath"
	"strings"

	"app-backend/internal/domain/contract"
)

type importRules struct{}

func (r importRules) SupportedExtensions() map[string]bool {
	return map[string]bool{
		".epub": true,
		".mobi": true,
		".azw":  true,
		".azw3": true,
		".pdf":  true,
		".cbz":  true,
		".cbr":  true,
		".mp3":  true,
		".m4b":  true,
		".flac": true,
	}
}

func (r importRules) IsJunk(filename string) bool {
	name := strings.ToLower(strings.TrimSpace(filename))
	if name == "" {
		return true
	}
	base := strings.ToLower(filepath.Base(name))
	if strings.HasPrefix(base, ".") {
		return true
	}
	junkSuffixes := []string{".nfo", ".txt", ".jpg", ".jpeg", ".png", ".srt", ".sub"}
	for _, suffix := range junkSuffixes {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

func (r importRules) GroupFiles(files []string) []contract.FileGroup {
	if len(files) == 0 {
		return []contract.FileGroup{}
	}
	grouped := make(map[string][]string)
	for _, file := range files {
		if r.IsJunk(file) {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file))
		if !r.SupportedExtensions()[ext] {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		if strings.TrimSpace(base) == "" {
			base = "unknown"
		}
		grouped[base] = append(grouped[base], file)
	}
	result := make([]contract.FileGroup, 0, len(grouped))
	for key, group := range grouped {
		result = append(result, contract.FileGroup{Key: key, Files: group})
	}
	return result
}
