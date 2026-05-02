package collector

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Config struct {
	ZTInterfacePrefix string
	PublicIPProbeURL  string
	LatencyProbeURL   string
}

type Collector struct {
	config     Config
	httpClient *http.Client
	sysfsRoot  string
	procRoot   string
	mu         sync.Mutex
	lastCPU    cpuSample
	hasLastCPU bool
	hostnameFn func() (string, error)
	ifaceFn    func() ([]net.Interface, error)
	addrsFn    func(*net.Interface) ([]net.Addr, error)
}

type Option func(*Collector)

func New(config Config, options ...Option) *Collector {
	collector := &Collector{
		config: config,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		sysfsRoot:  "/sys",
		procRoot:   "/proc",
		hostnameFn: os.Hostname,
		ifaceFn:    net.Interfaces,
		addrsFn: func(iface *net.Interface) ([]net.Addr, error) {
			return iface.Addrs()
		},
	}

	for _, option := range options {
		option(collector)
	}

	return collector
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(collector *Collector) {
		if httpClient != nil {
			collector.httpClient = httpClient
		}
	}
}

func WithRoots(sysfsRoot string, procRoot string) Option {
	return func(collector *Collector) {
		if sysfsRoot != "" {
			collector.sysfsRoot = sysfsRoot
		}
		if procRoot != "" {
			collector.procRoot = procRoot
		}
	}
}

func WithHostAndInterfaces(
	hostnameFn func() (string, error),
	ifaceFn func() ([]net.Interface, error),
	addrsFn func(*net.Interface) ([]net.Addr, error),
) Option {
	return func(collector *Collector) {
		if hostnameFn != nil {
			collector.hostnameFn = hostnameFn
		}
		if ifaceFn != nil {
			collector.ifaceFn = ifaceFn
		}
		if addrsFn != nil {
			collector.addrsFn = addrsFn
		}
	}
}

func (collector *Collector) Snapshot(ctx context.Context) (Snapshot, error) {
	hostname, err := collector.hostnameFn()
	if err != nil {
		return Snapshot{}, fmt.Errorf("get hostname: %w", err)
	}

	ztInterfaces, ztIPs, err := collector.zeroTierInterfaces()
	if err != nil {
		return Snapshot{}, err
	}

	publicIP, err := collector.publicIP(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	load, err := collector.load()
	if err != nil {
		return Snapshot{}, err
	}

	network, err := collector.network(ctx, ztInterfaces)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		Hostname: hostname,
		PublicIP: publicIP,
		ZTIPs:    ztIPs,
		Load:     load,
		Network:  network,
	}, nil
}

func (collector *Collector) zeroTierInterfaces() ([]string, []string, error) {
	interfaces, err := collector.ifaceFn()
	if err != nil {
		return nil, nil, fmt.Errorf("list interfaces: %w", err)
	}

	var names []string
	var ips []string
	for index := range interfaces {
		iface := interfaces[index]
		if !strings.HasPrefix(iface.Name, collector.config.ZTInterfacePrefix) {
			continue
		}
		names = append(names, iface.Name)

		addrs, err := collector.addrsFn(&iface)
		if err != nil {
			return nil, nil, fmt.Errorf("list addresses for %s: %w", iface.Name, err)
		}
		for _, addr := range addrs {
			ip := addrIP(addr)
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			ips = append(ips, ip.String())
		}
	}

	return names, ips, nil
}

func (collector *Collector) publicIP(ctx context.Context) (string, error) {
	if collector.config.PublicIPProbeURL == "" {
		return "", nil
	}

	body, err := collector.fetch(ctx, http.MethodGet, collector.config.PublicIPProbeURL)
	if err != nil {
		return "", fmt.Errorf("probe public ip: %w", err)
	}

	value := strings.TrimSpace(string(body))
	ip := net.ParseIP(value)
	if ip == nil {
		return "", fmt.Errorf("probe public ip: invalid IP %q", value)
	}

	return ip.String(), nil
}

func (collector *Collector) load() (LoadSnapshot, error) {
	collector.mu.Lock()
	defer collector.mu.Unlock()

	load1, err := readLoad1(collector.procRoot)
	if err != nil {
		load1 = 0
	}

	cpuPercent := 0.0
	cpu, err := readCPUSample(collector.procRoot)
	if err == nil {
		if collector.hasLastCPU {
			cpuPercent = cpu.percentSince(collector.lastCPU)
		}
		collector.lastCPU = cpu
		collector.hasLastCPU = true
	}

	memoryPercent, err := readMemoryPercent(collector.procRoot)
	if err != nil {
		memoryPercent = 0
	}

	return LoadSnapshot{
		CPUPercent:    cpuPercent,
		MemoryPercent: memoryPercent,
		Load1:         load1,
	}, nil
}

func (collector *Collector) network(ctx context.Context, interfaces []string) (NetworkSnapshot, error) {
	rxBytes, txBytes, err := readNetworkBytes(collector.sysfsRoot, interfaces)
	if err != nil {
		rxBytes = 0
		txBytes = 0
	}

	latencyMS, err := collector.latency(ctx)
	if err != nil {
		latencyMS = -1
	}

	return NetworkSnapshot{
		RXBytes:   rxBytes,
		TXBytes:   txBytes,
		LatencyMS: latencyMS,
	}, nil
}

func (collector *Collector) latency(ctx context.Context) (int64, error) {
	if collector.config.LatencyProbeURL == "" {
		return -1, nil
	}

	startedAt := time.Now()
	if _, err := collector.fetch(ctx, http.MethodHead, collector.config.LatencyProbeURL); err != nil {
		return -1, err
	}
	return time.Since(startedAt).Milliseconds(), nil
}

func (collector *Collector) fetch(ctx context.Context, method string, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "RelayAgentGo")

	resp, err := collector.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 1024))
}

func addrIP(addr net.Addr) net.IP {
	switch value := addr.(type) {
	case *net.IPNet:
		return value.IP
	case *net.IPAddr:
		return value.IP
	default:
		return nil
	}
}
