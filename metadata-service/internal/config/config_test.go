package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DispatchPolicy_ExtractedFromProviders(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	content := `
server:
  port: 8080

database:
  host: localhost
  port: 5432
  user: metadata
  password: metadata
  dbname: metadata

providers:
  dispatch_policy:
    quarantine_mode: disabled
  openlibrary:
    enabled: true
    timeout_seconds: 10
    rate_limit: 100
    priority: 100

health_monitor:
  enabled: true
  interval_minutes: 5
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.ProviderDispatchPolicy.QuarantineMode != "disabled" {
		t.Fatalf("expected quarantine mode disabled, got %q", cfg.ProviderDispatchPolicy.QuarantineMode)
	}
	if cfg.ProviderDispatchPolicy.Source != "providers.dispatch_policy.quarantine_mode" {
		t.Fatalf("expected dispatch policy source, got %q", cfg.ProviderDispatchPolicy.Source)
	}
	if _, ok := cfg.Providers["dispatch_policy"]; ok {
		t.Fatalf("dispatch_policy should be removed from provider map")
	}
	if _, ok := cfg.Providers["openlibrary"]; !ok {
		t.Fatalf("expected openlibrary provider entry to remain")
	}
}

func TestLoad_DispatchPolicy_BooleanShape(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	content := `
server:
  port: 8080

database:
  host: localhost
  port: 5432
  user: metadata
  password: metadata
  dbname: metadata

providers:
  quarantine:
    disable_dispatch: true
  openlibrary:
    enabled: true
    timeout_seconds: 10
    rate_limit: 100
    priority: 100

health_monitor:
  enabled: true
  interval_minutes: 5
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.ProviderDispatchPolicy.QuarantineMode != "disabled" {
		t.Fatalf("expected quarantine mode disabled from boolean form, got %q", cfg.ProviderDispatchPolicy.QuarantineMode)
	}
	if cfg.ProviderDispatchPolicy.Source != "providers.quarantine.disable_dispatch" {
		t.Fatalf("expected legacy quarantine source, got %q", cfg.ProviderDispatchPolicy.Source)
	}
	if _, ok := cfg.Providers["quarantine"]; ok {
		t.Fatalf("quarantine policy key should be removed from provider map")
	}
}

func TestLoad_DispatchPolicy_PrecedenceOverLegacyShape(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	content := `
server:
  port: 8080

database:
  host: localhost
  port: 5432
  user: metadata
  password: metadata
  dbname: metadata

providers:
  dispatch_policy:
    quarantine_mode: last_resort
  quarantine:
    disable_dispatch: true
  openlibrary:
    enabled: true
    timeout_seconds: 10
    rate_limit: 100
    priority: 100

health_monitor:
  enabled: true
  interval_minutes: 5
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.ProviderDispatchPolicy.QuarantineMode != "last_resort" {
		t.Fatalf("expected dispatch_policy mode to win precedence, got %q", cfg.ProviderDispatchPolicy.QuarantineMode)
	}
	if cfg.ProviderDispatchPolicy.Source != "providers.dispatch_policy.quarantine_mode" {
		t.Fatalf("expected dispatch policy source to win precedence, got %q", cfg.ProviderDispatchPolicy.Source)
	}
}
