package netops

import (
	"context"
	"fmt"
	"strings"
)

const (
	DefaultNFTFamily = "ip"
	DefaultNFTTable  = "relay_agent"
)

type NATRule struct {
	Name         string
	Type         string
	Family       string
	Table        string
	Src          string
	Dst          string
	InInterface  string
	OutInterface string
	ToAddress    string
	ToPort       string
}

type NFTManager struct {
	runner Runner
}

func NewNFTManager(runner Runner) *NFTManager {
	return &NFTManager{runner: runner}
}

func (manager *NFTManager) ListRuleset(ctx context.Context) (Result, error) {
	return manager.runner.Run(ctx, Command{
		Name: "nft",
		Args: []string{"list", "ruleset"},
	})
}

func (manager *NFTManager) Apply(ctx context.Context, script string) (Result, error) {
	if strings.TrimSpace(script) == "" {
		return Result{}, fmt.Errorf("nft script is required")
	}
	return manager.runner.Run(ctx, Command{
		Name:  "nft",
		Args:  []string{"-f", "-"},
		Stdin: script,
	})
}

func (manager *NFTManager) EnsureNAT(ctx context.Context, rules []NATRule) (Result, error) {
	script, err := BuildNATScript(rules)
	if err != nil {
		return Result{}, err
	}
	return manager.runner.Run(ctx, Command{
		Name:  "nft",
		Args:  []string{"-f", "-"},
		Stdin: script,
	})
}

func BuildNATScript(rules []NATRule) (string, error) {
	var builder strings.Builder
	builder.WriteString("flush table ip relay_agent\n")
	builder.WriteString("table ip relay_agent {\n")
	builder.WriteString("  chain postrouting {\n")
	builder.WriteString("    type nat hook postrouting priority srcnat; policy accept;\n")

	for _, rule := range rules {
		line, err := buildNATRuleLine(rule)
		if err != nil {
			return "", err
		}
		builder.WriteString("    ")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	builder.WriteString("  }\n")
	builder.WriteString("}\n")
	return builder.String(), nil
}

func buildNATRuleLine(rule NATRule) (string, error) {
	ruleType := strings.ToLower(rule.Type)
	if ruleType == "" {
		return "", fmt.Errorf("nat rule type is required")
	}
	if ruleType != "masquerade" && ruleType != "snat" {
		return "", fmt.Errorf("unsupported nat rule type %q", rule.Type)
	}

	var parts []string
	if rule.Src != "" {
		parts = append(parts, "ip", "saddr", rule.Src)
	}
	if rule.Dst != "" {
		parts = append(parts, "ip", "daddr", rule.Dst)
	}
	if rule.InInterface != "" {
		parts = append(parts, "iifname", quote(rule.InInterface))
	}
	if rule.OutInterface != "" {
		parts = append(parts, "oifname", quote(rule.OutInterface))
	}

	switch ruleType {
	case "masquerade":
		parts = append(parts, "masquerade")
	case "snat":
		if rule.ToAddress == "" {
			return "", fmt.Errorf("snat rule toAddress is required")
		}
		parts = append(parts, "snat", "to", rule.ToAddress)
	}

	if rule.Name != "" {
		parts = append(parts, "comment", quote("relay-agent:"+rule.Name))
	}

	return strings.Join(parts, " "), nil
}

func quote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}
