package collector

type Snapshot struct {
	Hostname string
	PublicIP string
	ZTIPs    []string
	Load     LoadSnapshot
	Network  NetworkSnapshot
}

type LoadSnapshot struct {
	CPUPercent    float64
	MemoryPercent float64
	Load1         float64
}

type NetworkSnapshot struct {
	RXBytes   uint64
	TXBytes   uint64
	LatencyMS int64
}
