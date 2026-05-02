package netops

import (
	"context"
	"strings"
	"testing"
)

func TestSysctlSetBuildsCommand(t *testing.T) {
	runner := &DryRunRunner{}
	manager := NewSysctl(runner)

	if _, err := manager.Set(context.Background(), "net.ipv4.ip_forward", "1"); err != nil {
		t.Fatalf("set sysctl: %v", err)
	}

	assertCommand(t, runner.Commands[0], "sysctl", "-w", "net.ipv4.ip_forward=1")
}

func TestRouteReplaceBuildsCommand(t *testing.T) {
	runner := &DryRunRunner{}
	manager := NewRouteManager(runner)

	if _, err := manager.Replace(context.Background(), Route{
		Dst:    "10.20.0.0/24",
		Via:    "10.147.17.1",
		Dev:    "ztxxxxxx",
		Metric: 100,
	}); err != nil {
		t.Fatalf("replace route: %v", err)
	}

	assertCommand(t, runner.Commands[0], "ip", "route", "replace", "10.20.0.0/24", "via", "10.147.17.1", "dev", "ztxxxxxx", "metric", "100")
}

func TestRouteDeleteBuildsCommand(t *testing.T) {
	runner := &DryRunRunner{}
	manager := NewRouteManager(runner)

	if _, err := manager.Delete(context.Background(), Route{Dst: "10.20.0.0/24", Dev: "ztxxxxxx"}); err != nil {
		t.Fatalf("delete route: %v", err)
	}

	assertCommand(t, runner.Commands[0], "ip", "route", "del", "10.20.0.0/24", "dev", "ztxxxxxx")
}

func TestBuildNATScript(t *testing.T) {
	script, err := BuildNATScript([]NATRule{
		{
			Name:         "zt-to-public",
			Type:         "masquerade",
			Src:          "10.147.17.0/24",
			OutInterface: "eth0",
		},
	})
	if err != nil {
		t.Fatalf("build nat script: %v", err)
	}

	expectedParts := []string{
		"flush table ip relay_agent",
		"table ip relay_agent",
		"type nat hook postrouting priority srcnat",
		`ip saddr 10.147.17.0/24 oifname "eth0" masquerade comment "relay-agent:zt-to-public"`,
	}
	for _, part := range expectedParts {
		if !strings.Contains(script, part) {
			t.Fatalf("script missing %q:\n%s", part, script)
		}
	}
}

func TestBuildNATScriptRejectsUnsupportedRule(t *testing.T) {
	_, err := BuildNATScript([]NATRule{{Type: "dnat"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNFTEnsureNATUsesDryRunRunner(t *testing.T) {
	runner := &DryRunRunner{}
	manager := NewNFTManager(runner)

	result, err := manager.EnsureNAT(context.Background(), []NATRule{{Type: "masquerade", Src: "10.147.17.0/24"}})
	if err != nil {
		t.Fatalf("ensure nat: %v", err)
	}
	if !result.DryRun {
		t.Fatal("expected dry-run result")
	}
	assertCommand(t, runner.Commands[0], "nft", "-f", "-")
	if !strings.Contains(runner.Commands[0].Stdin, "masquerade") {
		t.Fatalf("expected nft script in dry-run command stdin: %#v", runner.Commands[0].Stdin)
	}
}

func assertCommand(t *testing.T, command Command, name string, args ...string) {
	t.Helper()
	if command.Name != name {
		t.Fatalf("unexpected command name: %s", command.Name)
	}
	if len(command.Args) != len(args) {
		t.Fatalf("unexpected args length: %#v", command.Args)
	}
	for i, arg := range args {
		if command.Args[i] != arg {
			t.Fatalf("unexpected arg %d: got %q want %q; all args=%#v", i, command.Args[i], arg, command.Args)
		}
	}
}
