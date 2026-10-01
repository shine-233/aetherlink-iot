# 国产化部署：数据库方言适配与 tp_to_db gRPC 对接（TP-20）

> 适用版本：批次三后端面（2026-09-25）。本文是国产数据库部署的对接章节，覆盖
> `grpc.tptodb_*` 外置遥测查询开关、`db.dialect` 方言声明与 `backend/internal/dialect`
> 方言适配层的使用口径。

## 1. 范围与边界

**已交付（本仓库内）**

- `backend/internal/dialect`：SQL 方言适配纯函数层（分页、元查询、配置解析、能力矩阵），零 DB 依赖、零驱动依赖，保持 `CGO_ENABLED=0` 纯 Go 交叉编译属性（配合 `backend/Makefile` 的 linux/amd64、arm64、loong64 目标）。
- `grpc.tptodb_type`（NONE/TSDB/KINGBASE/POLARDB）外置遥测查询开关：历史/当前/聚合遥测**读路径**经 tp_to_db gRPC 服务，写路径与保留清理仍走本地库。
- `db.dialect` 配置键：方言口径声明（见 `backend/configs/conf.example.yml`）。

**明确不做（residual，需外部环境）**

- TDengine/KingBase **原生驱动接入**与真实读写验证：需外部数据库环境，且为保持纯 Go 交叉编译不引入 CGO 驱动；tp_to_db 服务端本身不在本仓库。
- 元查询 SQL（`ins_columns`/`user_tab_columns` 等字段名）按 TDengine 3.x 与 Oracle 兼容字典的公开文档映射，接真实驱动后需以服务端实测校准。

## 2. 架构与数据流

```
                     ┌─ tptodb_type=NONE（默认）──────────────┐
 设备 → MQTT → 上行管线 → 写：本地 PostgreSQL（telemetry_datas）   │
                     │  读：本地 PostgreSQL（直连查询）          │
                     └─ tptodb_type=TSDB/KINGBASE/POLARDB ────┘
                        写：本地 PostgreSQL（不变）
                        读：tp_to_db gRPC 服务（外置国产库）
                        冷层降采样：自动停用
```

关键事实（代码依据）：

| 口径 | 位置 |
| --- | --- |
| 开关判定（NONE/TSDB/KINGBASE/POLARDB） | `backend/internal/app/grpc_service.go`（`externalTelemetryGRPCEnabled`）、`backend/internal/dal/telemetry_datas.go`（`usesTelemetryQueryClient`） |
| 走 gRPC 的读路径 | 当前值（`GetCurrentTelemetrDetailData`、`GetCurrentTelemetryDataEvolution`、`GetCurrentTelemetryReadiness`）、历史（`GetHistoryTelemetrData`、`GetHistoryTelemetrDataByPage`）、统计/聚合（`GetTelemetrStatisticDataWithLimit`、`GetTelemetrStatisticaAgregationData`） |
| 写路径不分支 | 批量写入、当前值 upsert、保留清理（`CreateTelemetrDataBatch`/`UpdateTelemetrDataBatch`/`DeleteTelemetrDataByTime`）恒走本地库 |
| 冷层降采样停用 | `backend/internal/dal/telemetry_rollups.go`（`TelemetryDownsamplingActive` = 非 gRPC 模式才激活） |
| 启动 fail-fast | `grpc.tptodb_server` 不可达时 `GrpcTptodbInit` 返回错误，`ServiceManager.StartAll` 回滚已启动服务并阻断启动（`backend/internal/app/service.go`）；拨号预算 10 秒（`third_party/grpc/tptodb_client/init.go`） |

## 3. 配置项参考

| 配置键 | 取值 | 说明 |
| --- | --- | --- |
| `grpc.tptodb_type` | `NONE`（默认）/`TSDB`/`KINGBASE`/`POLARDB` | 外置遥测查询开关；`NONE` = 全部走本地 PostgreSQL |
| `grpc.tptodb_server` | `host:port` | tp_to_db gRPC 服务地址；启用外置模式时必填，空值阻断启动 |
| `db.dialect` | `postgres`（默认）/`tdengine`/`kingbase`/`polardb` | 方言口径声明；显式配置优先于 `tptodb_type` 推导，配置非法值 fail closed |

`tptodb_type` → 方言映射（实现：`internal/dialect.FromTptodbType` / `Effective`）：

