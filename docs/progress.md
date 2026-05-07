# RelayAgentGo 项目进度看板

更新时间：2026-05-07

## 阶段状态

| 阶段 | 名称 | 状态 | 说明 |
|---|---|---|---|
| 0 | 项目骨架 | 已完成 | `go.mod`、入口、配置、基础日志、systemd 草稿已具备 |
| 1 | 控制器联通 | 已完成 | `internal/controller` 已支持 register/heartbeat/get-config/report-result |
| 2 | 本机信息采集 | 已完成 | `internal/collector` 已覆盖主机、网卡、负载、延迟等 |
| 3 | 注册与心跳主循环 | 已完成 | `internal/service` + `internal/state` 已接入并落盘 |
| 4 | 网络操作封装 | 已完成 | `internal/netops` 已支持 `sysctl`/`ip route`/`nft` + dry-run |
| 5 | 配置 reconcile | 已完成 | 已实现校验与按 `sysctl -> nft -> route` 顺序应用 |
| 6 | 控制器侧改造 | 进行中 | 进入开工阶段，详见 `docs/stage6-controller-kickoff.md` |
| 7 | 集成联调 | 未开始 | 待第 6 阶段接口落地 |
| 8 | 部署与运维 | 进行中 | service 文件已有，`install.sh` 待补 |

## 当前结论

- Agent 主体闭环已经完成，可在 dry-run 模式联调控制器。
- 真实线上闭环阻塞点在 `OwnZeroTierController` 的 relay 接口与数据模型。
- 文档与运维说明已补齐到可交接状态，后续重点转入第 6 阶段实现。
