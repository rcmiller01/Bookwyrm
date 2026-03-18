package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigReadsEnvFile(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	envPath := filepath.Join(configDir, "bookwyrm.env")
	content := "BOOKWYRM_LAUNCH_URL=http://localhost:9000\nBOOKWYRM_RESTART_LIMIT=7\nLIBRARY_ROOT=D:\\Media\\Books\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	cfg, err := LoadConfig(base)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.LaunchURL != "http://localhost:9000" {
		t.Fatalf("expected launch URL override, got %s", cfg.LaunchURL)
	}
	if cfg.RestartLimit != 7 {
		t.Fatalf("expected restart limit 7, got %d", cfg.RestartLimit)
	}
	if cfg.Env["LIBRARY_ROOT"] != "D:\\Media\\Books" {
		t.Fatalf("expected LIBRARY_ROOT in env map")
	}
}

