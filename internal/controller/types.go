package controller

type RegisterRequest struct {
	NodeID      string            `json:"nodeId,omitempty"`
	Hostname    string            `json:"hostname"`
	ZTNetworkID string            `json:"ztNetworkId"`
	ZTIPs       []string          `json:"ztIps,omitempty"`
	PublicIP    string            `json:"publicIp,omitempty"`
	Version     string            `json:"version"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type RegisterResponse struct {
	RelayID                  string `json:"relayId"`
	NodeID                   string `json:"nodeId"`
	ConfigVersion            int64  `json:"configVersion"`
	HeartbeatIntervalSeconds int    `json:"heartbeatIntervalSeconds"`
}

type HeartbeatRequest struct {
	NodeID   string          `json:"nodeId"`
	PublicIP string          `json:"publicIp,omitempty"`
	ZTIPs    []string        `json:"ztIps,omitempty"`
	Load     LoadSnapshot    `json:"load"`
	Network  NetworkSnapshot `json:"network"`
	Runtime  RuntimeSnapshot `json:"runtime"`
}

type LoadSnapshot struct {
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memoryPercent"`
	Load1         float64 `json:"load1"`
}

type NetworkSnapshot struct {
	RXBytes   uint64 `json:"rxBytes"`
	TXBytes   uint64 `json:"txBytes"`
	LatencyMS int64  `json:"latencyMs"`
}

type RuntimeSnapshot struct {
	ConfigVersion int64 `json:"configVersion"`
	NFTApplied    bool  `json:"nftApplied"`
	RouteApplied  bool  `json:"routeApplied"`
}

type HeartbeatResponse struct {
	ConfigVersion int64 `json:"configVersion"`
	HasNewConfig  bool  `json:"hasNewConfig"`
}

type RelayConfig struct {
	Version int64             `json:"version"`
	Sysctl  map[string]string `json:"sysctl,omitempty"`
	Routes  []RouteConfig     `json:"routes,omitempty"`
	NAT     []NATConfig       `json:"nat,omitempty"`
}

type RouteConfig struct {
	Dst    string `json:"dst"`
	Via    string `json:"via,omitempty"`
	Dev    string `json:"dev,omitempty"`
	Metric int    `json:"metric,omitempty"`
}

type NATConfig struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Family       string `json:"family"`
	Table        string `json:"table,omitempty"`
	Src          string `json:"src,omitempty"`
	Dst          string `json:"dst,omitempty"`
	InInterface  string `json:"inInterface,omitempty"`
	OutInterface string `json:"outInterface,omitempty"`
	ToAddress    string `json:"toAddress,omitempty"`
	ToPort       string `json:"toPort,omitempty"`
}

type ApplyResultRequest struct {
	Version       int64    `json:"version"`
	Success       bool     `json:"success"`
	Message       string   `json:"message,omitempty"`
	ChangedRoutes []string `json:"changedRoutes,omitempty"`
	ChangedRules  []string `json:"changedRules,omitempty"`
}

type ApplyResultResponse struct {
	Accepted bool `json:"accepted"`
}
