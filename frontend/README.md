# AetherLink IoT 前端

本目录是 AetherLink IoT 平台的 Vue 3 Web 客户端，覆盖设备管理、遥测数据、告警、自动化、可视化大屏、系统设置，以及用于预览和端到端验证的前端入口。

## 技术栈

- Vue 3、TypeScript、Vite、Pinia、Vue Router、Naive UI。
- Vitest 用于前端业务逻辑、路由合同和组件状态测试。
- Playwright E2E 通过仓库级 `automation_tests/` 工作区运行。

## 常用命令

```bash
pnpm install
pnpm dev
pnpm typecheck
pnpm test:coverage
pnpm build
```

## 目录关系

- `src/`：前端应用源码，包含页面、服务、路由、状态、组件和核心引擎。
- `packages/`：本地 workspace 包，提供请求、hooks、工具、物料和构建脚本。
- `public/`：静态公共资源；`src/assets/`、`src/theme/`、`src/styles/`：源码内的资源和展示层基础设施。
- `dist/`、`coverage/`、本地报告目录属于生成产物，不作为公开源码维护目标。

## 关键模块

### `src/views/`

承载设备、告警、自动化、可视化、系统管理等真实业务页面。当前静态整理优先级最高的区域集中在：

- `src/views/device/`
- `src/views/automation/`
- `src/views/visualization/`

这些页面通常同时包含路由同步、接口请求、弹窗状态、条件裁剪和表单提交，是最容易积累大文件与历史兼容逻辑的区域。

### `src/core/`

封装脚本引擎、交互系统、数据架构等核心能力。这里既有高复用价值，也有较强的兼容敏感性，尤其是：

- `src/core/data-architecture/`
- `src/core/interaction-system/`
- `src/core/script-engine/`

修改这些目录前，应先确认 README、导出边界和配置兼容要求。

### `src/components/`

存放页面复用组件、筛选器、面板、列表页和 ThingsVis 相关组件。公共组件改动往往会放大到多个页面，因此更适合做“先补说明、再做小步重构”的节奏。

### `src/service/` 与 `src/store/`

分别负责接口封装与状态管理。若页面行为依赖接口字段约定或权限状态，通常需要同步核对这两个目录，而不是只改页面模板。

可视化 provider 的权威契约位于 `src/service/visualization-provider/`：`native-board` 是默认本地实现，`legacy-thingsvis` 是显式启用的外部兼容 adapter。`src/service/visualization-provider.ts` 只保留历史导入兼容门面，不得重新形成第二套 provider 类型。

## 当前重构热点

| 区域 | 当前问题 | 改进方向 | 预期效果 |
| --- | --- | --- | --- |
| `views/device/*` | 详情壳层偏重，路由/Tab/在线状态/刷新逻辑耦合。 | 继续抽 composable 或局部 helper，补最小行为说明。 | 降低页面心智负担，便于后续扩展和验证。 |
| `views/automation/*` | 表单态与接口态转换复杂，历史兼容分支较多。 | 分离回显映射、提交映射和联动状态管理。 | 减少回填错位和提交流程回归风险。 |
| `core/data-architecture/*` | 配置桥接、数据源解析、兼容层概念密集。 | 明确层次边界，逐步抽纯函数和类型说明。 | 提高可理解性，降低误改兼容面的概率。 |
| `components/*` 公共组件 | README 与实际契约容易漂移。 | README 与代码一起维护，记录输入/输出和注意事项。 | 让新维护者能更快判断改动影响面。 |

## 文档与审查建议

- 新增页面或公共组件时，应至少补一段文件级说明，描述职责、关键输入、主要副作用和维护注意事项。
- 目录 README 不应只列文件名，还应说明上下游关系、依赖契约和当前已知重构建议。
- 改动公共组件、核心引擎或路由权限逻辑前，优先确认是否需要同步更新根目录 `README.md`、`COMPATIBILITY.md` 或对应目录 README。

## 维护注意事项

- 公开项目名称统一为 AetherLink IoT。
- ThingsVis、RDI、嵌入式 provider 等兼容标识属于运行时合同，修改前需要查阅根目录兼容性说明。
- 前端静态检查通过不等于完整发布就绪；正式发布仍需 typecheck、build、Vitest、API 自动化和 Playwright E2E 证据。
- 当前这一轮整理以“静态源码审查 + 中文文档化 + 低风险重构”为主，不能把 README 补齐或局部页面减负写成“已发布就绪”。
