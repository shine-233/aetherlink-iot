# MQTT 模拟发布

## 目录定位

本目录提供面向调试、演示和业务验证的模拟 MQTT 发布入口，用于模拟设备向指定 topic 发送 payload。

## 文件说明

- `send.go`：按调用参数创建临时 MQTT 客户端并发布一次消息。
- `simulation_publish_test.go`：验证客户端选项构造和 payload 转换，不连接真实 Broker。

## 依赖关系

- 依赖 `github.com/eclipse/paho.mqtt.golang` 创建临时 MQTT 客户端。
- 通常由测试、调试脚本或运维验证流程调用，不应成为核心业务路径的强依赖。

## 当前实现边界

- `PublishMessage` 当前接受明文 `host`、`port`、`topic`、用户名、密码和 `clientId`，通过 `net.JoinHostPort` 组装 Broker 地址；函数本身没有对空值、topic 语法、QoS 或端口做集中校验。
- 连接和发布都使用 Paho token 的无超时 `Wait()`；Broker 不可达或发布阻塞时，调用方不能依靠本函数获得统一的 context deadline。
- 调试日志会记录 username、clientId、host、port、topic 和 payload；当前没有记录 password，但 payload/topic 仍可能含敏感业务数据，调用方必须避免把真实凭据或敏感报文传入日志路径。
- `simulation_publish_test.go` 只测试选项构造和 payload 转换，不连接真实 Broker；这份 README 不能证明真实 MQTT 发布成功。

## 审查记录与重构建议

- 问题描述：函数接收明文账号、密码、host、port 和 topic，缺少集中参数校验。
- 改进方案：增加请求结构体、上下文超时和字段校验，把敏感参数日志脱敏。
- 实施步骤：新增结构化参数版本并保留旧函数兼容，补齐失败路径单测，再迁移调用方。
- 预期效果：降低误用风险，并让模拟发布在 Broker 不可达时更快失败。

## 验证建议

- 修改客户端选项后运行 `go test ./mqtt/simulation_publish -count=1`。
- 真实设备模拟需配合本地或测试环境 Broker 手工验证。
