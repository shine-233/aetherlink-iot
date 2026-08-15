# API 标准化文档

## 目录定位

本目录用于说明后端 API 的分层开发方式、路由位置、处理器职责、服务层边界和生成文件维护注意事项。

## 文件说明

- `开发帮助.md`：说明当前 AetherLink IoT 后端 API 相关目录、开发建议、配置位置和生成文件注意事项。

## 依赖关系

API 标准化文档需要与 `backend/router/`、`backend/internal/api/`、`backend/internal/service/`、`backend/internal/dal/` 和 `backend/docs/swagger.*` 保持一致。接口新增或迁移时，应同步更新本目录说明。

## 审查记录与重构建议

- 问题描述：API 开发容易在路由层、处理器层和服务层之间产生职责漂移。
- 改进方案：坚持 `router -> api -> service -> dal/query/model` 调用方向，并在 README 中固定审查口径。
- 实施步骤：新增接口时先确认路由归属，再补处理器、服务测试、Swagger 注释和自动化覆盖。
- 预期效果：减少业务规则散落在路由或工具函数中的情况，提升接口可维护性。

## 验证建议

- 修改 API 标准后检查对应代码目录是否仍遵守分层关系。
- 发布前结合自动化 API 目录和 Swagger 输出做一次接口清单比对。
