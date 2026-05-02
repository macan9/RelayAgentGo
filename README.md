# RelayAgentGo

`RelayAgentGo` 是面向 Linux 中继节点的 ZeroTier relay agent。它负责向 `OwnZeroTierController` 注册、上报本机状态、接收路由/NAT 配置，并把配置落地到本机 `nftables` 和 `ip route`；实际流量转发由 Linux 内核完成。

## 当前文档

- [RelayAgentGo 技术方案与开发步骤](docs/relay-agent-implementation-plan.md)
- [中继知识学习](docs/中继知识学习.md)

## 核心链路

```text
RelayAgentGo
  -> 注册到 OwnZeroTierController
  -> 上报公网 IP、ZeroTier IP、负载、延迟
  -> 接收路由/NAT 配置
  -> 修改本机 nftables / ip route
  -> 实际转发由 Linux 内核完成
```

## 下一步

建议先按文档的第 0 到第 5 阶段实现 agent 本体闭环，再改造 `OwnZeroTierController` 增加 relay 注册、心跳和配置下发接口。

## 本地运行骨架

先准备配置文件：

```bash
cp deploy/relay-agent.env.example relay-agent.env
```

运行：

```bash
RELAY_AGENT_CONFIG=relay-agent.env go run ./cmd/relay-agent
```

测试：

```bash
go test ./...
```

当前入口会初始化 `OwnZeroTierController` 客户端，支持 Bearer Token、HTTP 超时和重试配置；真实注册和心跳主循环会在后续阶段接入。

第 2 阶段已加入本机采集器，能采集 hostname、ZeroTier 网卡 IP、公网 IP 探测、load、内存、CPU 增量、网卡收发字节和延迟探测；后续心跳主循环会复用这些采集结果。

第 3 阶段已接入注册和心跳主循环：启动后会向控制器注册，周期性上报状态，并把 `nodeId`、`relayId`、配置版本和最近状态保存到 `STATE_PATH`。
