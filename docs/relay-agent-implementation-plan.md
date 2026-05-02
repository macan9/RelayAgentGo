# RelayAgentGo 技术方案与开发步骤

## 1. 目标定位

`RelayAgentGo` 是部署在 Linux 中继节点上的轻量级代理服务。它不直接实现数据转发逻辑，数据面交给 Linux 内核完成；它负责把本机状态注册并上报给 `OwnZeroTierController`，再接收控制器下发的路由与 NAT 策略，最终落地到本机 `nftables` 和 `ip route`。

整体链路：

```text
RelayAgentGo
  -> 注册到 OwnZeroTierController
  -> 上报公网 IP、ZeroTier IP、负载、延迟
  -> 接收路由/NAT 配置
  -> 修改本机 nftables / ip route
  -> 实际转发由 Linux 内核完成
```

## 2. 系统边界

### RelayAgentGo 负责

- 节点启动时注册 relay 节点身份。
- 周期性采集并上报公网 IP、ZeroTier IP、CPU、内存、负载、转发状态和延迟探测结果。
- 拉取或接收控制器下发的路由、NAT、转发表配置。
- 校验配置合法性，生成本机网络变更计划。
- 幂等修改 `nftables`、`ip route`、`sysctl`。
- 在配置失败时回滚或上报失败状态。
- 以 systemd 服务方式常驻运行。

### OwnZeroTierController 需要补充

- relay 节点注册接口。
- relay 节点心跳与状态上报接口。
- relay 配置查询或下发接口。
- relay 节点健康状态、最近上报时间、配置版本管理。
- relay 相关审计日志。

### Linux 内核负责

- 实际 L3 转发。
- SNAT / DNAT / MASQUERADE。
- 路由查表和转发。
- nftables 规则匹配与执行。

### 不在第一版做

- 自研转发协议。
- 用户态流量代理。
- 多控制器高可用。
- 复杂流量调度算法。
- Web 管理后台。

## 3. 推荐架构

```text
+---------------------------+
| OwnZeroTierController     |
| - relay registry          |
| - config version          |
| - health/status           |
+-------------+-------------+
              ^
              | HTTPS + Bearer Token / mTLS
              v
+-------------+-------------+
| RelayAgentGo              |
| - register                |
| - heartbeat               |
| - metrics collector       |
| - config reconciler       |
| - nftables manager        |
| - route manager           |
+-------------+-------------+
              |
              v
+-------------+-------------+
| Linux Kernel              |
| - ip_forward              |
| - ip route                |
| - nftables nat/filter     |
+---------------------------+
```

第一版建议采用 agent 主动轮询控制器的方式：

- 实现简单，穿透 NAT 和防火墙更容易。
- 控制器不需要主动连接 relay 节点。
- 后续可升级为 SSE、WebSocket 或 gRPC stream。

## 4. 核心技术选型

- 语言：Go。
- HTTP 客户端：标准库 `net/http`。
- 配置：环境变量 + YAML 文件。
- 日志：`zap` 或 `slog`，第一版可用 `slog`。
- 本机命令执行：封装 `ip`、`nft`、`sysctl` 命令。
- 配置落盘：`/var/lib/relay-agent/state.json`。
- 运行方式：systemd。
- Linux 依赖：`iproute2`、`nftables`、`zerotier-one`。

## 5. 关键数据模型

### Relay 节点注册信息

```json
{
  "nodeId": "zt-node-id-or-agent-generated-id",
  "hostname": "relay-hk-01",
  "ztNetworkId": "8056c2e21c000001",
  "ztIp": "10.147.17.20",
  "publicIp": "1.2.3.4",
  "version": "0.1.0",
  "labels": {
    "region": "hk",
    "isp": "cmcc"
  }
}
```

### 心跳状态

```json
{
  "nodeId": "relay-hk-01",
  "publicIp": "1.2.3.4",
  "ztIps": ["10.147.17.20"],
  "load": {
    "cpuPercent": 12.3,
    "memoryPercent": 41.6,
    "load1": 0.35
  },
  "network": {
    "rxBytes": 123456,
    "txBytes": 654321,
    "latencyMs": 38
  },
  "runtime": {
    "configVersion": 12,
    "nftApplied": true,
    "routeApplied": true
  }
}
```

### 下发配置

```json
{
  "version": 12,
  "sysctl": {
    "net.ipv4.ip_forward": "1"
  },
  "routes": [
    {
      "dst": "10.20.0.0/24",
      "via": "10.147.17.1",
      "dev": "ztxxxxxx",
      "metric": 100
    }
  ],
  "nat": [
    {
      "name": "zt-to-public-masquerade",
      "type": "masquerade",
      "family": "ip",
      "src": "10.147.17.0/24",
      "outInterface": "eth0"
    }
  ]
}
```

