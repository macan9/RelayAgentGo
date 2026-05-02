package netops

import (
	"context"
	"fmt"
)

type Sysctl struct {
	runner Runner
}

func NewSysctl(runner Runner) *Sysctl {
	return &Sysctl{runner: runner}
}

func (sysctl *Sysctl) Set(ctx context.Context, key string, value string) (Result, error) {
	if key == "" {
		return Result{}, fmt.Errorf("sysctl key is required")
	}
	if value == "" {
		return Result{}, fmt.Errorf("sysctl value is required")
	}

	return sysctl.runner.Run(ctx, Command{
		Name: "sysctl",
		Args: []string{"-w", key + "=" + value},
	})
}
