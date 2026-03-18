package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanMediaFilesFiltersSupportedAndIgnoresJunk(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "book.epub"), "epub-data")
	mustWriteFile(t, filepath.Join(root, "notes.txt"), "notes")
	mustWriteFile(t, filepath.Join(root, "cover.jpg"), "img")
	mustWriteFile(t, filepath.Join(root, "sample.mp3"), "sample")
	mustWriteFile(t, filepath.Join(root, "audio.m4b"), "audio")

	files, err := ScanMediaFiles(root, 100)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 supported files, got %d", len(files))
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
}
