package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (h *Handlers) SystemLogsLocation(w http.ResponseWriter, _ *http.Request) {
	logDir := strings.TrimSpace(os.Getenv("BOOKWYRM_LOG_DIR"))
	if logDir == "" {
		logDir = filepath.Clean("./logs")
	}
	info, err := os.Stat(logDir)
	writeJSON(w, map[string]any{
		"log_dir":   logDir,
		"exists":    err == nil && info.IsDir(),
		"file_uri":  "file:///" + filepath.ToSlash(logDir),
		"open_hint": "Use this path in File Explorer if browser cannot open file:// links.",
		"launcher":  filepath.Join(logDir, "launcher.log"),
		"metadata":  filepath.Join(logDir, "metadata-service.log"),
		"indexer":   filepath.Join(logDir, "indexer-service.log"),
		"backend":   filepath.Join(logDir, "backend.log"),
	})
}
