package importer

import (
	"os"
	"path/filepath"
	"strings"
)

type ScannedFile struct {
	Path string
	Ext  string
	Size int64
}

var supportedExtensions = map[string]struct{}{
	".epub": {},
	".mobi": {},
	".azw3": {},
	".pdf":  {},
	".m4b":  {},
	".mp3":  {},
	".m4a":  {},
	".flac": {},
}

var ignoredExtensions = map[string]struct{}{
	".txt":  {},
	".nfo":  {},
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".gif":  {},
	".sfv":  {},
}

func ScanMediaFiles(root string, maxFiles int) ([]ScannedFile, error) {
	if maxFiles <= 0 {
		maxFiles = 5000
	}
	found := make([]ScannedFile, 0, 32)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if len(found) >= maxFiles {
			return filepath.SkipAll
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(strings.TrimSpace(d.Name()))
		if strings.Contains(name, "sample") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		if _, ignored := ignoredExtensions[ext]; ignored {
			return nil
		}
		if _, ok := supportedExtensions[ext]; !ok {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		found = append(found, ScannedFile{
			Path: path,
			Ext:  ext,
			Size: info.Size(),
		})
		return nil
	})
	return found, err
}

func formatFromExt(ext string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), ".")
}