## 6. 控制器接口建议

当前 `OwnZeroTierController` 已有 `/api/networks`、成员授权、同步和审计能力，但还缺 relay-agent 专用接口。建议新增：

```text
POST /api/relays/register
POST /api/relays/{nodeId}/heartbeat
GET  /api/relays/{nodeId}/config
POST /api/relays/{nodeId}/config-apply-result
GET  /api/relays
GET  /api/relays/{nodeId}
```

### 注册接口

- 用途：agent 首次启动或身份变化时注册。
- 输入：节点 ID、主机名、公网 IP、ZeroTier IP、版本、标签。
- 输出：控制器分配的 relay ID、当前配置版本、心跳间隔。

### 心跳接口

- 用途：周期性上报运行状态。
- 建议周期：10 到 30 秒。
- 输出：是否有新配置、最新配置版本。

### 配置拉取接口

- 用途：agent 发现配置版本变化后主动拉取。
- 要求：配置必须带 `version`，agent 只应用更高版本。

### 配置应用结果接口

- 用途：agent 上报配置是否成功落地。
- 字段：`version`、`success`、`message`、`changedRoutes`、`changedRules`。

## 7. RelayAgentGo 模块拆分

```text
cmd/relay-agent/
  main.go
internal/config/
  读取 env 和 YAML 配置
internal/controller/
  OwnZeroTierController HTTP client
internal/identity/
  获取/生成 agent node id
internal/collector/
  采集公网 IP、ZeroTier IP、负载、延迟
internal/reconciler/
  对比目标配置和当前状态，生成应用计划
internal/netops/
  封装 ip route、nft、sysctl
internal/state/
  本地状态落盘，保存配置版本和上次应用结果
internal/service/
  agent 主循环、注册、心跳、配置拉取
deploy/
  systemd unit、安装脚本
docs/
  方案和运维文档
```

## 8. 本机网络配置方案

### sysctl

第一版必须确保：

```text
net.ipv4.ip_forward=1
```

如后续支持 IPv6，再增加：

```text
net.ipv6.conf.all.forwarding=1
```

### ip route

所有路由变更必须幂等：

```bash
ip route replace <dst> via <via> dev <dev> metric <metric>
ip route del <dst> via <via> dev <dev>
```

建议第一版只管理带有 agent 标记的路由，避免误删用户已有规则。

### nftables

建议创建独立 table 和 chain：

```text
table ip relay_agent
chain postrouting {
  type nat hook postrouting priority srcnat; policy accept;
}
```

规则命名和注释统一带 `relay-agent` 前缀。agent 只修改自己的 table，不碰系统其他 nftables 规则。

## 9. 配置应用流程

```text
1. 心跳发现 controllerConfigVersion > localConfigVersion
2. 拉取最新配置
3. 校验配置字段
4. 采集本机当前 route/nft/sysctl 状态
5. 生成变更计划
6. 先应用 sysctl
7. 再应用 nftables
8. 最后应用 ip route
9. 保存本地 state
10. 上报应用结果
```

失败处理：

- 配置校验失败：不应用，直接上报失败。
- `nft` 应用失败：不继续执行路由变更。
- `ip route` 部分失败：记录失败项，并上报控制器。
- 第一版不做复杂回滚，但必须保证下一轮 reconcile 可重试。

## 10. 安全设计

- 控制器 API 必须使用 HTTPS。
- 第一版可使用静态 Bearer Token。
- 生产建议升级为 mTLS 或 agent 独立 token。
- agent 配置文件权限建议为 `0600`。
- 只允许控制器下发白名单网段内的路由和 NAT。
- 禁止下发 `0.0.0.0/0` 默认路由，除非显式开启 `ALLOW_DEFAULT_ROUTE=true`。
- 命令执行必须使用参数数组，不拼接 shell 字符串。

## 11. 开发步骤分类

### 第 0 阶段：项目骨架

- 初始化 Go module。
- 建立目录结构。
- 增加配置加载、日志、版本号。
- 增加 `README.md` 和 systemd 草稿。

交付物：

- `go.mod`
- `cmd/relay-agent/main.go`
- `internal/config`
- `deploy/relay-agent.service`

阶段 0 约定：

- 模块名：`relay-agent-go`。
- 配置优先级：环境变量优先，`RELAY_AGENT_CONFIG` 指定的 env 文件作为默认值来源。
- 默认配置文件：`/etc/relay-agent/relay-agent.env`。
- 配置文件格式：每行一个 `KEY=VALUE`，支持 `#` 注释和空行。
- 第一版入口只负责加载配置、初始化 JSON 日志、打印启动摘要并响应 `SIGTERM` / `Ctrl+C`。
- 真实注册、心跳和网络配置能力从第 1 阶段开始接入。

