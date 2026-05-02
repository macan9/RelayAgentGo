package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"relay-agent-go/internal/collector"
	"relay-agent-go/internal/controller"
	"relay-agent-go/internal/reconciler"
	"relay-agent-go/internal/state"
)

func TestRunRegistersAndPersistsState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := &memoryStore{}
	controllerClient := &fakeController{
		registerResponse: controller.RegisterResponse{
			RelayID:       "relay-1",
			NodeID:        "node-1",
			ZTNetworkID:   "8056c2e21c000001",
			ConfigVersion: 7,
		},
	}
	metricsCollector := &fakeCollector{snapshot: collector.Snapshot{Hostname: "relay-01"}}
	configReconciler := &fakeReconciler{}

	svc := New(Config{
		ZTNetworkID:       "8056c2e21c000001",
		RelayName:         "relay-01",
		Version:           "0.1.0",
		HeartbeatInterval: time.Hour,
	}, controllerClient, metricsCollector, store, configReconciler, nil)

	go func() {
		for store.saveCount() == 0 {
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()

	if err := svc.Run(ctx); err != nil {
		t.Fatalf("run service: %v", err)
	}

	if controllerClient.registerRequest.NodeID != "relay-01" {
		t.Fatalf("expected relay name as initial node id, got %s", controllerClient.registerRequest.NodeID)
	}
	current := store.current()
	if current.NodeID != "node-1" || current.RelayID != "relay-1" || current.ZTNetworkID != "8056c2e21c000001" || current.ConfigVersion != 7 {
		t.Fatalf("unexpected state: %+v", current)
	}
}

func TestHeartbeatFetchesNewConfigAppliesAndReportsResult(t *testing.T) {
	store := &memoryStore{state: state.State{
		NodeID:        "node-1",
		RelayID:       "relay-1",
		ConfigVersion: 7,
		NFTApplied:    true,
		RouteApplied:  true,
	}}
	controllerClient := &fakeController{
		heartbeatResponse: controller.HeartbeatResponse{
			ConfigVersion: 8,
			HasNewConfig:  true,
		},
		configResponse: controller.RelayConfig{Version: 8},
	}
	metricsCollector := &fakeCollector{snapshot: collector.Snapshot{
		Hostname: "relay-01",
		Load:     collector.LoadSnapshot{CPUPercent: 1},
	}}
	configReconciler := &fakeReconciler{
		outcome: reconciler.Outcome{
			Version:       8,
			Applied:       true,
			Message:       "applied successfully",
			ChangedRoutes: []string{"ip route replace 10.20.0.0/24"},
			ChangedRules:  []string{"nft -f -"},
		},
	}

	svc := New(Config{HeartbeatInterval: time.Hour}, controllerClient, metricsCollector, store, configReconciler, nil)

	next, err := svc.heartbeat(context.Background(), store.state)
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	if controllerClient.heartbeatRequest.Runtime.ConfigVersion != 7 {
		t.Fatalf("unexpected heartbeat runtime: %+v", controllerClient.heartbeatRequest.Runtime)
	}
	if controllerClient.getConfigNodeID != "node-1" {
		t.Fatalf("expected get config for node-1, got %s", controllerClient.getConfigNodeID)
	}
	if configReconciler.config.Version != 8 {
		t.Fatalf("expected reconciler to receive config version 8, got %d", configReconciler.config.Version)
	}
	if next.ConfigVersion != 8 || !next.NFTApplied || !next.RouteApplied {
		t.Fatalf("unexpected state after apply: %+v", next)
	}
	if !controllerClient.applyResult.Success || controllerClient.applyResult.Version != 8 {
		t.Fatalf("unexpected apply result report: %+v", controllerClient.applyResult)
	}
}

func TestHeartbeatReportsReconcileFailureAndKeepsPendingState(t *testing.T) {
	store := &memoryStore{state: state.State{
		NodeID:        "node-1",
		RelayID:       "relay-1",
		ConfigVersion: 7,
		NFTApplied:    true,
		RouteApplied:  true,
	}}
	controllerClient := &fakeController{
		heartbeatResponse: controller.HeartbeatResponse{
			ConfigVersion: 8,
			HasNewConfig:  true,
		},
		configResponse: controller.RelayConfig{Version: 8},
	}
	metricsCollector := &fakeCollector{snapshot: collector.Snapshot{Hostname: "relay-01"}}
	configReconciler := &fakeReconciler{
		outcome: reconciler.Outcome{
			Version: 8,
			Message: "apply failed",
		},
		err: errors.New("forced reconcile failure"),
	}

	svc := New(Config{HeartbeatInterval: time.Hour}, controllerClient, metricsCollector, store, configReconciler, nil)

	next, err := svc.heartbeat(context.Background(), store.state)
	if err != nil {
		t.Fatalf("heartbeat should keep running after reconcile failure: %v", err)
	}
	if next.ConfigVersion != 8 || next.NFTApplied || next.RouteApplied {
		t.Fatalf("expected pending state after failure: %+v", next)
	}
	if controllerClient.applyResult.Success {
		t.Fatalf("expected failed apply report: %+v", controllerClient.applyResult)
	}
}

func TestHeartbeatRetriesPendingConfigEvenWithoutNewVersion(t *testing.T) {
	store := &memoryStore{state: state.State{
		NodeID:        "node-1",
		RelayID:       "relay-1",
		ConfigVersion: 8,
		NFTApplied:    false,
		RouteApplied:  false,
	}}
	controllerClient := &fakeController{
		heartbeatResponse: controller.HeartbeatResponse{
			ConfigVersion: 8,
			HasNewConfig:  false,
		},
		configResponse: controller.RelayConfig{Version: 8},
	}
	metricsCollector := &fakeCollector{snapshot: collector.Snapshot{Hostname: "relay-01"}}
	configReconciler := &fakeReconciler{
		outcome: reconciler.Outcome{Version: 8, Applied: true, Message: "applied successfully"},
	}

	svc := New(Config{HeartbeatInterval: time.Hour}, controllerClient, metricsCollector, store, configReconciler, nil)

	next, err := svc.heartbeat(context.Background(), store.state)
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if controllerClient.getConfigNodeID != "node-1" {
		t.Fatalf("expected pending config retry, got get config node %q", controllerClient.getConfigNodeID)
	}
	if !next.NFTApplied || !next.RouteApplied {
		t.Fatalf("expected pending config to be applied: %+v", next)
	}
}

type fakeController struct {
	registerRequest   controller.RegisterRequest
	registerResponse  controller.RegisterResponse
	heartbeatRequest  controller.HeartbeatRequest
	heartbeatResponse controller.HeartbeatResponse
	getConfigNodeID   string
	configResponse    controller.RelayConfig
	applyResult       controller.ApplyResultRequest
}

func (fake *fakeController) Register(ctx context.Context, request controller.RegisterRequest) (controller.RegisterResponse, error) {
	fake.registerRequest = request
	return fake.registerResponse, nil
}

func (fake *fakeController) Heartbeat(ctx context.Context, nodeID string, request controller.HeartbeatRequest) (controller.HeartbeatResponse, error) {
	fake.heartbeatRequest = request
	return fake.heartbeatResponse, nil
}

func (fake *fakeController) GetConfig(ctx context.Context, nodeID string) (controller.RelayConfig, error) {
	fake.getConfigNodeID = nodeID
	return fake.configResponse, nil
}

func (fake *fakeController) ReportApplyResult(ctx context.Context, nodeID string, request controller.ApplyResultRequest) (controller.ApplyResultResponse, error) {
	fake.applyResult = request
	return controller.ApplyResultResponse{Accepted: true}, nil
}

type fakeReconciler struct {
	config  controller.RelayConfig
	outcome reconciler.Outcome
	err     error
}

func (fake *fakeReconciler) Apply(ctx context.Context, current state.State, config controller.RelayConfig) (state.State, reconciler.Outcome, error) {
	fake.config = config
	if fake.err != nil {
		return current, fake.outcome, fake.err
	}
	current.NFTApplied = true
	current.RouteApplied = true
	current.LastApplyMessage = fake.outcome.Message
	return current, fake.outcome, nil
}

type fakeCollector struct {
	snapshot collector.Snapshot
}

func (fake *fakeCollector) Snapshot(ctx context.Context) (collector.Snapshot, error) {
	return fake.snapshot, nil
}

type memoryStore struct {
	state StateAlias
	saves int
}

type StateAlias = state.State

func (store *memoryStore) Load() (state.State, error) {
	return state.State(store.state), nil
}

func (store *memoryStore) Save(current state.State) error {
	store.state = StateAlias(current)
	store.saves++
	return nil
}

func (store *memoryStore) current() state.State {
	return state.State(store.state)
}

func (store *memoryStore) saveCount() int {
	return store.saves
}
