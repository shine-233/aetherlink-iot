# Data Architecture Templates

## 目录职责

维护数据架构配置器使用的预置模板，当前重点是 HTTP 数据源模板。这里的内容用于帮助用户快速生成可编辑的请求配置，因此属于“默认配置契约”的一部分。

## 文件关系

- `http-templates.ts` 依赖 `types/http-config.ts` 中的 `HttpConfig` 类型。
- 模板中的内部接口路径应与 `data/internal-address-data.ts` 的地址清单保持一致。
- 模板字段会被配置器、导入导出和兼容测试间接消费，不能只按展示文案理解。

## 重点文件

- `http-templates.ts`：维护 GET、POST、遥测统计等 HTTP 配置模板，是审查模板默认值、动态变量、请求脚本和参数兼容性的主入口。

## 审查建议

- 修改 URL、参数名、`variableName`、`dataType` 或脚本默认值时，先确认已有保存配置是否依赖旧字段。
- 新增业务模板时，优先复用 `internal-address-data.ts` 已登记的内部 API，避免模板路径和选择器路径分叉。
- 审查时重点看模板是否混入临时演示数据、过期 token 或不再存在的后端接口。
