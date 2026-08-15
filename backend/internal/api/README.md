# 后端 API 层

`backend/internal/api` 保存 AetherLink IoT 后端的 Gin HTTP 处理入口，负责把外部请求转换成后端 service 可消费的参数，并把 service 返回结果包装成前端和外部调用方可识别的接口响应。

这层的理想边界是“轻入口、薄编排、强可追踪”：尽量不把长期可复用的业务规则直接堆在 handler 里。

## 目录职责

- 绑定请求参数、校验基础字段、处理接口级鉴权前置条件。
- 调用 `internal/service` 完成真正的业务规则、租户边界和持久化编排。
- 将 service 层结果包装成前端、自动化和外部调用方可消费的响应结构。
- 维持接口入口、路由、错误码、返回字段之间的关系容易追溯。

## 重要 handler 文件与 service 对应关系

- `device.go`、`device_config.go`、`device_model.go`、`device_auth.go`、`device_debug.go`
  - 对应 `internal/service/device*.go` 设备生命周期、配置、模型、调试等能力。
- `device_connection_diagnostics.go`
  - 集中设备诊断与连接诊断 HTTP 参数/响应编排，对应 `internal/service/device_connection_diagnostics.go`；`device.go` 继续保留通用设备生命周期入口。该纯迁移已通过 `GOMAXPROCS=1 go test -p=1 -gcflags=all='-N -l' ./internal/api ./internal/storage`。
- `scene_automations.go`、`scene.go`
  - 对应 `internal/service/scene_automations.go` 和自动化执行相关 service。
- `alarm.go`
  - 对应 `internal/service/alarm.go` 与通知相关链路；年度月度告警趋势走 `GET /alarm/info/history/monthly?year=YYYY&timezone=IANA`，历史列表、单条详情展开和汇总统计都必须在 service/DAL 层保持同一设备 owner 范围。
- `telemetry_data.go`、`attribute_data.go`、`event_data.go`、`expected_data.go`
  - 对应遥测、属性、事件、期望值相关 service。
- `command_set_log.go`
  - 对应普通异步命令、单设备在线 Direct Method、命令日志/诊断和 Command Jobs；Direct Method 的 1-30 秒等待使用 HTTP request context，发布成功与设备成功必须保持不同字段。
- `service_access.go`、`service_plugin.go`、`protocol_plugin.go`
  - 对应接入服务、插件和协议接入相关 service。
- `casbin.go`、`role.go`
  - 对应角色、功能权限、用户授权关系与租户角色管理相关 service。
- `system_monitor_api.go`
  - 对应系统监控指标查询与管理员只读监控能力。
- `ota.go`
  - 对应 OTA 升级包、升级任务、固件下载和分片下发相关 service 与下载链路。
- `rdi.go`
  - 对应 RDI 相关 service 和设备扩展链路。
- `sys_user.go`
  - 对应登录、租户用户管理、账号安全、验证码与个人资料相关 service。
- `board.go`、`dashboard_menu.go`
  - 对应看板与可视化相关 service。
- `enter.go`
  - 是较高频的入口/聚合文件之一，适合承接公共绑定、校验和响应 helper。

## 统一模式与维护约束

- 参数绑定：
  - handler 负责读 path、query、body，并把基础参数错误尽量提前暴露。
- 权限与业务边界：
  - 真正的租户边界、设备边界、场景边界应落在 service，而不是长期停留在 handler。
- 响应包装：
  - 尽量走统一响应包装链，不要在局部 handler 自创不兼容返回结构。
- 历史兼容：
  - 已存在的兼容字段、兼容文案和兼容结构不要随意扩散；能隔离在少数入口的尽量隔离。

## 当前静态审查结论

### 发现的问题

- 当参数绑定、权限条件、历史兼容和响应包装同时出现时，handler 很容易变厚。
- 一些入口文件已经有重复的 bind、validate、response 模式，但目录级文档还不够明确。
- API 层 README 之前更偏原则说明，对“哪个 handler 对哪个 service”帮助还不够。
- 权限与角色相关入口已经出现 `MustGet("claims")` 假设、硬编码权限字符串和局部手写 JSON 响应，说明权限边界和统一错误协议仍有收敛空间。
- Casbin 这类“先删后加”的授权覆盖流程已经暴露出事务性和补偿语义风险，单靠 handler 注释难以完全兜住。
- OTA 下载链路已经暴露出路径穿越、Range 边界、CRC16 游标前置条件与 416/500 错误出口一致性这些更偏基础设施层的风险点。
- 用户与 RDI 相关入口同时承载安全敏感动作和高副作用设备动作，目录级文档需要持续强调“API 层只做协议适配，安全与归属校验必须在 service 层闭合”。

### 改进方案

- 继续保持 handler 只做参数入口、响应出口和薄编排。
- 把跨 handler 的绑定、校验、错误包装下沉成 helper 或 service 级能力。
- 在目录 README 中保留 handler -> service 的映射表，降低维护者跨层跳转成本。
- 对权限、角色、监控这类高敏感入口，优先统一 claims 读取、管理员常量和错误响应格式，避免局部自行实现。
- 对 Casbin 覆盖写入、角色删除前检查等高风险链路，在 service 层补强原子性与审计语义说明。
- 对 OTA、RDI、用户安全链路这类“高副作用 + 高敏感”入口，优先统一路径参数读取、token/claims 获取与错误出口风格。
- 对 `enter.go` 中的公共绑定与 WebSocket 升级策略，后续可继续沉淀为更明确的安全与兼容约束说明。

### 建议实施步骤

1. 先继续补高频入口文件的中文文件头和 helper 边界。
2. 再识别重复的绑定、校验、响应包装模式，逐步收敛。
3. 静态批次完成后，再做聚焦接口回归验证，而不是把广泛编译当成完成证明。

### 预期效果

- API 入口职责更清楚，定位问题时更容易判断该看 handler 还是 service。
- 前后端联调和自动化断言更容易围绕真实契约排查。
- GitHub 浏览者能更快理解“HTTP 层职责”和“业务规则职责”的分工。
