# Data Architecture Static Data

## 目录职责

维护数据架构编辑和配置流程使用的静态数据。当前主要承载内部 API 地址、模块分组、参数种子值和检索辅助函数。

## 文件关系

- `internal-address-data.ts` 使用 `types/internal-api.ts` 定义的 `InternalAddressOptions` 和 `InternalApiItem`。
- 内部 API 地址会被 HTTP 配置表单、内部地址选择器和模板逻辑引用。
- 与 `templates/http-templates.ts` 存在契约关系：模板里的内部接口路径和参数应能在这里找到来源或解释。

## 重点文件

- `internal-address-data.ts`：维护 telemetry、device、attribute、event、alarm 等模块的内部接口清单，并提供按模块、地址和关键字检索的工具函数。

## 审查建议

- 后端接口路径、方法或参数变更时，先同步本文件，再检查模板和 UI 选择器是否仍能正确生成配置。
- 参数种子值应保持安全、稳定、可解释，避免写入真实账号、token 或环境私有标识。
- 如果接口清单继续扩大，建议评估从 OpenAPI 或后端路由清单生成，减少手工维护漂移。