| tptodb_type | db.dialect 方言 | 分页语式 | 元查询体系 | 标识符引用符 |
| --- | --- | --- | --- | --- |
| `NONE`/未配置 | `postgres` | `LIMIT n OFFSET m` | `information_schema` | `"` |
| `TSDB` | `tdengine` | `LIMIT n OFFSET m`（TDengine 3.x，OFFSET 与 LIMIT 同现） | `INFORMATION_SCHEMA.ins_tables` / `ins_columns`（`db_name = DATABASE()`） | `` ` `` |
| `KINGBASE` | `kingbase`（Oracle 兼容模式） | 双层 `ROWNUM` 包裹 | `user_tables` / `user_tab_columns`（绑定参数大写归一） | `"` |
| `POLARDB` | `polardb`（MySQL 兼容版口径） | `LIMIT n OFFSET m` | `information_schema` | `` ` `` |

> POLARDB 按 MySQL 兼容版对待；若部署的是 PolarDB-PG 版，请在 `db.dialect` 显式填 `postgres` 覆盖推导。

## 4. tp_to_db gRPC 服务对接步骤

1. **部署服务端**：tp_to_db gRPC 服务不在本仓库，由部署环境提供（TDengine/KingBase/PolarDB 的查询代理）。确认其监听地址可从后端容器/主机访问（默认示例 `127.0.0.1:50052` 仅适合本机）。
2. **确认 gRPC 契约**：服务端需实现 `third_party/grpc/tptodb_client/grpc_tptodb` 的 AetherLink 服务五个方法——`GetDeviceAttributesCurrents`、`GetDeviceHistory`、`GetDeviceHistoryWithPageAndPage`、`GetDeviceKVDataWithNoAggregate`、`GetDeviceKVDataWithAggregate`（生成符号与 method path 属外部合约，见 `COMPATIBILITY.md`）。当前客户端使用 insecure 明文连接，服务端须部署在可信内网。
3. **配置后端**：`configs/conf.yml`（或环境变量 `GOTP_GRPC_TPTODB_SERVER`、`GOTP_GRPC_TPTODB_TYPE`）设置 `grpc.tptodb_server` 与 `grpc.tptodb_type`；如需固定方言口径，同时设置 `db.dialect` 并保持与 `tptodb_type` 一致。
4. **启动并验证**：启动后端。日志出现「外部遥测 gRPC 客户端初始化完成」即连接成功；若 `tptodb_server` 不可达或未配置，启动会 fail-fast 失败并回滚已启动服务——这是刻意设计，不允许"假连通"。启动后在前端设备详情页查看遥测历史/当前值，确认读路径已走外置库。
5. **验收口径**：`go test ./internal/app ./internal/dal ./third_party/grpc/tptodb_client/...` 不回归；方言适配层 `go test ./internal/dialect/...` 通过。真实国产库的迁移+读写验证属 residual，由具备外部环境的部署方执行。
6. **回退**：把 `grpc.tptodb_type` 改回 `NONE` 并重启即可恢复全本地路径（读路径立即回本地库；外置库中已写入的数据保留在服务端，不回迁）。

## 5. 方言适配层（backend/internal/dialect）使用说明

纯函数包，零驱动依赖。主要 API：

```go
// 解析与推导
d, err := dialect.Parse(cfg.Dialect)               // db.dialect 归一化（postgres/tdengine/kingbase/polardb）
d2, ok := dialect.FromTptodbType(cfg.TptodbType)   // tptodb_type → 方言（NONE/未知 → !ok）
d, err := dialect.Effective(cfg.Dialect, cfg.TptodbType) // 显式 > 推导 > postgres 默认；非法显式值报错

// 分页（limit∈[1,MaxInt32]，offset∈[0,MaxInt32]；内层查询禁含分号）
sql, err := dialect.Paginate(dialect.DialectKingbase, innerSQL, limit, offset)
// postgres/tdengine/polardb → "... LIMIT n OFFSET m"
// kingbase                  → "SELECT * FROM (SELECT page_src.*, ROWNUM AS page_rn
//                              FROM (...) page_src WHERE ROWNUM <= n+m) WHERE page_rn > m"

// 1 基页码换算（与 dal applyTelemetryPagination 同口径：offset=(page-1)*pageSize）
limit, offset, err := dialect.PageToOffset(page, pageSize)

// 元查询（SQL 与绑定参数成对返回；表名先过 ValidateIdentifier 防注入）
sql, args, err := dialect.TableExistsSQL(d, "telemetry_datas")
sql, args, err := dialect.ListColumnsSQL(d, "telemetry_datas")

// 能力矩阵与标识符引用
caps := d.Capabilities()                 // PageStyle/MetadataStyle/QuoteChar/UpperIdentifier/MaxIdentifierLen
quoted, err := dialect.QuoteIdentifier(d, "col")
```

原生驱动（若未来引入）落地时的接线点：以 `dialect.Effective` 的结果选择 gorm `Dialector` 与元查询执行器；在此之前，该包的映射结果不参与任何运行时 SQL 执行——这是本阶段的诚实边界。

## 6. 验证清单

| 检查 | 命令 | 期望 |
| --- | --- | --- |
| 方言适配层单测 | `go test ./internal/dialect/...`（在 `backend/` 下） | ok，全量表驱动用例通过 |
| 既有面不回归 | `go test ./internal/app ./internal/dal ./third_party/grpc/tptodb_client/...` | ok |
| 交叉编译属性 | `go build ./...`（或 `make cross-compile`） | 无 CGO 依赖 |
| 配置示例可解析 | 部署前 diff `configs/conf.example.yml` | 新键 `db.dialect` 与 `grpc` 段注释在位 |
