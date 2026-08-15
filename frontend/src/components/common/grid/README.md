# grid 组件说明

## 目录职责

本目录封装通用网格布局能力，用于承载可拖拽、可缩放、可响应式调整的页面组件布局。它在第三方 `grid-layout-plus` 能力之上提供统一类型、默认配置、错误处理、历史记录、性能优化和组件化渲染边界。

## 文件关系

- `GridLayoutPlus.vue` 是主要 Vue 入口，负责归一化布局、合并配置、订阅 hooks，并向外转发网格生命周期和交互事件。
- `components/` 承载核心渲染层、拖放区和网格项内容包装。
- `hooks/` 承载布局状态、响应式、性能、历史、虚拟网格和增强版布局逻辑。
- `utils/` 承载布局校验、断点计算、性能辅助、通用工具和布局算法。
- `gridLayoutPlusTypes.ts` 定义增强网格的公开类型和默认配置。
- `gridLayoutPlusUtils.ts`、`utils-enhanced.ts` 提供增强布局操作和兼容工具。
- `errorHandler.ts` 统一处理网格校验、渲染、布局和性能错误。
- `index.ts`、`gridLayoutPlusIndex.ts` 是对外导出入口。

## 重点文件

- `GridLayoutPlus.vue`：外部页面最常使用的网格布局组件。
- `components/GridCore.vue`：第三方网格组件适配层，事件转发和插槽回退都集中在这里。
- `hooks/useGridLayoutPlus.ts`：增强版网格状态管理入口，聚合校验、排序、搜索、历史、响应式和性能工具。
- `gridLayoutPlusUtils.ts`：布局增删改查、压缩、碰撞检测、导入导出等关键算法集合。
- `errorHandler.ts`：错误收集和安全执行工具，测试环境会抑制未 mock 的控制台噪声。

## 审查建议

- 审查布局变更时先看类型和默认配置，再看 hooks 与工具函数，最后看 Vue 渲染层是否按同一语义转发。
- 对拖拽、缩放、断点、历史记录、导入导出和错误恢复的改动，应优先补充 `*.test.ts` 中的边界用例。
- 不要在布局工具里写入页面业务假设，网格项内容应继续通过插槽、`component` 或 `props` 传入。
- 保持 `index.ts` 与 `gridLayoutPlusIndex.ts` 的导出边界清晰，避免同一能力出现多个不一致入口。
