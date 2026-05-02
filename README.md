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
