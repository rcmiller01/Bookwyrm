package launcher

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type ServiceConfig struct {
	Name       string
	Executable string
	Args       []string
	HealthURL  string
	LogFile    string
}

type Config struct {
	BaseDir                 string
	BinDir                  string
	ConfigDir               string
	LogDir                  string
	DataDir                 string
	EnvFile                 string
	Env                     map[string]string
	LaunchURL               string
	OpenBrowserOnFirstStart bool
	HealthTimeout           time.Duration
	StopTimeout             time.Duration
	RestartLimit            int
	RestartWindow           time.Duration
	RestartBaseDelay        time.Duration
	RestartMaxDelay         time.Duration
	Services                []ServiceConfig
	ServiceName             string
}

func LoadConfig(baseOverride string) (Config, error) {
	base := strings.TrimSpace(baseOverride)
	if base == "" {
		base = strings.TrimSpace(os.Getenv("BOOKWYRM_HOME"))
	}
	if base == "" && runtime.GOOS == "windows" {
		base = filepath.Join(os.Getenv("ProgramData"), "Bookwyrm")
	}
	if base == "" {
		base = filepath.Clean("./bookwyrm")
	}

	cfg := Config{
		BaseDir:                 base,
		BinDir:                  filepath.Join(base, "bin"),
		ConfigDir:               filepath.Join(base, "config"),
		LogDir:                  filepath.Join(base, "logs"),
		DataDir:                 filepath.Join(base, "data"),
		LaunchURL:               "http://localhost:8090",
		OpenBrowserOnFirstStart: true,
		HealthTimeout:           90 * time.Second,
		StopTimeout:             20 * time.Second,
		RestartLimit:            5,
		RestartWindow:           5 * time.Minute,
		RestartBaseDelay:        2 * time.Second,
		RestartMaxDelay:         30 * time.Second,
		ServiceName:             "Bookwyrm",
		Env:                     map[string]string{},
	}
	cfg.EnvFile = filepath.Join(cfg.ConfigDir, "bookwyrm.env")
	if err := ensureDirs(cfg); err != nil {
		return Config{}, err
	}

	envMap, err := readEnvFile(cfg.EnvFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	for k, v := range envMap {
		cfg.Env[k] = v
	}
	if v := strings.TrimSpace(cfg.Env["BOOKWYRM_LAUNCH_URL"]); v != "" {
		cfg.LaunchURL = v
	}
	cfg.OpenBrowserOnFirstStart = envBoolWithDefault(cfg.Env["BOOKWYRM_OPEN_BROWSER_ON_FIRST_START"], true)
	cfg.HealthTimeout = envDurationWithDefault(cfg.Env["BOOKWYRM_HEALTH_TIMEOUT_SEC"], 90*time.Second)
	cfg.StopTimeout = envDurationWithDefault(cfg.Env["BOOKWYRM_STOP_TIMEOUT_SEC"], 20*time.Second)
	cfg.RestartLimit = envIntWithDefault(cfg.Env["BOOKWYRM_RESTART_LIMIT"], 5)
	cfg.RestartWindow = envDurationWithDefault(cfg.Env["BOOKWYRM_RESTART_WINDOW_SEC"], 5*time.Minute)
	cfg.RestartBaseDelay = envDurationWithDefault(cfg.Env["BOOKWYRM_RESTART_BASE_DELAY_SEC"], 2*time.Second)
	cfg.RestartMaxDelay = envDurationWithDefault(cfg.Env["BOOKWYRM_RESTART_MAX_DELAY_SEC"], 30*time.Second)
	if v := strings.TrimSpace(cfg.Env["BOOKWYRM_SERVICE_NAME"]); v != "" {
		cfg.ServiceName = v
	}

	exeSuffix := ""
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}
	metadataExe := firstNonEmpty(
		cfg.Env["BOOKWYRM_METADATA_EXE"],
		filepath.Join(cfg.BinDir, "metadata-service"+exeSuffix),
	)
	indexerExe := firstNonEmpty(
		cfg.Env["BOOKWYRM_INDEXER_EXE"],
		filepath.Join(cfg.BinDir, "indexer-service"+exeSuffix),
	)
	backendExe := firstNonEmpty(
		cfg.Env["BOOKWYRM_BACKEND_EXE"],
		filepath.Join(cfg.BinDir, "backend"+exeSuffix),
	)
	metadataCfg := firstNonEmpty(
		cfg.Env["METADATA_CONFIG_PATH"],
		filepath.Join(cfg.ConfigDir, "metadata-service.yaml"),
	)

	cfg.Services = []ServiceConfig{
		{
			Name:       "metadata-service",
			Executable: metadataExe,
			Args:       []string{metadataCfg},
			HealthURL:  firstNonEmpty(cfg.Env["BOOKWYRM_METADATA_HEALTH_URL"], "http://localhost:8080/healthz"),
			LogFile:    filepath.Join(cfg.LogDir, "metadata-service.log"),
		},
		{
			Name:       "indexer-service",
			Executable: indexerExe,
			HealthURL:  firstNonEmpty(cfg.Env["BOOKWYRM_INDEXER_HEALTH_URL"], "http://localhost:8091/v1/indexer/health"),
			LogFile:    filepath.Join(cfg.LogDir, "indexer-service.log"),
		},
		{
			Name:       "backend",
			Executable: backendExe,
			HealthURL:  firstNonEmpty(cfg.Env["BOOKWYRM_BACKEND_HEALTH_URL"], "http://localhost:8090/api/v1/healthz"),
			LogFile:    filepath.Join(cfg.LogDir, "backend.log"),
		},
	}
	return cfg, nil
}

func ensureDirs(cfg Config) error {
	for _, p := range []string{cfg.BaseDir, cfg.BinDir, cfg.ConfigDir, cfg.LogDir, cfg.DataDir} {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if err := os.MkdirAll(p, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func readEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	out := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if key != "" {
			out[key] = val
		}
	}
	return out, scanner.Err()
}

func envIntWithDefault(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envDurationWithDefault(raw string, fallback time.Duration) time.Duration {
	secs := envIntWithDefault(raw, int(fallback.Seconds()))
	return time.Duration(secs) * time.Second
}

func envBoolWithDefault(raw string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
