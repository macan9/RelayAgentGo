# RelayAgentGo 运维手册

## 1. 运行前检查

在 relay 节点确认以下依赖可用：

- `zerotier-one`
- `ip`（iproute2）
- `nft`
- `sysctl`

建议命令：

```bash
which zerotier-cli ip nft sysctl
```

## 2. 配置文件

示例模板：

```bash
cp deploy/relay-agent.env.example relay-agent.env
```

关键配置：

- `CONTROLLER_BASE_URL`
- `CONTROLLER_TOKEN`
- `RELAY_NAME`
- `STATE_PATH`
- `DRY_RUN`

首次联调建议：

- `DRY_RUN=true`（只上报和生成命令，不改本机网络）

## 3. 本地启动与测试

```bash
RELAY_AGENT_CONFIG=relay-agent.env go run ./cmd/relay-agent
go test ./...
```

## 4. systemd 部署

安装二进制后使用：

```bash
sudo cp deploy/relay-agent.service /etc/systemd/system/relay-agent.service
sudo mkdir -p /etc/relay-agent
sudo cp relay-agent.env /etc/relay-agent/relay-agent.env
sudo systemctl daemon-reload
sudo systemctl enable --now relay-agent
```

查看状态与日志：

```bash
systemctl status relay-agent
journalctl -u relay-agent -f
```

## 5. 联调检查清单

1. 启动后能看到 `relay registered` 日志。  
2. 每个心跳周期有上报日志，无持续 `heartbeat failed`。  
3. 控制器配置版本提升后，agent 能拉取配置并上报应用结果。  
4. `DRY_RUN=false` 时，检查：

```bash
sysctl net.ipv4.ip_forward
ip route
nft list ruleset
```

## 6. 常见故障

### 控制器 401/403

- 检查 `CONTROLLER_TOKEN` 是否正确。
- 检查控制器是否按 `Authorization: Bearer <token>` 校验。

### 心跳反复失败

- 检查 `CONTROLLER_BASE_URL` 连通性和 TLS 证书。
- 观察 `HTTP_TIMEOUT_SECONDS` 与 `CONTROLLER_RETRY_*` 是否过小。

### 配置应用失败

- 先用 `DRY_RUN=true` 重放，确认命令生成是否符合预期。
- 核对路由 CIDR、NAT 类型、`snat` 的 `toAddress` 是否完整。

### 重启后状态丢失

- 检查 `STATE_PATH` 路径权限。
- 确认运行用户有写权限（systemd 默认 root）。

## 7. 升级建议

1. 先灰度节点升级，并保持 `DRY_RUN=true` 验证心跳与配置拉取。  
2. 观测 10-30 分钟无异常后，切换 `DRY_RUN=false`。  
3. 分批升级其余节点，保留回滚版本二进制。  
