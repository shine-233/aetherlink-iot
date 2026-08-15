# common 组件说明

## 目录职责

本目录承载前端可复用的通用组件与轻量工具，主要服务于应用外壳、主题切换、语言切换、图标选择、全屏、异常展示以及网格布局组件。这里的组件通常被多个页面或业务模块共享，修改时需要优先确认调用方范围。

## 文件关系

- `grid/` 是当前重点维护的网格布局封装，包含 `GridLayoutPlus.vue`、组合式 hooks、布局工具函数、错误处理与局部组件。
- `icons.ts` 为通用图标集合入口，`icon-selector.vue` 负责图标选择 UI。
- `app-provider.vue`、`dark-mode-container.vue`、`theme-schema-switch.vue` 负责应用级主题与上下文展示。
- 其余按钮/切换类组件用于导航、刷新、全屏、语言等基础交互。

## 重点文件

- `grid/GridLayoutPlus.vue`：网格布局对外组件，负责把布局数据、配置和事件统一传递给核心渲染层。
- `grid/gridLayoutPlusIndex.ts`：Grid Layout Plus 对外导出入口。
- `grid/hooks/useGridLayoutPlus.ts`：增强网格状态、历史、响应式与导入导出能力的组合式逻辑。
- `grid/errorHandler.ts`：网格错误类型、安全执行和错误收集的公共入口。
- `grid/components/GridCore.vue`：对 `grid-layout-plus` 第三方组件的核心适配层。

## 审查建议

- 修改 `grid/` 前先确认事件名、插槽参数和导出类型是否仍兼容既有页面。
- 对布局算法、响应式断点、错误处理或历史记录的改动，应补充或更新相邻测试。
- 避免在通用组件中引入具体业务接口或页面状态；如必须接入业务数据，优先通过 props、slots 或外部适配层传入。
- 网格组件存在第三方依赖边界，升级 `grid-layout-plus` 时需要同时验证拖拽、缩放、断点切换和只读模式。
