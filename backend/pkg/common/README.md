# backend/pkg/common

## 目录定位

`backend/pkg/common` 是后端公共工具包，放置跨 API、service、DAL、MQTT 响应等场景都会复用的小型帮助函数。这里应只承载通用能力，不承载租户、权限、设备等业务规则。

## 文件用途

- `common.go`：提供空字符串判断、JSON 序列化、错误包装、MQTT 响应体、随机字符串和数字验证码等通用工具。
- `time.go`：提供本日、本月、本年起点计算，以及场景联动定时条件的下一次执行时间计算。
- `lock.go`：基于全局 Redis 客户端提供带随机所有权 token 和原子校验释放的分布式锁；旧布尔接口仅保留兼容契约。
- `user.go`：封装系统管理员角色判断。
- `http.go`：保存公共成功状态常量。
- `common_time_test.go`：覆盖字符串、JSON、错误、随机码、管理员判断和时间计算等公共行为。

## 依赖关系

本目录依赖 `backend/pkg/constant` 获取角色和空值常量，依赖 `backend/pkg/global` 使用 Redis 全局客户端；时间调度依赖 `github.com/robfig/cron`，错误包装依赖 `github.com/pkg/errors`。调用方通常来自后端 service、API 和消息处理链路。

## 审查发现

- 公共函数较多且用途分散，调用前需要确认函数是否真的通用，避免把业务分支继续塞进公共包。
- `AcquireLockToken`/`ReleaseLockToken` 通过随机 token 和 Lua 比较删除保护锁所有权；Redis 未初始化时获取会失败关闭，旧 `ReleaseLock` 不再执行无条件删除。
- 场景时间解析依赖约定字符串格式，格式变化会影响自动化执行时间。

## 重构建议

后续可按职责拆成 `json`、`random`、`timewindow`、`lock` 等更小文件或子包，并把 Redis 锁抽象成可注入接口，降低全局变量耦合和单元测试成本。

## 验证建议

修改本目录后优先运行 `cd backend; go test ./pkg/common -count=1`。如果改动涉及 Redis 锁，需要补充带 mock Redis 或集成 Redis 的窄范围测试，不要用 broad `go test ./...` 代替。
