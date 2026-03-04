package config

import (
	"os"

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

	HealthMonitor struct {
		Enabled         bool `yaml:"enabled"`
		IntervalMinutes int  `yaml:"interval_minutes"`
	} `yaml:"health_monitor"`
}

type ProviderConfig struct {
	Enabled        bool   `yaml:"enabled"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	RateLimit      int    `yaml:"rate_limit"`
	Priority       int    `yaml:"priority"`
	APIKey         string `yaml:"api_key"`
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
		envKey := "PROVIDER_" + name + "_API_KEY"
		if v := os.Getenv(envKey); v != "" {
			pc.APIKey = v
			cfg.Providers[name] = pc
		}
	}

	return &cfg, nil
}
