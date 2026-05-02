package service

import (
	"context"
	"testing"
	"time"

	"relay-agent-go/internal/collector"
	"relay-agent-go/internal/controller"
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
			ConfigVersion: 7,
		},
	}
	metricsCollector := &fakeCollector{snapshot: collector.Snapshot{Hostname: "relay-01"}}

	svc := New(Config{
		ZTNetworkID:       "8056c2e21c000001",
		RelayName:         "relay-01",
		Version:           "0.1.0",
		HeartbeatInterval: time.Hour,
	}, controllerClient, metricsCollector, store, nil)

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
	if current.NodeID != "node-1" || current.RelayID != "relay-1" || current.ConfigVersion != 7 {
		t.Fatalf("unexpected state: %+v", current)
	}
}

func TestHeartbeatFetchesNewConfigAndMarksPendingApply(t *testing.T) {
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

	svc := New(Config{HeartbeatInterval: time.Hour}, controllerClient, metricsCollector, store, nil)

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
	if next.ConfigVersion != 8 || next.NFTApplied || next.RouteApplied {
		t.Fatalf("unexpected state after config fetch: %+v", next)
	}
}

type fakeController struct {
	registerRequest   controller.RegisterRequest
	registerResponse  controller.RegisterResponse
	heartbeatRequest  controller.HeartbeatRequest
	heartbeatResponse controller.HeartbeatResponse
	getConfigNodeID   string
	configResponse    controller.RelayConfig
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
