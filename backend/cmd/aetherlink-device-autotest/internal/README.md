# internal

## 功能定位

`internal` 承载自动测试工具的核心实现，负责把“真实环境集成测试”拆成配置、设备行为、协议构造、平台访问和断言工具几个层次。

它不是纯业务域模型，而是围绕设备接入验证搭起来的支撑层：既要能模拟设备上报，也要能辅助平台下行和数据库验收。

## 文件关系

- `config` 提供 YAML 配置模型和启动前校验。
- `device` 根据配置选择直连或网关设备实现，负责 MQTT 连接、订阅、发布和消息缓存。
- `protocol` 负责把测试语义数据转换成 MQTT payload。
- `platform` 负责调用平台 API 与读取 PostgreSQL 验证结果。
- `utils` 提供 topic、脱敏测试数据、日志和断言工具，横向支撑其余模块。

调用方向基本是：

`config -> device -> protocol`

`tests -> platform`

`device/tests/platform -> utils`

## 静态审查建议

### 协议一致性

- 先核对 `protocol` 和 `utils/mqtt_topics.go` 是否与项目根目录接入规范、验证文档和本地脱敏协议资料保持一致。
- 再检查 `device` 是否按同一 topic 与 payload 假设实现了发布、订阅和响应。

### 重复实现风险

- `device/direct_device.go` 与 `device/gateway_device.go` 有大段重复连接与缓存逻辑，审查时要特别关注是否已经漂移。
- `device/topology.go` 与 `config/config.go` 存在重复拓扑模型，后续字段变更容易只改一边。

### 断言可信度

- `platform/db_client.go` 中多处查询只取“最新一条”，不严格使用时间窗口。
- `utils/validator.go` 已补齐当前 MQTT topic `message_id` 解析；后续仍需继续补数值归一化、事件结构和更多 topic 模板测试。

## 重构建议

1. 抽出共享 MQTT 适配层，让直连/网关只保留协议差异。
2. 合并重复拓扑模型，避免配置字段漂移。
3. 为 topic 和 payload 增加契约 fixture 测试，降低外部协议回归风险。
