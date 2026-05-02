package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnvFile(t *testing.T) {
	t.Setenv("RELAY_AGENT_CONFIG", filepath.Join(t.TempDir(), "relay-agent.env"))
	configPath := os.Getenv("RELAY_AGENT_CONFIG")
	content := []byte(`
CONTROLLER_BASE_URL=https://controller.example.com/
CONTROLLER_TOKEN=test-token
ZT_NETWORK_ID=8056c2e21c000001
RELAY_NAME=relay-01
HEARTBEAT_INTERVAL_SECONDS=15
HTTP_TIMEOUT_SECONDS=3
DRY_RUN=true
`)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.ControllerBaseURL != "https://controller.example.com" {
		t.Fatalf("unexpected controller URL: %s", cfg.ControllerBaseURL)
	}
	if cfg.HeartbeatInterval.String() != "15s" {
		t.Fatalf("unexpected heartbeat interval: %s", cfg.HeartbeatInterval)
	}
	if !cfg.DryRun {
		t.Fatal("expected dry-run to be enabled")
	}
}

func TestValidateMissingRequiredConfig(t *testing.T) {
	cfg := Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}
