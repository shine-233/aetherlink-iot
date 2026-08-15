# 多数据库本地测试栈

## 目录定位

本目录保存用于本地联调的多数据库 `docker-compose.yml`，帮助开发者快速启动 MySQL、PostgreSQL 和 Adminer。

## 文件说明

- `docker-compose.yml`：启动 MySQL、PostgreSQL 和 Adminer 的示例配置。

## 审查记录与重构建议

- 问题描述：当前配置使用示例密码和默认端口，适合本地快速调试，但不具备生产安全性。
- 改进方案：增加项目专用 Docker 网络、健康检查、数据卷隔离和 `.env.example`。
- 实施步骤：先补 README 标注边界，再为危险集成测试创建独立 compose profile。
- 预期效果：减少误用风险，并让数据库集成测试更可重复。

## 验证建议

- 本地启动：`docker compose up -d`。
- 使用完毕后按需执行：`docker compose down -v`，避免测试数据残留影响下一次验证。
