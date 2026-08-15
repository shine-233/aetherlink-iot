# 后端测试辅助目录

## 目录定位

`backend/test` 保存少量跨模块测试辅助文件和危险集成测试示例。大部分常规单元测试应继续放在对应业务包旁边，本目录只承载确实需要跨包或独立环境说明的测试材料。

## 文件说明

- `rsa_test.go`：验证 RSA 公私钥加载、加密、解密和密码恢复流程；测试运行时生成临时 PEM 密钥，并覆盖缺失文件、非法 PEM 和未初始化操作，不读取或提交仓库/生产密钥。
- `pg_test.go`：受 `dangerous_integration` build tag 保护的数据库重建测试，会执行 `DROP SCHEMA public CASCADE`，只能在可销毁的隔离数据库运行。
- `multidb/`：提供 MySQL、PostgreSQL 和 Adminer 的本地 docker-compose 示例。

## 依赖关系

- `rsa_test.go` 只依赖 Go 标准库生成的临时测试密钥，不依赖固定目录、部署密钥或外部服务。
- `pg_test.go` 依赖 `backend/configs/conf-localdev.yml`（`run_env=localdev`）、PostgreSQL、`backend/sql/1.sql` 和 `run_env` 环境变量；源码的 `run_env=git-actions` 分支引用 `../configs/conf-push-test.yml`，当前树中没有该配置文件，因此不能把该分支描述为可直接复现。
- `multidb/docker-compose.yml` 只用于本地临时数据库，不应作为生产部署配置。

## 审查记录与重构建议

- 已完成：RSA 测试改为测试内临时生成密钥，不再因本地路径差异而跳过。
- 剩余问题：数据库集成测试具备破坏性，仍必须显式启用 build tag 并只连接可销毁数据库。
- 后续方案：数据库测试迁移到容器化临时库，并默认使用独立 schema，再收敛危险 SQL 重置操作。
- 预期效果：继续减少环境差异，并避免误删共享数据库。

## 验证建议

- 默认轻量验证：`go test ./test -count=1`；RSA 测试会生成并清理自己的临时密钥。
- 危险集成验证必须显式开启：`go test -tags dangerous_integration ./test -run TestDatebase -count=1`，且只能指向可销毁数据库；该命令会重建 `public` schema，不能对共享或生产数据库执行。
