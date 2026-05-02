package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"relay-agent-go/internal/buildinfo"
	"relay-agent-go/internal/collector"
	"relay-agent-go/internal/controller"
	"relay-agent-go/internal/reconciler"
	"relay-agent-go/internal/state"
)

type Controller interface {
	Register(context.Context, controller.RegisterRequest) (controller.RegisterResponse, error)
	Heartbeat(context.Context, string, controller.HeartbeatRequest) (controller.HeartbeatResponse, error)
	GetConfig(context.Context, string) (controller.RelayConfig, error)
	ReportApplyResult(context.Context, string, controller.ApplyResultRequest) (controller.ApplyResultResponse, error)
}

type Collector interface {
	Snapshot(context.Context) (collector.Snapshot, error)
}

type StateStore interface {
	Load() (state.State, error)
	Save(state.State) error
}

type Reconciler interface {
	Apply(context.Context, state.State, controller.RelayConfig) (state.State, reconciler.Outcome, error)
}

type Config struct {
	ZTNetworkID       string
	RelayName         string
	Version           string
	HeartbeatInterval time.Duration
	Labels            map[string]string
}

type Service struct {
	config     Config
	controller Controller
	collector  Collector
	store      StateStore
	reconciler Reconciler
	logger     *slog.Logger
}

func New(config Config, controllerClient Controller, metricsCollector Collector, store StateStore, reconciler Reconciler, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if config.Version == "" {
		config.Version = buildinfo.Version
	}
	return &Service{
		config:     config,
		controller: controllerClient,
		collector:  metricsCollector,
		store:      store,
		reconciler: reconciler,
		logger:     logger,
	}
}

func (service *Service) Run(ctx context.Context) error {
	current, err := service.store.Load()
	if err != nil {
		return err
	}

	current, err = service.register(ctx, current)
	if err != nil {
		return err
	}

	if err := service.store.Save(current); err != nil {
		return err
	}

	interval := service.config.HeartbeatInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			next, err := service.heartbeat(ctx, current)
			if err != nil {
				service.logger.Error("heartbeat failed", "error", err)
				continue
			}
			current = next
			if err := service.store.Save(current); err != nil {
				service.logger.Error("save state after heartbeat failed", "error", err)
			}
		}
	}
}

func (service *Service) register(ctx context.Context, current state.State) (state.State, error) {
	snapshot, err := service.collector.Snapshot(ctx)
	if err != nil {
		return current, fmt.Errorf("collect register snapshot: %w", err)
	}

	request := snapshot.RegisterRequest(service.config.ZTNetworkID, service.config.Version, service.config.Labels)
	if current.NodeID != "" {
		request.NodeID = current.NodeID
	} else if service.config.RelayName != "" {
		request.NodeID = service.config.RelayName
	}
	if request.Hostname == "" && service.config.RelayName != "" {
		request.Hostname = service.config.RelayName
	}

	response, err := service.controller.Register(ctx, request)
	if err != nil {
		return current, fmt.Errorf("register relay: %w", err)
	}

	now := time.Now().UTC()
	current.RelayID = response.RelayID
	current.NodeID = response.NodeID
	if current.NodeID == "" {
		current.NodeID = request.NodeID
	}
	current.ConfigVersion = response.ConfigVersion
	current.LastRegisterAt = now
	current.LastControllerSeen = now

	service.logger.Info(
		"relay registered",
		"relayId", current.RelayID,
		"nodeId", current.NodeID,
		"configVersion", current.ConfigVersion,
	)

	return current, nil
}

func (service *Service) heartbeat(ctx context.Context, current state.State) (state.State, error) {
	if current.NodeID == "" {
		return current, fmt.Errorf("node id is empty")
	}

	snapshot, err := service.collector.Snapshot(ctx)
	if err != nil {
		return current, fmt.Errorf("collect heartbeat snapshot: %w", err)
	}

	request := snapshot.HeartbeatRequest(current.NodeID, current.ConfigVersion, current.NFTApplied, current.RouteApplied)
	response, err := service.controller.Heartbeat(ctx, current.NodeID, request)
	if err != nil {
		return current, fmt.Errorf("send heartbeat: %w", err)
	}

	now := time.Now().UTC()
	current.LastHeartbeatAt = now
	current.LastControllerSeen = now

	if response.HasNewConfig || response.ConfigVersion > current.ConfigVersion || !current.NFTApplied || !current.RouteApplied {
		relayConfig, err := service.controller.GetConfig(ctx, current.NodeID)
		if err != nil {
			return current, fmt.Errorf("fetch relay config: %w", err)
		}
		if relayConfig.Version > current.ConfigVersion || !current.NFTApplied || !current.RouteApplied {
			current.ConfigVersion = relayConfig.Version
			current.NFTApplied = false
			current.RouteApplied = false
			current.LastApplyMessage = "config fetched; reconcile pending"
			service.logger.Info("new relay config fetched", "version", relayConfig.Version)

			next, outcome, err := service.reconciler.Apply(ctx, current, relayConfig)
			current = next
			current.LastApplyMessage = outcome.Message

			_, reportErr := service.controller.ReportApplyResult(ctx, current.NodeID, controller.ApplyResultRequest{
				Version:       outcome.Version,
				Success:       err == nil,
				Message:       outcome.Message,
				ChangedRoutes: outcome.ChangedRoutes,
				ChangedRules:  outcome.ChangedRules,
			})
			if reportErr != nil {
				service.logger.Error("report apply result failed", "error", reportErr)
			}
			if err != nil {
				service.logger.Error("reconcile failed", "error", err, "version", relayConfig.Version)
				return current, nil
			}
		}
	}

	return current, nil
}
