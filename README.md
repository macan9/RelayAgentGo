# RelayAgentGo

`RelayAgentGo` 是面向 Linux 中继节点的 ZeroTier relay agent。它负责向 `OwnZeroTierController` 注册、上报本机状态、接收路由/NAT 配置，并把配置落地到本机 `nftables` 和 `ip route`；实际流量转发由 Linux 内核完成。

## 当前文档

- [RelayAgentGo 技术方案与开发步骤](docs/relay-agent-implementation-plan.md)
- [中继知识学习](docs/中继知识学习.md)
- [项目进度看板](docs/progress.md)
- [运维手册](docs/operations.md)
- [第 6 阶段（Controller 改造）开工清单](docs/stage6-controller-kickoff.md)

## 核心链路

```text
RelayAgentGo
  -> 注册到 OwnZeroTierController
  -> 上报公网 IP、ZeroTier IP、负载、延迟
  -> 接收路由/NAT 配置
  -> 修改本机 nftables / ip route
  -> 实际转发由 Linux 内核完成
```

## 当前进度

当前已完成第 0 到第 5 阶段，`RelayAgentGo` 已具备：

- 注册/心跳主循环
- 本机指标采集
- 配置拉取与校验
- `sysctl -> nftables -> ip route` 的 reconcile 应用链路
- dry-run 联调能力

下一步进入第 6 阶段：在 `OwnZeroTierController` 落地 `/api/relays/*` 接口与数据模型，再进行第 7 阶段联调。

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

第 2 阶段完成：采集 hostname、ZeroTier 网卡 IP、公网 IP 探测、load、内存、CPU 增量、网卡收发字节和延迟探测。  
第 3 阶段完成：接入注册与心跳主循环，状态落盘到 `STATE_PATH`。  
第 4 阶段完成：`netops` 封装 `sysctl` / `ip route` / `nftables` 并支持 dry-run。  
第 5 阶段完成：`reconciler` 支持配置校验、顺序应用、结果上报和失败重试。

## 线上联调模板

已提供面向 `139.196.158.225:8080` 的配置模板：

```bash
cp deploy/relay-agent.online.env.example relay-agent.online.env
```

补充 `CONTROLLER_TOKEN`，按需调整 `RELAY_NAME` 后运行。`ZT_NETWORK_ID` 可以留空，由 controller 注册响应返回。

```bash
RELAY_AGENT_CONFIG=relay-agent.online.env go run ./cmd/relay-agent
```
