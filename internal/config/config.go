package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultConfigPath        = "/etc/relay-agent/relay-agent.env"
	defaultHeartbeatInterval = 30 * time.Second
	defaultHTTPTimeout       = 10 * time.Second
	defaultStatePath         = "/var/lib/relay-agent/state.json"
)

type Config struct {
	ControllerBaseURL string
	ControllerToken   string
	ZTNetworkID       string
	RelayName         string
	LogLevel          string
	ConfigPath        string
	StatePath         string
	HeartbeatInterval time.Duration
	HTTPTimeout       time.Duration
	DryRun            bool
}

func Load() (Config, error) {
	cfg := Config{
		ConfigPath:        envString("RELAY_AGENT_CONFIG", defaultConfigPath),
		StatePath:         envString("STATE_PATH", defaultStatePath),
		LogLevel:          strings.ToLower(envString("LOG_LEVEL", "info")),
		HeartbeatInterval: envDuration("HEARTBEAT_INTERVAL_SECONDS", defaultHeartbeatInterval),
		HTTPTimeout:       envDuration("HTTP_TIMEOUT_SECONDS", defaultHTTPTimeout),
		DryRun:            envBool("DRY_RUN", false),
	}

	if err := loadEnvFile(cfg.ConfigPath); err != nil {
		return Config{}, err
	}

	cfg.ControllerBaseURL = strings.TrimRight(envString("CONTROLLER_BASE_URL", cfg.ControllerBaseURL), "/")
	cfg.ControllerToken = envString("CONTROLLER_TOKEN", cfg.ControllerToken)
	cfg.ZTNetworkID = envString("ZT_NETWORK_ID", cfg.ZTNetworkID)
	cfg.RelayName = envString("RELAY_NAME", cfg.RelayName)
	cfg.StatePath = envString("STATE_PATH", cfg.StatePath)
	cfg.LogLevel = strings.ToLower(envString("LOG_LEVEL", cfg.LogLevel))
	cfg.HeartbeatInterval = envDuration("HEARTBEAT_INTERVAL_SECONDS", cfg.HeartbeatInterval)
	cfg.HTTPTimeout = envDuration("HTTP_TIMEOUT_SECONDS", cfg.HTTPTimeout)
	cfg.DryRun = envBool("DRY_RUN", cfg.DryRun)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (cfg Config) Validate() error {
	var missing []string
	if cfg.ControllerBaseURL == "" {
		missing = append(missing, "CONTROLLER_BASE_URL")
	}
	if cfg.ControllerToken == "" {
		missing = append(missing, "CONTROLLER_TOKEN")
	}
	if cfg.ZTNetworkID == "" {
		missing = append(missing, "ZT_NETWORK_ID")
	}
	if cfg.RelayName == "" {
		missing = append(missing, "RELAY_NAME")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}

	parsed, err := url.Parse(cfg.ControllerBaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("CONTROLLER_BASE_URL must be an absolute URL")
	}
	if cfg.HeartbeatInterval <= 0 {
		return errors.New("HEARTBEAT_INTERVAL_SECONDS must be greater than 0")
	}
	if cfg.HTTPTimeout <= 0 {
		return errors.New("HTTP_TIMEOUT_SECONDS must be greater than 0")
	}

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error")
	}

	return nil
}

func loadEnvFile(path string) error {
	if path == "" {
		return nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read config file %s: %w", path, err)
	}

	lines := strings.Split(string(content), "\n")
	for index, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("parse config file %s line %d: expected KEY=VALUE", path, index+1)
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			return fmt.Errorf("parse config file %s line %d: key is empty", path, index+1)
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set env %s from config file: %w", key, err)
			}
		}
	}

	return nil
}

func envString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	seconds, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
