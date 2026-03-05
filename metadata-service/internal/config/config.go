package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`

	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DBName   string `yaml:"dbname"`
	} `yaml:"database"`

	Providers map[string]ProviderConfig `yaml:"providers"`

	ProviderDispatchPolicy DispatchPolicyConfig `yaml:"-"`

	HealthMonitor struct {
		Enabled         bool `yaml:"enabled"`
		IntervalMinutes int  `yaml:"interval_minutes"`
	} `yaml:"health_monitor"`

	Enrichment EnrichmentConfig `yaml:"enrichment"`
}

type ProviderConfig struct {
	Enabled         bool   `yaml:"enabled"`
	TimeoutSeconds  int    `yaml:"timeout_seconds"`
	RateLimit       int    `yaml:"rate_limit"`
	Priority        int    `yaml:"priority"`
	APIKey          string `yaml:"api_key"`
	QuarantineMode  string `yaml:"quarantine_mode"`
	DisableDispatch bool   `yaml:"disable_dispatch"`
}

type DispatchPolicyConfig struct {
	QuarantineMode string
	Source         string
}

type EnrichmentConfig struct {
	Enabled           bool `yaml:"enabled"`
	WorkerCount       int  `yaml:"worker_count"`
	MaxJobsPerRequest int  `yaml:"max_jobs_per_request"`

	Limits struct {
		MaxAuthorWorks  int `yaml:"max_author_works"`
		MaxWorkEditions int `yaml:"max_work_editions"`
	} `yaml:"limits"`

	Preferences struct {
		Languages []string `yaml:"languages"`
		Formats   []string `yaml:"formats"`
	} `yaml:"preferences"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}

	cfg.ProviderDispatchPolicy.QuarantineMode = "last_resort"
	cfg.ProviderDispatchPolicy.Source = "default"

	if cfg.Providers == nil {
		cfg.Providers = map[string]ProviderConfig{}
	}

	if cfg.Enrichment.WorkerCount <= 0 {
		cfg.Enrichment.WorkerCount = 2
	}
	if cfg.Enrichment.MaxJobsPerRequest <= 0 {
		cfg.Enrichment.MaxJobsPerRequest = 5
	}
	if cfg.Enrichment.Limits.MaxAuthorWorks <= 0 {
		cfg.Enrichment.Limits.MaxAuthorWorks = 50
	}
	if cfg.Enrichment.Limits.MaxWorkEditions <= 0 {
		cfg.Enrichment.Limits.MaxWorkEditions = 100
	}

	hasDispatchPolicy := false
	if pc, ok := cfg.Providers["dispatch_policy"]; ok {
		hasDispatchPolicy = true
		mode := strings.ToLower(strings.TrimSpace(pc.QuarantineMode))
		if mode == "" {
			if pc.DisableDispatch {
				mode = "disabled"
			} else {
				mode = "last_resort"
			}
		}
		cfg.ProviderDispatchPolicy.QuarantineMode = mode
		cfg.ProviderDispatchPolicy.Source = "providers.dispatch_policy.quarantine_mode"
		delete(cfg.Providers, "dispatch_policy")
	}

	if quarantine, ok := cfg.Providers["quarantine"]; ok {
		if !hasDispatchPolicy {
			mode := strings.ToLower(strings.TrimSpace(quarantine.QuarantineMode))
			if mode == "" {
				if quarantine.DisableDispatch {
					mode = "disabled"
				} else {
					mode = "last_resort"
				}
			}
			cfg.ProviderDispatchPolicy.QuarantineMode = mode
			cfg.ProviderDispatchPolicy.Source = "providers.quarantine.disable_dispatch"
		}
		delete(cfg.Providers, "quarantine")
	}

	// environment overrides
	if v := os.Getenv("DATABASE_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DATABASE_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DATABASE_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("DATABASE_NAME"); v != "" {
		cfg.Database.DBName = v
	}
	// API key overrides via environment (e.g. PROVIDER_GOOGLEBOOKS_API_KEY)
	for name, pc := range cfg.Providers {
		envKey := "PROVIDER_" + strings.ToUpper(name) + "_API_KEY"
		if v := os.Getenv(envKey); v != "" {
			pc.APIKey = v
			cfg.Providers[name] = pc
		}
	}

	return &cfg, nil
}
