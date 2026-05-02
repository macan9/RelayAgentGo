package netops

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Command struct {
	Name  string
	Args  []string
	Stdin string
}

func (command Command) String() string {
	if len(command.Args) == 0 {
		return command.Name
	}
	return command.Name + " " + strings.Join(command.Args, " ")
}

type Result struct {
	Command Command
	Stdout  string
	Stderr  string
	DryRun  bool
}

type Runner interface {
	Run(context.Context, Command) (Result, error)
}

type ExecRunner struct{}

func (runner ExecRunner) Run(ctx context.Context, command Command) (Result, error) {
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	if command.Stdin != "" {
		cmd.Stdin = strings.NewReader(command.Stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := Result{
		Command: command,
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
	}
	if err != nil {
		return result, fmt.Errorf("%s failed: %w: %s", command.String(), err, strings.TrimSpace(result.Stderr))
	}

	return result, nil
}

type DryRunRunner struct {
	Commands []Command
}

func (runner *DryRunRunner) Run(ctx context.Context, command Command) (Result, error) {
	runner.Commands = append(runner.Commands, command)
	return Result{
		Command: command,
		DryRun:  true,
	}, nil
}