阶段 0 必填配置：

```env
CONTROLLER_BASE_URL=https://controller.example.com
CONTROLLER_TOKEN=change-me
ZT_NETWORK_ID=8056c2e21c000001
RELAY_NAME=relay-01
```

阶段 0 可选配置：

```env
LOG_LEVEL=info
STATE_PATH=/var/lib/relay-agent/state.json
HEARTBEAT_INTERVAL_SECONDS=30
HTTP_TIMEOUT_SECONDS=10
DRY_RUN=false
```

阶段 0 验收命令：

```bash
go test ./...
go run ./cmd/relay-agent
```

本地运行时可以先复制 `deploy/relay-agent.env.example`，再通过 `RELAY_AGENT_CONFIG` 指向该文件。

### 第 1 阶段：控制器联通

- 实现 `controller.Client`。
- 支持 register、heartbeat、get config、report apply result。
- 支持 Bearer Token。
- 增加超时、重试和错误日志。

交付物：

- `internal/controller`
- 控制器接口 mock 测试。

### 第 2 阶段：本机信息采集

- 获取 hostname。
- 获取公网 IP。
- 获取 ZeroTier interface 和 IP。
- 采集 CPU、内存、load。
- 对控制器或指定探测地址测延迟。

交付物：

- `internal/collector`
- 心跳 payload 单元测试。

### 第 3 阶段：注册与心跳主循环

- 启动时注册。
- 周期性心跳。
- 根据控制器返回判断是否需要拉配置。
- 本地保存 nodeId、configVersion 和最近应用状态。

交付物：

- `internal/service`
- `internal/state`

### 第 4 阶段：网络操作封装

- 封装 `sysctl`。
- 封装 `ip route list/replace/del`。
- 封装 `nft list/apply`。
- 所有命令支持 dry-run。

交付物：

- `internal/netops`
- dry-run 测试。

### 第 5 阶段：配置 reconcile

- 定义目标配置结构。
- 校验路由、NAT、接口名和网段。
- 对比目标配置和当前状态。
- 生成变更计划。
- 按顺序应用 sysctl、nftables、route。

交付物：

- `internal/reconciler`
- 配置校验测试。
- 应用计划测试。

### 第 6 阶段：控制器侧改造

- 在 `OwnZeroTierController` 增加 relays 表。
- 增加 relay 心跳表或状态字段。
- 增加 relay 配置表，支持版本号。
- 新增 `/api/relays/*` 接口。
- relay 注册、心跳、配置变更写审计日志。

交付物：

- 数据库迁移 SQL。
- relay HTTP handler。
- relay service/store。
- API 文档更新。

### 第 7 阶段：集成联调

- 本机启动 `zerotier-one`。
- controller 和 relay-agent 同机或两台机器部署。
- 验证 agent 注册。
- 验证状态上报。
- 验证配置版本变化后 agent 自动拉取。
- 验证 `nft list ruleset` 和 `ip route` 生效。
- 验证 Linux 内核实际转发。

交付物：

- 联调记录。
- 最小可运行配置样例。

### 第 8 阶段：部署和运维

- systemd unit。
- 安装脚本。
- 日志轮转建议。
- 健康检查命令。
- 升级流程。
- 故障排查文档。

交付物：

- `deploy/install.sh`
- `deploy/relay-agent.service`
- `docs/operations.md`

## 12. MVP 验收标准

- agent 能成功注册到 `OwnZeroTierController`。
- 控制器能看到 relay 节点在线状态。
- agent 能周期性上报公网 IP、ZeroTier IP、负载和延迟。
- 控制器修改配置版本后，agent 能拉取并应用。
- `net.ipv4.ip_forward=1` 自动生效。
- 指定路由出现在 `ip route`。
- 指定 NAT 规则出现在 `nft list ruleset`。
- 转发流量实际由 Linux 内核完成。
- agent 重启后不会重复创建冲突规则。
- 配置应用失败时，控制器能看到失败原因。

## 13. 推荐实施顺序

优先做 agent 自身闭环，再改控制器：

```text
1. RelayAgentGo 项目骨架
2. 本机 collector
3. netops dry-run
4. reconciler
5. controller client mock
6. OwnZeroTierController relay API
7. agent 对接真实 controller
8. 单机 nftables/ip route 联调
9. 多节点转发验证
10. systemd 部署
```

这样可以先用 mock 配置验证本机网络落地能力，再接入控制器，降低联调复杂度。
