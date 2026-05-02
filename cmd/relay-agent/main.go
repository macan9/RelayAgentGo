package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"relay-agent-go/internal/buildinfo"
	"relay-agent-go/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info(
		"relay agent starting",
		"version", buildinfo.Version,
		"commit", buildinfo.Commit,
		"controllerBaseURL", cfg.ControllerBaseURL,
		"ztNetworkId", cfg.ZTNetworkID,
		"relayName", cfg.RelayName,
		"heartbeatInterval", cfg.HeartbeatInterval.String(),
		"dryRun", cfg.DryRun,
	)

	<-ctx.Done()
	logger.Info("relay agent stopped")
}

func newLogger(levelName string) *slog.Logger {
	level := slog.LevelInfo
	switch levelName {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}
