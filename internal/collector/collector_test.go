package collector

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotCollectsHostNetworkLoadAndProbes(t *testing.T) {
	publicIPServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.2.3.4\n"))
	}))
	defer publicIPServer.Close()

	latencyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer latencyServer.Close()

	sysfsRoot, procRoot := testRoots(t)
	writeFile(t, filepath.Join(sysfsRoot, "class", "net", "ztabc", "statistics", "rx_bytes"), "123")
	writeFile(t, filepath.Join(sysfsRoot, "class", "net", "ztabc", "statistics", "tx_bytes"), "456")
	writeFile(t, filepath.Join(procRoot, "stat"), "cpu  100 0 100 800 0 0 0 0 0 0\n")
	writeFile(t, filepath.Join(procRoot, "loadavg"), "0.42 0.20 0.10 1/100 99\n")
	writeFile(t, filepath.Join(procRoot, "meminfo"), "MemTotal: 1000 kB\nMemAvailable: 250 kB\n")

	collector := New(Config{
		ZTInterfacePrefix: "zt",
		PublicIPProbeURL:  publicIPServer.URL,
		LatencyProbeURL:   latencyServer.URL,
	}, WithRoots(sysfsRoot, procRoot), WithHostAndInterfaces(
		func() (string, error) { return "relay-01", nil },
		func() ([]net.Interface, error) {
			return []net.Interface{{Name: "eth0"}, {Name: "ztabc"}}, nil
		},
		func(iface *net.Interface) ([]net.Addr, error) {
			if iface.Name != "ztabc" {
				return nil, nil
			}
			ip, network, err := net.ParseCIDR("10.147.17.20/24")
			if err != nil {
				return nil, err
			}
			network.IP = ip
			return []net.Addr{network}, nil
		},
	))

	snapshot, err := collector.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	if snapshot.Hostname != "relay-01" {
		t.Fatalf("unexpected hostname: %s", snapshot.Hostname)
	}
	if snapshot.PublicIP != "1.2.3.4" {
		t.Fatalf("unexpected public IP: %s", snapshot.PublicIP)
	}
	if len(snapshot.ZTIPs) != 1 || snapshot.ZTIPs[0] != "10.147.17.20" {
		t.Fatalf("unexpected ZeroTier IPs: %#v", snapshot.ZTIPs)
	}
	if snapshot.Network.RXBytes != 123 || snapshot.Network.TXBytes != 456 {
		t.Fatalf("unexpected network bytes: %+v", snapshot.Network)
	}
	if snapshot.Load.Load1 != 0.42 {
		t.Fatalf("unexpected load1: %v", snapshot.Load.Load1)
	}
	if snapshot.Load.MemoryPercent != 75 {
		t.Fatalf("unexpected memory percent: %v", snapshot.Load.MemoryPercent)
	}
	if snapshot.Network.LatencyMS < 0 {
		t.Fatalf("expected non-negative latency: %d", snapshot.Network.LatencyMS)
	}
}

func TestSnapshotAllowsMissingOptionalProbes(t *testing.T) {
	sysfsRoot, procRoot := testRoots(t)
	writeFile(t, filepath.Join(procRoot, "stat"), "cpu  100 0 100 800 0 0 0 0 0 0\n")
	writeFile(t, filepath.Join(procRoot, "loadavg"), "0.01 0.02 0.03 1/100 99\n")
	writeFile(t, filepath.Join(procRoot, "meminfo"), "MemTotal: 1000 kB\nMemAvailable: 1000 kB\n")

	collector := New(Config{ZTInterfacePrefix: "zt"}, WithRoots(sysfsRoot, procRoot), WithHostAndInterfaces(
		func() (string, error) { return "relay-01", nil },
		func() ([]net.Interface, error) { return []net.Interface{{Name: "ztabc"}}, nil },
		func(iface *net.Interface) ([]net.Addr, error) { return nil, nil },
	))

	snapshot, err := collector.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.PublicIP != "" {
		t.Fatalf("expected empty public IP, got %s", snapshot.PublicIP)
	}
	if snapshot.Network.LatencyMS != -1 {
		t.Fatalf("expected latency -1, got %d", snapshot.Network.LatencyMS)
	}
}

