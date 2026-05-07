# 第 6 阶段开工清单（OwnZeroTierController）

更新时间：2026-05-07

## 目标

在 `OwnZeroTierController` 完成 relay 管理闭环，使当前 `RelayAgentGo` 客户端可直接对接真实接口：

- `POST /api/relays/register`
- `POST /api/relays/{nodeId}/heartbeat`
- `GET /api/relays/{nodeId}/config`
- `POST /api/relays/{nodeId}/config-apply-result`
- `GET /api/relays`
- `GET /api/relays/{nodeId}`
- `PATCH /api/relays/{nodeId}/config`（管理侧）

## 建议数据模型

## `relays`

建议字段（可按现有风格调整命名）：

- `id`（relayId，主键）
- `node_id`（唯一索引）
- `hostname`
- `zt_network_id`
- `public_ip`
- `zt_ips_json`
- `version`
- `labels_json`
- `config_version`
- `nft_applied`
- `route_applied`
- `last_apply_message`
- `last_register_at`
- `last_heartbeat_at`
- `last_seen_at`
- `load_json`
- `network_json`
- `updated_at`
- `created_at`

## `relay_configs`

- `id`
- `node_id`（索引）
- `version`（同一 node_id 下唯一）
- `config_json`
- `created_by`
- `created_at`

## 关键约定

1. 首次注册自动为该 relay 生成默认配置：`net.ipv4.ip_forward=1`。  
2. 心跳时返回：
   - `configVersion`（控制器当前版本）
   - `hasNewConfig`（对比 agent runtime 结果）  
3. `PATCH /config` 若请求版本 `<= 当前版本`，控制器自动递增版本。  
4. 应用结果接口保存最近一次结果，并写审计日志。  
5. 所有 relay 接口沿用 Bearer Token 认证。  

## 实施顺序（建议）

1. 数据库迁移：`relays`、`relay_configs`。  
2. store/repository 层：增查改接口。  
3. service 层：注册、心跳判新、配置读写、应用结果落盘。  
4. handler 层：实现 `/api/relays/*`。  
5. 审计日志：注册、配置变更、应用结果。  
6. API 文档与测试：handler 单测 + service 单测 + 端到端 smoke。  

## 验收标准

1. `RelayAgentGo` 在 `DRY_RUN=true` 下可完成 register + heartbeat + get-config + report-result 全链路。  
2. 管理侧更新配置后，下一次心跳返回 `hasNewConfig=true`。  
3. agent 上报 apply 结果后，控制器查询接口可看到最近结果。  
4. `go test ./...` 全部通过。  

## 与 RelayAgentGo 的当前契约提醒

- `RegisterResponse` 需要返回：`relayId`、`nodeId`、`configVersion`，建议附带 `ztNetworkId`。  
- `HeartbeatResponse` 需要返回：`configVersion`、`hasNewConfig`。  
- `GetConfig` 返回体必须包含 `version`，且 `version > 0`。  
