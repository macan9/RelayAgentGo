package collector

import "relay-agent-go/internal/controller"

func (snapshot Snapshot) RegisterRequest(ztNetworkID string, version string, labels map[string]string) controller.RegisterRequest {
	return controller.RegisterRequest{
		Hostname:    snapshot.Hostname,
		ZTNetworkID: ztNetworkID,
		ZTIPs:       snapshot.ZTIPs,
		PublicIP:    snapshot.PublicIP,
		Version:     version,
		Labels:      labels,
	}
}

func (snapshot Snapshot) HeartbeatRequest(nodeID string, configVersion int64, nftApplied bool, routeApplied bool) controller.HeartbeatRequest {
	return controller.HeartbeatRequest{
		NodeID:   nodeID,
		PublicIP: snapshot.PublicIP,
		ZTIPs:    snapshot.ZTIPs,
		Load: controller.LoadSnapshot{
			CPUPercent:    snapshot.Load.CPUPercent,
			MemoryPercent: snapshot.Load.MemoryPercent,
			Load1:         snapshot.Load.Load1,
		},
		Network: controller.NetworkSnapshot{
			RXBytes:   snapshot.Network.RXBytes,
			TXBytes:   snapshot.Network.TXBytes,
			LatencyMS: snapshot.Network.LatencyMS,
		},
		Runtime: controller.RuntimeSnapshot{
			ConfigVersion: configVersion,
			NFTApplied:    nftApplied,
			RouteApplied:  routeApplied,
		},
	}
}
