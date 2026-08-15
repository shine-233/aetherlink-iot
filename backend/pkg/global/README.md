# backend/pkg/global

## 目录定位

`backend/pkg/global` 保存后端进程级共享对象，包括数据库、Redis、Casbin、响应处理器、版本号，以及 SSE/WebSocket 长连接管理器。

## 文件用途

- `global.go`：定义全局版本、数据库、Redis、Casbin、响应处理器和事件通道变量。
- `SSEManager.go`：实现基于 Redis Pub/Sub 的租户级 SSE 客户端管理和事件广播。
- `WSManager.go`：实现设备维度 WebSocket 订阅、字段过滤、推送和 Redis 订阅计数。
- `sse_manager_test.go`：验证 SSE 客户端注册、租户隔离和移除清理。
- `ws_manager_test.go`：验证 WebSocket 字段过滤、统计和缓冲区满时的非阻塞行为。

## 依赖关系

本目录依赖 GORM、Redis、Casbin、Gin、Gorilla WebSocket 和响应中间件。初始化通常由 `backend/internal/app` 或路由初始化过程完成；调用方遍布中间件、service、公共锁、SSE/WS API 和后台任务。

## 审查发现

- 全局变量简化了历史代码接入，但也会隐藏初始化顺序和测试隔离风险。
- SSE/WS 管理器依赖 Redis Pub/Sub 支撑多实例广播，Redis 不可用时需要调用方或初始化层明确降级策略。
- WebSocket 推送采用非阻塞发送，缓冲区满时会丢弃消息以保护管理器不被阻塞。

## 重构建议

后续新增代码优先使用显式依赖注入，逐步把数据库、Redis、响应处理器和连接管理器收敛到应用上下文中；SSE/WS 可补充上下文取消能力，便于优雅停止和测试退出。

## 验证建议

修改本目录后运行 `cd backend; go test ./pkg/global -count=1`。如果改动 Redis Pub/Sub 或连接生命周期，还应增加窄范围集成测试或使用模拟 Redis 验证断线重连和清理行为。
