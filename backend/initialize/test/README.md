# 初始化相关测试

本目录保存初始化模块的 Go 测试，主要用于验证自动化缓存、告警缓存和构建标签边界。

## 文件说明

- `automate_cache_test.go`：带 `integration` 构建标签的自动化缓存集成测试，需要可用的本地配置和 Redis。
- `alarm_cache_test.go`：带 `integration` 构建标签的告警缓存集成测试，需要初始化日志、配置和 Redis。
- `buildtag_boundary_test.go`：用于保证该测试包在未启用 `integration` 标签时仍有合法包边界。

## 运行提示

- 普通快速检查可运行 `go test ./backend/initialize/test`，此时带 `integration` 标签的测试不会执行。
- 需要真正跑缓存集成测试时，再显式增加 `-tags integration`，并先确认 `backend/configs/conf-localdev.yml` 中的 Redis 配置可连接。
