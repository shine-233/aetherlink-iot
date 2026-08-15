# backend/router

## 目录定位

`backend/router` 是后端 HTTP 入口的路由装配层，负责把 Gin 引擎、中间件、公开接口、鉴权接口、SSE、静态文件、Swagger 和 Prometheus metrics 串起来。

## 文件用途

- `router_init.go`：创建 Gin 路由，挂载 Swagger、metrics、静态文件、公共 API、JWT、Casbin 和各业务路由模块。
- `sse.go`：注册登录后的系统事件 SSE 路由。
- `router_contract_test.go`：用 AST 检查关键公开路由和 P0/P1 业务路由注册是否仍存在。
- `apps/`：按业务模块拆分具体路由注册方法。
- `publicfiles/`：提供 `/files/*filepath` 的本地文件路径解析与逃逸防护。

## 依赖关系

本目录依赖 `backend/internal/api`、`backend/internal/middleware`、`backend/internal/service`、`backend/pkg/global`、`backend/pkg/metrics` 和 `backend/router/apps`。前端 service wrapper、API 自动化目录和部署网关都会依赖这里暴露的路径契约。

## 审查发现

- 路由层同时承担公开入口、鉴权链和业务模块注册，改动一个路径可能影响前端、自动化、Swagger 和第三方集成。
- `/files/*filepath` 已通过 `publicfiles.ResolvePath` 做路径逃逸防护，不应退回直接拼接本地路径。
- `router_contract_test.go` 只证明关键路径仍被注册，不能证明完整业务行为正确。

## 重构建议

后续可把公共接口、登录后免 Casbin 接口、业务模块接口和运维接口拆成更清晰的注册函数，并把路由清单生成到 API 自动化目录，减少手工同步遗漏。

## 验证建议

修改本目录后运行 `cd backend; go test ./router ./router/publicfiles -count=1`。若改动业务路径，还应同步运行对应 API 自动化或前端 service wrapper 的 targeted 检查，避免 broad `go test ./...` 掩盖路由契约问题。
