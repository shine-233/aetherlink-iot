# topicalias fifo

## 目录定位

这个目录实现 MQTT topic alias 的 FIFO 管理策略，用于在客户端维度维护主题名和 alias 编号之间的映射。

## 文件用途

- `fifo.go`：注册 `fifo` topic alias 管理器，并实现 alias 分配、命中和满队列淘汰。
- `fifo_test.go`：验证 alias 从 1 递增分配、已有 topic 命中、新 topic 复用最早 alias。

## 生成物、示例、测试数据边界

本目录只包含手写 Go 源码和测试，没有生成物、示例配置或外部测试数据。测试数据在测试函数内构造。

## 审查发现

- FIFO 行为简单清楚，使用 list 保存顺序、map 提供 topic 到 alias 的快速查询。
- 当前测试覆盖正常容量下的分配和淘汰，但没有覆盖 `maxAlias == 0` 等异常配置。
- 测试创建了 gomock controller 但没有实际 mock 依赖，可作为后续清理点。

## 重构建议

- 明确 `maxAlias == 0` 的期望行为，必要时在构造或 `Check` 中做保护。
- 用表驱动测试补充边界容量、重复 topic 和连续淘汰场景。
- 如果未来允许并发访问同一管理器，需要显式加锁或在接口层声明单协程约束。

## 验证建议

- 修改本目录后运行：`go test ./topicalias/fifo -count=1`。
- 修改注册名或工厂逻辑时，额外检查 broker 配置中 `fifo` 策略是否仍可加载。