func TestCPUPercentUsesDeltaBetweenSamples(t *testing.T) {
	procRoot := t.TempDir()
	writeFile(t, filepath.Join(procRoot, "stat"), "cpu  100 0 100 800 0 0 0 0 0 0\n")

	first, err := readCPUSample(procRoot)
	if err != nil {
		t.Fatalf("read first cpu sample: %v", err)
	}

	writeFile(t, filepath.Join(procRoot, "stat"), "cpu  150 0 150 900 0 0 0 0 0 0\n")
	second, err := readCPUSample(procRoot)
	if err != nil {
		t.Fatalf("read second cpu sample: %v", err)
	}

	if got := second.percentSince(first); got != 50 {
		t.Fatalf("unexpected cpu percent: %v", got)
	}
}

func TestSnapshotRejectsInvalidPublicIP(t *testing.T) {
	publicIPServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-an-ip"))
	}))
	defer publicIPServer.Close()

	collector := New(Config{
		ZTInterfacePrefix: "zt",
		PublicIPProbeURL:  publicIPServer.URL,
	}, WithHostAndInterfaces(
		func() (string, error) { return "relay-01", nil },
		func() ([]net.Interface, error) { return nil, nil },
		nil,
	))

	if _, err := collector.Snapshot(context.Background()); err == nil {
		t.Fatal("expected invalid public IP error")
	}
}

func TestSnapshotBuildsControllerPayloads(t *testing.T) {
	snapshot := Snapshot{
		Hostname: "relay-01",
		PublicIP: "1.2.3.4",
		ZTIPs:    []string{"10.147.17.20"},
		Load:     LoadSnapshot{CPUPercent: 1, MemoryPercent: 2, Load1: 3},
		Network:  NetworkSnapshot{RXBytes: 4, TXBytes: 5, LatencyMS: 6},
	}

	register := snapshot.RegisterRequest("8056c2e21c000001", "0.1.0", map[string]string{"region": "hk"})
	if register.Hostname != "relay-01" || register.ZTNetworkID == "" || register.Version != "0.1.0" {
		t.Fatalf("unexpected register request: %+v", register)
	}

	heartbeat := snapshot.HeartbeatRequest("node-1", 12, true, false)
	if heartbeat.NodeID != "node-1" || heartbeat.Runtime.ConfigVersion != 12 || heartbeat.Network.RXBytes != 4 {
		t.Fatalf("unexpected heartbeat request: %+v", heartbeat)
	}
}

func testRoots(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	sysfsRoot := filepath.Join(root, "sys")
	procRoot := filepath.Join(root, "proc")
	if err := os.MkdirAll(sysfsRoot, 0o755); err != nil {
		t.Fatalf("mkdir sys root: %v", err)
	}
	if err := os.MkdirAll(procRoot, 0o755); err != nil {
		t.Fatalf("mkdir proc root: %v", err)
	}
	return sysfsRoot, procRoot
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestReadNetworkBytes(t *testing.T) {
	sysfsRoot := t.TempDir()
	writeFile(t, filepath.Join(sysfsRoot, "class", "net", "zt1", "statistics", "rx_bytes"), "100")
	writeFile(t, filepath.Join(sysfsRoot, "class", "net", "zt1", "statistics", "tx_bytes"), "200")
	writeFile(t, filepath.Join(sysfsRoot, "class", "net", "zt2", "statistics", "rx_bytes"), "300")
	writeFile(t, filepath.Join(sysfsRoot, "class", "net", "zt2", "statistics", "tx_bytes"), "400")

	rx, tx, err := readNetworkBytes(sysfsRoot, []string{"zt1", "zt2"})
	if err != nil {
		t.Fatalf("read network bytes: %v", err)
	}
	if rx != 400 || tx != 600 {
		t.Fatalf("unexpected bytes: rx=%d tx=%d", rx, tx)
	}
}

func TestAddrIPUnknownType(t *testing.T) {
	if ip := addrIP(dummyAddr("dummy")); ip != nil {
		t.Fatalf("expected nil ip, got %s", ip)
	}
}

type dummyAddr string

func (addr dummyAddr) Network() string {
	return string(addr)
}

func (addr dummyAddr) String() string {
	return fmt.Sprintf("%s", string(addr))
}
