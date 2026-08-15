# backend/router/apps

## 目录定位

`backend/router/apps` 按业务域拆分后端 API 路由注册，是 `backend/router/router_init.go` 在鉴权链之后挂载设备、告警、角色、OTA、RDI、通知等模块的主要入口。

## 文件用途

- `enter.go`：聚合各模块路由注册结构，供 `router_init.go` 统一调用。
- `device.go`、`telemetry_data.go`、`attribute_data.go`、`event_data.go`、`command_data.go`：注册设备数据和上下行相关接口。
- `alarm.go`、`scene.go`、`scene_automations.go`、`notification_*.go`、`message_push.go`：注册告警、场景联动和通知相关接口。
- `sys_user.go`、`role.go`、`casbin.go`、`sys_dict.go`、`sys_function.go`、`sys_ui_elements.go`：注册用户、权限和系统配置接口。
- `ota.go`、`protocol_plugin.go`、`service_plugin.go`、`rdi.go`、`open_api_keys.go`、`upload.go`、`logo.go` 等文件：注册插件、开放能力、上传和辅助业务接口。

## 依赖关系

本目录主要依赖 `backend/internal/api` 的控制器结构和 Gin 路由组；上游由 `backend/router` 传入已套用 JWT/Casbin 中间件的 `v1` 路由组。前端 API wrapper、自动化测试目录和外部集成方会依赖这些路径。

## 审查发现

- 多数文件只做路由声明，逻辑风险低，但路径字符串属于外部契约，不能随意改名或移动。
- 路由注册与权限菜单、前端请求封装和 API 自动化之间没有完全自动同步，新增接口容易遗漏测试或文档。
- 该目录已有文件头注释覆盖，本轮不重复修改业务路由文件。

## 重构建议

后续可为每个模块生成机器可读路由清单，并把权限菜单、前端 wrapper 和自动化目录纳入同一审查链；大型模块可按公开接口、管理接口和设备侧接口继续拆分。

## 验证建议

改动本目录后至少运行 `cd backend; go test ./router -run TestRouterContract -count=1`。如果新增或改动具体接口，应补充对应 API 自动化或 handler/service targeted 测试。
