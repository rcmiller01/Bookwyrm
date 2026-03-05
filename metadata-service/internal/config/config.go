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

	Enrichment     EnrichmentConfig     `yaml:"enrichment"`
	Recommendation RecommendationConfig `yaml:"recommendation"`
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

type RecommendationConfig struct {
	CacheTTLHours      int `yaml:"cache_ttl_hours"`
	MaxDepth           int `yaml:"max_depth"`
	MaxCandidatePool   int `yaml:"max_candidate_pool"`
	SeriesLimit        int `yaml:"series_limit"`
	AuthorLimit        int `yaml:"author_limit"`
	MaxSubjects        int `yaml:"max_subjects"`
	MaxWorksPerSubject int `yaml:"max_works_per_subject"`
	RelationshipLimit  int `yaml:"relationship_limit"`

	Weights struct {
		SeriesNeighbor  float64 `yaml:"series_neighbor"`
		SameSeries      float64 `yaml:"same_series"`
		SameAuthor      float64 `yaml:"same_author"`
		SharedSubject   float64 `yaml:"shared_subject"`
		ExplicitRelated float64 `yaml:"explicit_related"`
		PreferenceBoost float64 `yaml:"preference_boost"`
	} `yaml:"weights"`
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

	if cfg.Recommendation.CacheTTLHours <= 0 {
		cfg.Recommendation.CacheTTLHours = 2
	}
	if cfg.Recommendation.MaxDepth <= 0 {
		cfg.Recommendation.MaxDepth = 2
	}
	if cfg.Recommendation.MaxCandidatePool <= 0 {
		cfg.Recommendation.MaxCandidatePool = 250
	}
	if cfg.Recommendation.SeriesLimit <= 0 {
		cfg.Recommendation.SeriesLimit = 10
	}
	if cfg.Recommendation.AuthorLimit <= 0 {
		cfg.Recommendation.AuthorLimit = 25
	}
	if cfg.Recommendation.MaxSubjects <= 0 {
		cfg.Recommendation.MaxSubjects = 10
	}
	if cfg.Recommendation.MaxWorksPerSubject <= 0 {
		cfg.Recommendation.MaxWorksPerSubject = 10
	}
	if cfg.Recommendation.RelationshipLimit <= 0 {
		cfg.Recommendation.RelationshipLimit = 100
	}
	if cfg.Recommendation.Weights.SeriesNeighbor <= 0 {
		cfg.Recommendation.Weights.SeriesNeighbor = 1.00
	}
	if cfg.Recommendation.Weights.SameSeries <= 0 {
		cfg.Recommendation.Weights.SameSeries = 0.85
	}
	if cfg.Recommendation.Weights.SameAuthor <= 0 {
		cfg.Recommendation.Weights.SameAuthor = 0.70
	}
	if cfg.Recommendation.Weights.SharedSubject <= 0 {
		cfg.Recommendation.Weights.SharedSubject = 0.55
	}
	if cfg.Recommendation.Weights.ExplicitRelated <= 0 {
		cfg.Recommendation.Weights.ExplicitRelated = 0.90
	}
	if cfg.Recommendation.Weights.PreferenceBoost <= 0 {
		cfg.Recommendation.Weights.PreferenceBoost = 0.05
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
