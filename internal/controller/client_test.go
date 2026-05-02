package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRegisterSendsBearerTokenAndDecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/api/relays/register" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		var request RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Hostname != "relay-01" {
			t.Fatalf("unexpected hostname: %s", request.Hostname)
		}

		writeJSON(t, w, RegisterResponse{
			RelayID:                  "relay-1",
			NodeID:                   "node-1",
			ConfigVersion:            7,
			HeartbeatIntervalSeconds: 30,
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", time.Second, WithRetry(0, 0))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	response, err := client.Register(context.Background(), RegisterRequest{
		Hostname:    "relay-01",
		ZTNetworkID: "8056c2e21c000001",
		Version:     "0.1.0",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if response.NodeID != "node-1" || response.ConfigVersion != 7 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestHeartbeatUsesEscapedNodeID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/relays/node%2F1/heartbeat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeJSON(t, w, HeartbeatResponse{
			ConfigVersion: 8,
			HasNewConfig:  true,
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", time.Second, WithRetry(0, 0))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	response, err := client.Heartbeat(context.Background(), "node/1", HeartbeatRequest{NodeID: "node/1"})
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if !response.HasNewConfig || response.ConfigVersion != 8 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestGetConfigDecodesRelayConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		writeJSON(t, w, RelayConfig{
			Version: 9,
			Sysctl: map[string]string{
				"net.ipv4.ip_forward": "1",
			},
			Routes: []RouteConfig{
				{Dst: "10.20.0.0/24", Via: "10.147.17.1", Dev: "ztxxxxxx", Metric: 100},
			},
			NAT: []NATConfig{
				{Name: "zt-to-public", Type: "masquerade", Family: "ip", Src: "10.147.17.0/24", OutInterface: "eth0"},
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", time.Second, WithRetry(0, 0))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	config, err := client.GetConfig(context.Background(), "node-1")
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if config.Version != 9 || len(config.Routes) != 1 || len(config.NAT) != 1 {
		t.Fatalf("unexpected config: %+v", config)
	}
}

func TestReportApplyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request ApplyResultRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Version != 12 || !request.Success {
			t.Fatalf("unexpected request: %+v", request)
		}
		writeJSON(t, w, ApplyResultResponse{Accepted: true})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", time.Second, WithRetry(0, 0))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	response, err := client.ReportApplyResult(context.Background(), "node-1", ApplyResultRequest{
		Version: 12,
		Success: true,
	})
	if err != nil {
		t.Fatalf("report apply result: %v", err)
	}
	if !response.Accepted {
		t.Fatal("expected accepted response")
	}
}

func TestRetryOnServerError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary failure", http.StatusBadGateway)
			return
		}
		writeJSON(t, w, RegisterResponse{NodeID: "node-1"})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", time.Second, WithRetry(1, 0))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if _, err := client.Register(context.Background(), RegisterRequest{Hostname: "relay-01"}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestDoesNotRetryBadRequest(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", time.Second, WithRetry(3, 0))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.Register(context.Background(), RegisterRequest{Hostname: "relay-01"})
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
