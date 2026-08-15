# 依赖与部署能力矩阵

> 本文件描述当前工作树的依赖用途和默认部署边界。它不是删包清单；删除依赖前必须证明无源码引用、无生成用途、无外部合约，并通过对应测试。

## 默认核心能力

| 能力 | 实现/依赖 | 默认状态 | 处置 |
| --- | --- | --- | --- |
| 数据库 | PostgreSQL + GORM PostgreSQL driver | required/core | 保留；默认 Compose 提供。 |
| 缓存 | Redis client | required/core | 保留；默认 Compose 提供。 |
| MQTT 接入 | 仓库内 `mqtt-broker`、Paho adapter | required/core | 保留；topic、认证和插件接口属于契约。 |
| 可视化 | frontend native board provider | available/core | 作为 ThingsVis 的本地替代，默认不依赖外部镜像。 |
| 文件与遥测降级 | 本地卷、file spool、durable input | local/core | 保留；不要求对象存储或外部时序库才能启动。 |

## 测试、生成与开发依赖

| 依赖 | 真实引用 | 默认运行时是否必需 | 结论 |
| --- | --- | --- | --- |
| `github.com/glebarez/sqlite` | 多个 backend DAL/service/storage 测试 | 否，仅测试 | 保留；提供无 cgo 的本地数据库测试替代。 |
| `gorm.io/driver/mysql` / `gorm.io/driver/sqlserver` | `casbin/gorm-adapter` 依赖链与 multidb fixture | 否，显式测试/兼容链 | 暂不删除；先确认上游 adapter 构建约束。 |
| `gorm.io/gen`、Swagger generator/files | GORM/Swagger 生成与已提交 API 文档 | 生成/开发 | 保留生成契约；参见 `GENERATED_FILES.md`。 |
| `github.com/xuri/excelize/v2` | 设备预注册、遥测/数据导出服务 | 业务可选入口 | 保留；不能按体积误判为闲置。 |
| `github.com/yuin/gopher-lua` | backend 本地脚本处理器 | 核心业务能力 | 保留；本地脚本能力是外部脚本服务的轻量替代。 |
| `github.com/prometheus/client_golang` | backend/mqtt-broker 指标 | 可选观测能力 | 保留；指标输出不得成为核心业务的外部阻断。 |
| `google.golang.org/grpc` + generated client | `backend/third_party/grpc/tptodb_client` | 外部可选 | 保留 symbol/method path；默认使用本地 PostgreSQL 遥测路径。 |

## 外部可选能力与本地替代

| 能力 ID | 本地/默认替代 | 外部阻断条件 | 对外契约 |
| --- | --- | --- | --- |
| `thingsvis` | native board provider | optional profile 未启用、镜像/运行时未配置 | provider、embed、SSO、`tv:*` 消息语义保留。 |
| `http-adapter` | backend 内部 API/MQTT adapter | optional profile 未启用或外部镜像不可达 | HTTP protocol plugin 注册、通知、错误分类保留。 |
| `market` | 本地模板/配置能力 | `market.enabled` 未显式为 `true`，或未配置合法 `market.base_url` | 返回明确 unavailable/blocked，不访问占位地址；超级管理员初始化复用同一 fail-closed 边界。 |
| `smtp` | 本地通知历史和受控失败审计 | provider 未启用或 SMTP 不可达 | 投递错误分类与审计契约保留。 |
| `map-provider` | 无外部 key 时页面降级 | 未配置 provider key | 不加载外部 SDK，不阻断核心平台。 |
| `external-telemetry-store` | 本地 PostgreSQL telemetry + file spool | `grpc.tptodb_type` 选择 TSDB/KINGBASE/POLARDB 时启用；无 endpoint 为 `configuration-required`，有 endpoint 但无健康探针为 `external-blocked` | 生成 gRPC symbol、method path 和 wrapper 保留。 |

## 状态语义

`backend/internal/service/deployment_capability.go` 是能力状态模型：

- `disabled`：默认核心路径已足够，外部能力未启用。
- `configuration-required`：显式选择了能力，但缺少必要非秘密配置。
- `blocked`：核心或本地能力已配置，但当前本地运行条件未满足。
- `external-blocked`：外部可选能力已配置，但运行时探测、外部镜像或凭据条件尚未满足；不阻断默认核心栈。
- `available`：已配置并通过当前探测。

能力状态只输出 ID、分类、状态和布尔值，不输出 URL、密码、token、endpoint 或原始 provider 配置。

## 审查与删包规则

1. 先用 `go mod why -m`、源码 import、生成命令和测试覆盖确认依赖用途。
2. 仅删除没有直接/间接运行、测试、生成或兼容用途的依赖；同步 manifest、lockfile 和验证证据。
3. 外部能力必须保留接口契约；能本地化的核心能力优先走仓库内实现，不能本地化的能力必须显式标为 optional 或 external-blocked。
4. 依赖矩阵变化要同步本文件、目标测试和 `references/README.md`，并重新运行模块级测试。
