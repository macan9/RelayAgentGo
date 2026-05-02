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
ZT_INTERFACE_PREFIX=zt
RELAY_NAME=relay-01
PUBLIC_IP_PROBE_URL=https://public-ip.example.com
LATENCY_PROBE_URL=https://latency.example.com/ping
HEARTBEAT_INTERVAL_SECONDS=15
HTTP_TIMEOUT_SECONDS=3
CONTROLLER_RETRY_MAX=4
CONTROLLER_RETRY_WAIT_MS=50
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
	if cfg.PublicIPProbeURL != "https://public-ip.example.com" {
		t.Fatalf("unexpected public ip probe URL: %s", cfg.PublicIPProbeURL)
	}
	if cfg.LatencyProbeURL != "https://latency.example.com/ping" {
		t.Fatalf("unexpected latency probe URL: %s", cfg.LatencyProbeURL)
	}
	if cfg.ControllerRetryMax != 4 {
		t.Fatalf("unexpected retry max: %d", cfg.ControllerRetryMax)
	}
	if cfg.ControllerRetryWait.String() != "50ms" {
		t.Fatalf("unexpected retry wait: %s", cfg.ControllerRetryWait)
	}
}

func TestValidateMissingRequiredConfig(t *testing.T) {
	cfg := Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}
