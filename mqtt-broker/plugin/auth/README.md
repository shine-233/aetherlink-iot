# Auth 认证插件

`mqtt-broker/plugin/auth` 提供 MQTT CONNECT 用户名/密码认证，以及账号管理 API 和密码文件持久化能力。

## 目录定位

- 通过 broker hook 校验 MQTT CONNECT 的用户名和密码。
- 从 YAML 密码文件加载账号，并在账号 API 更新后写回。
- 通过 Admin/gateway API 栈暴露账号列表、详情、更新和删除接口。
- 兼容历史 hash 类型：`plain`、`md5`、`sha256`、`bcrypt`。

## 关键文件关系

- `auth.go`：插件注册、密码记录加载、凭证校验、hash 生成/比较和账号 API 注册。
- `config.go`：密码文件路径和 hash 算法配置。
- `hooks.go`：把认证失败映射为 MQTT v3/v5 鉴权错误。
- `grpc_handler.go`：账号管理 API 与密码文件写入路径。
- `testdata/`：认证与账号 handler 测试使用的密码文件样例。
- `protos/`、`swagger/`、`account*.pb.go`、`account*.pb.gw.go`、mock 文件：API 契约和生成物。

## API 文档

本仓库内置 Swagger 位于：

- `swagger/account.swagger.json`

发布文档应引用本仓相对路径，不再引用上游仓库链接。

## 维护注意事项

- 密码处理属于安全敏感路径，日志中不得输出明文密码或 hash 原文。
- 当前默认 hash 可能保留弱算法以兼容旧部署，迁移前必须提供兼容计划。
- 密码文件写入需要关注并发、原子性、路径安全和错误回滚。
- 修改账号 API 时应同时检查 Admin 插件复用的分页/索引逻辑。

## 代码审查与重构建议

- 问题：认证逻辑、账号管理 API 和文件持久化耦合在一起，安全审查容易漏掉写文件路径。
- 改进方案：把 hash 策略、账号仓储、API handler 和 MQTT hook 校验拆成独立测试边界。
- 实施步骤：先补 malformed YAML、弱 hash 兼容和写文件失败测试，再考虑迁移默认 hash。
- 预期效果：认证插件更安全，历史账号兼容和未来迁移路径更清楚。
