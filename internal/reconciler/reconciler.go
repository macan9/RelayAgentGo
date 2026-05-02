package reconciler

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"relay-agent-go/internal/controller"
	"relay-agent-go/internal/netops"
	"relay-agent-go/internal/state"
)

type Outcome struct {
	Version       int64
	Applied       bool
	Message       string
	ChangedRoutes []string
	ChangedRules  []string
}

type Reconciler struct {
	sysctl *netops.Sysctl
	routes *netops.RouteManager
	nft    *netops.NFTManager
	logger *slog.Logger
}

func New(sysctl *netops.Sysctl, routes *netops.RouteManager, nft *netops.NFTManager, logger *slog.Logger) *Reconciler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reconciler{
		sysctl: sysctl,
		routes: routes,
		nft:    nft,
		logger: logger,
	}
}

func (reconciler *Reconciler) Apply(ctx context.Context, current state.State, desired controller.RelayConfig) (state.State, Outcome, error) {
	outcome := Outcome{Version: desired.Version}

	if err := validateRelayConfig(desired); err != nil {
		outcome.Message = err.Error()
		return current, outcome, err
	}

	if desired.Version < current.ConfigVersion && current.NFTApplied && current.RouteApplied {
		outcome.Applied = false
		outcome.Message = "stale config ignored"
		return current, outcome, nil
	}

	next := current
	if desired.Version > next.ConfigVersion {
		next.ConfigVersion = desired.Version
	}

	for key, value := range desired.Sysctl {
		result, err := reconciler.sysctl.Set(ctx, key, value)
		if err != nil {
			outcome.Message = fmt.Sprintf("apply sysctl %s failed: %v", key, err)
			return next, outcome, err
		}
		outcome.ChangedRules = append(outcome.ChangedRules, result.Command.String())
	}

	if len(desired.NAT) > 0 {
		rules := make([]netops.NATRule, 0, len(desired.NAT))
		for _, nat := range desired.NAT {
			rule, err := toNATRule(nat)
			if err != nil {
				outcome.Message = err.Error()
				return next, outcome, err
			}
			rules = append(rules, rule)
		}
		result, err := reconciler.nft.EnsureNAT(ctx, rules)
		if err != nil {
			outcome.Message = fmt.Sprintf("apply nft rules failed: %v", err)
			return next, outcome, err
		}
		outcome.ChangedRules = append(outcome.ChangedRules, result.Command.String())
	}

	for _, route := range desired.Routes {
		result, err := reconciler.routes.Replace(ctx, netops.Route{
			Dst:    route.Dst,
			Via:    route.Via,
			Dev:    route.Dev,
			Metric: route.Metric,
		})
		if err != nil {
			outcome.Message = fmt.Sprintf("apply route %s failed: %v", route.Dst, err)
			return next, outcome, err
		}
		outcome.ChangedRoutes = append(outcome.ChangedRoutes, result.Command.String())
	}

	next.NFTApplied = true
	next.RouteApplied = true
	next.LastApplyMessage = "applied successfully"
	outcome.Applied = true
	outcome.Message = "applied successfully"

	return next, outcome, nil
}

func validateRelayConfig(config controller.RelayConfig) error {
	if config.Version <= 0 {
		return fmt.Errorf("config version must be greater than 0")
	}

	for key, value := range config.Sysctl {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("sysctl key is required")
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("sysctl value for %s is required", key)
		}
	}

	for _, route := range config.Routes {
		if strings.TrimSpace(route.Dst) == "" {
			return fmt.Errorf("route destination is required")
		}
		if _, _, err := net.ParseCIDR(route.Dst); err != nil {
			return fmt.Errorf("invalid route destination %q: %w", route.Dst, err)
		}
		if strings.TrimSpace(route.Via) == "" && strings.TrimSpace(route.Dev) == "" {
			return fmt.Errorf("route %s must set via or dev", route.Dst)
		}
	}

	for _, nat := range config.NAT {
		if strings.TrimSpace(nat.Type) == "" {
			return fmt.Errorf("nat rule type is required")
		}
		switch strings.ToLower(nat.Type) {
		case "masquerade":
		case "snat":
			if strings.TrimSpace(nat.ToAddress) == "" {
				return fmt.Errorf("snat nat rule %s requires toAddress", nat.Name)
			}
		default:
			return fmt.Errorf("unsupported nat rule type %q", nat.Type)
		}
	}

	return nil
}

func toNATRule(config controller.NATConfig) (netops.NATRule, error) {
	return netops.NATRule{
		Name:         config.Name,
		Type:         config.Type,
		Family:       defaultIfEmpty(config.Family, netops.DefaultNFTFamily),
		Table:        defaultIfEmpty(config.Table, netops.DefaultNFTTable),
		Src:          config.Src,
		Dst:          config.Dst,
		InInterface:  config.InInterface,
		OutInterface: config.OutInterface,
		ToAddress:    config.ToAddress,
		ToPort:       config.ToPort,
	}, nil
}

func defaultIfEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
