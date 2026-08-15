# backend/pkg/metrics

## 目录定位

`backend/pkg/metrics` 是后端运行指标和可选遥测包，负责 Prometheus 指标、内存历史曲线、系统资源采集和 PostHog 遥测事件上报。

## 文件用途

- `metrics.go`：定义 Prometheus 指标、历史存储接口、系统指标结构和周期采集逻辑。
- `memory_storage.go`：提供内存历史数据存储、保留期清理和当前指标读取。
- `collect.go`：构造实例信息、持久化实例 ID，并通过遥测周期发送数据。
- `telemetry.go`：控制遥测启用、心跳间隔、注册/升级/心跳事件、状态文件读写和事件 payload。
- `memory_storage_test.go`：覆盖内存存储、组合历史和无存储兜底。
- `telemetry_test.go`：覆盖遥测生命周期、配置兜底、状态读写和实例 ID 持久化。

## 依赖关系

本目录依赖 Prometheus 客户端、`gopsutil`、Viper、Logrus、UUID 和 `backend/pkg/global` 的版本信息。路由层会创建 `Metrics` 并挂载 metrics 中间件、`/metrics`、系统监控服务和内存历史存储。

## 审查发现

- Prometheus 指标注册使用默认全局 registry，重复创建同名 namespace 可能在测试或多实例初始化中冲突。
- 遥测应保持显式启用；关闭时不应创建状态文件或发送网络请求。
- 内存存储适合短期展示，不是持久化审计来源。

## 重构建议

后续可支持传入自定义 Prometheus registry，减少测试间共享状态；遥测发送可抽象为接口，便于离线环境、重试策略和隐私审计。

## 验证建议

修改本目录后运行 `cd backend; go test ./pkg/metrics -count=1`。如果改动指标注册或路由接入，还应运行路由包的 targeted 测试，确认 `/metrics` 和系统监控仍可注册。
