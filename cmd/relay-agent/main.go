package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"relay-agent-go/internal/buildinfo"
	"relay-agent-go/internal/collector"
	"relay-agent-go/internal/config"
	"relay-agent-go/internal/controller"
	"relay-agent-go/internal/netops"
	"relay-agent-go/internal/reconciler"
	"relay-agent-go/internal/service"
	"relay-agent-go/internal/state"
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

	controllerClient, err := controller.NewClient(
		cfg.ControllerBaseURL,
		cfg.ControllerToken,
		cfg.HTTPTimeout,
		controller.WithLogger(logger),
		controller.WithRetry(cfg.ControllerRetryMax, cfg.ControllerRetryWait),
	)
	if err != nil {
		logger.Error("failed to initialize controller client", "error", err)
		os.Exit(1)
	}
	_ = controllerClient

	metricsCollector := collector.New(collector.Config{
		ZTInterfacePrefix: cfg.ZTInterfacePrefix,
		PublicIPProbeURL:  cfg.PublicIPProbeURL,
		LatencyProbeURL:   cfg.LatencyProbeURL,
	})
	var runner netops.Runner = netops.ExecRunner{}
	if cfg.DryRun {
		runner = &netops.DryRunRunner{}
	}
	reconcilerService := reconciler.New(
		netops.NewSysctl(runner),
		netops.NewRouteManager(runner),
		netops.NewNFTManager(runner),
		logger,
	)
	stateStore := state.NewStore(cfg.StatePath)
	agentService := service.New(service.Config{
		ZTNetworkID:       cfg.ZTNetworkID,
		RelayName:         cfg.RelayName,
		Version:           buildinfo.Version,
		HeartbeatInterval: cfg.HeartbeatInterval,
	}, controllerClient, metricsCollector, stateStore, reconcilerService, logger)

	logger.Info(
		"relay agent starting",
		"version", buildinfo.Version,
		"commit", buildinfo.Commit,
		"controllerBaseURL", cfg.ControllerBaseURL,
		"ztNetworkId", cfg.ZTNetworkID,
		"ztInterfacePrefix", cfg.ZTInterfacePrefix,
		"relayName", cfg.RelayName,
		"publicIPProbeURL", cfg.PublicIPProbeURL,
		"latencyProbeURL", cfg.LatencyProbeURL,
		"heartbeatInterval", cfg.HeartbeatInterval.String(),
		"httpTimeout", cfg.HTTPTimeout.String(),
		"controllerRetryMax", cfg.ControllerRetryMax,
		"controllerRetryWait", cfg.ControllerRetryWait.String(),
		"dryRun", cfg.DryRun,
	)

	if err := agentService.Run(ctx); err != nil {
		logger.Error("relay agent stopped with error", "error", err)
		os.Exit(1)
	}
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
