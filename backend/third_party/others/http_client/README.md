# 协议插件 HTTP 客户端

## 目录定位

本目录封装后端调用协议插件服务的 HTTP 能力，包括表单配置查询、设备增删改通知、服务设备列表查询和签名 Webhook。

## 文件说明

- `request_method.go`：基础 JSON 请求、GET/POST/DELETE、HMAC 签名请求和默认超时客户端。
- `protocol_plugin.go`：面向协议插件业务 API 的函数封装，统一协议插件相关 HTTP 调用。
- `request_method_test.go`：使用本地 `httptest` 验证请求头、签名和错误状态处理。

## 依赖关系

- 依赖 `backend/pkg/errcode` 映射插件错误。
- 被设备协议、插件表单、服务访问和通知流程调用。
- 与外部协议插件服务的 HTTP API 强耦合，接口变更需同步后端调用和业务文档。

## 审查记录与重构建议

- 问题描述：部分函数返回 `*http.Response`，调用方必须自行关闭 Body；URL 通过字符串拼接生成，缺少统一 host 校验。
- 改进方案：引入结构化 `Client`、统一 URL builder、context 超时和响应体关闭约定。
- 实施步骤：先为新代码提供 `Client` 方法，保留旧函数转调，再为所有调用点增加测试覆盖。
- 预期效果：减少连接泄漏，提升错误处理一致性，并降低 SSRF 与敏感日志风险。

## 验证建议

- 修改基础请求或签名逻辑后运行 `go test ./third_party/others/http_client -count=1`。
- 修改插件业务 API 路径后需与协议插件服务联调。
