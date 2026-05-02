package reconciler

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"relay-agent-go/internal/controller"
	"relay-agent-go/internal/netops"
	"relay-agent-go/internal/state"
)

func TestApplyRunsSysctlNatRoutesInOrder(t *testing.T) {
	runner := &netops.DryRunRunner{}
	reconciler := New(
		netops.NewSysctl(runner),
		netops.NewRouteManager(runner),
		netops.NewNFTManager(runner),
		slog.Default(),
	)

	next, outcome, err := reconciler.Apply(context.Background(), state.State{}, controller.RelayConfig{
		Version: 1,
		Sysctl: map[string]string{
			"net.ipv4.ip_forward": "1",
		},
		Routes: []controller.RouteConfig{
			{Dst: "10.20.0.0/24", Via: "10.147.17.1", Dev: "zt0", Metric: 100},
		},
		NAT: []controller.NATConfig{
			{Name: "zt-to-public", Type: "masquerade", Src: "10.147.17.0/24", OutInterface: "eth0"},
		},
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !outcome.Applied || next.ConfigVersion != 1 || !next.NFTApplied || !next.RouteApplied {
		t.Fatalf("unexpected apply result: next=%+v outcome=%+v", next, outcome)
	}
	if len(runner.Commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(runner.Commands))
	}
	if runner.Commands[0].Name != "sysctl" || runner.Commands[1].Name != "nft" || runner.Commands[2].Name != "ip" {
		t.Fatalf("unexpected command order: %#v", runner.Commands)
	}
}

func TestApplyRejectsInvalidRoute(t *testing.T) {
	runner := &netops.DryRunRunner{}
	reconciler := New(netops.NewSysctl(runner), netops.NewRouteManager(runner), netops.NewNFTManager(runner), slog.Default())

	_, outcome, err := reconciler.Apply(context.Background(), state.State{}, controller.RelayConfig{
		Version: 1,
		Routes: []controller.RouteConfig{
			{Dst: "invalid"},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(outcome.Message, "invalid route destination") {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
	if len(runner.Commands) != 0 {
		t.Fatalf("expected no commands, got %#v", runner.Commands)
	}
}

func TestApplyIgnoresStaleAppliedConfig(t *testing.T) {
	runner := &netops.DryRunRunner{}
	reconciler := New(netops.NewSysctl(runner), netops.NewRouteManager(runner), netops.NewNFTManager(runner), slog.Default())

	current := state.State{ConfigVersion: 5, NFTApplied: true, RouteApplied: true}
	next, outcome, err := reconciler.Apply(context.Background(), current, controller.RelayConfig{Version: 4})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if outcome.Applied {
		t.Fatal("expected no-op for stale config")
	}
	if next.ConfigVersion != 5 || len(runner.Commands) != 0 {
		t.Fatalf("unexpected no-op result: next=%+v commands=%#v", next, runner.Commands)
	}
}

func TestApplyStopsOnNatFailure(t *testing.T) {
	runner := &failingRunner{failAfter: 1}
	reconciler := New(netops.NewSysctl(runner), netops.NewRouteManager(runner), netops.NewNFTManager(runner), slog.Default())

	_, outcome, err := reconciler.Apply(context.Background(), state.State{}, controller.RelayConfig{
		Version: 1,
		Sysctl:  map[string]string{"net.ipv4.ip_forward": "1"},
		NAT:     []controller.NATConfig{{Name: "zt-to-public", Type: "masquerade"}},
		Routes:  []controller.RouteConfig{{Dst: "10.20.0.0/24", Via: "10.147.17.1"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(outcome.Message, "apply nft rules failed") && !strings.Contains(outcome.Message, "apply nat") {
		t.Fatalf("unexpected outcome: %+v", outcome)
	}
	if runner.calls != 2 {
		t.Fatalf("expected 2 calls (sysctl+nft), got %d", runner.calls)
	}
}

type failingRunner struct {
	calls     int
	failAfter int
}

func (runner *failingRunner) Run(ctx context.Context, command netops.Command) (netops.Result, error) {
	runner.calls++
	if runner.calls > runner.failAfter {
		return netops.Result{}, errors.New("forced failure")
	}
	return netops.Result{Command: command}, nil
}

var _ netops.Runner = (*failingRunner)(nil)
