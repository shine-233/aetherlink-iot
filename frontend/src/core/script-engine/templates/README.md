# script-engine 模板目录

## 目录定位

`frontend/src/core/script-engine/templates` 存放脚本引擎的内置模板定义。这里的模板主要服务于脚本编辑器、规则配置和数据处理入口，用来快速生成可读、可改、可复用的起始脚本。

## 文件职责

- `built-in-templates.ts` 是兼容 facade，负责聚合模板、初始化注册和统计，对外 API 保持不变。
- `data-fetcher-templates.ts`、`data-processor-templates.ts`、`data-merger-templates.ts`、`utility-templates.ts` 按场景存放静态模板。
- `definition-types.ts` 存放模板共享定义类型。

## 维护边界

- 模板脚本、参数和使用说明只在对应场景文件维护；聚合、注册和统计逻辑留在兼容 facade，共享类型留在 `definition-types.ts`。
- 模板应只表达可复用的脚本片段、参数说明和输出形态，不应包含页面状态、网络副作用或绕过脚本沙箱的逻辑。
- 网络类模板只能调用 `_utils.networkUtils` 保留接口，禁止直接调用宿主 `fetch`；未接入可取消、可审计的宿主适配器时，必须明确返回 `SCRIPT_NETWORK_EXTERNAL_BLOCKED`，不得伪装成功。
- 新增模板时，同步检查模板名称、分类、参数描述和使用片段说明。

## 静态审查

- 确认场景模板只改对应静态模板文件，兼容 facade 未重复保存模板本体。
- 确认模板分类、注册、统计和对外导出保持一致。
- 确认模板说明与脚本行为一致，且文件不存在行尾空白。
