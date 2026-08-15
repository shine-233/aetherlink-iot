# grid/hooks 组件说明

## 目录职责

本目录保存网格布局相关组合式逻辑，负责把布局数据、响应式断点、性能配置、历史记录和虚拟化能力拆分成可复用的状态模块。

## 文件关系

- `useGridLayoutPlus.ts` 是增强版聚合入口，调用校验、布局、历史、响应式和性能工具。
- `useGridCore.ts` 聚合网格核心状态和基础事件处理。
- `useGridResponsive.ts`、`useGridPerformance.ts`、`useGridHistory.ts` 分别负责断点、性能和撤销恢复。
- `index.ts` 汇总导出 hooks，减少上层 import 路径分散。

## 重点文件

- `useGridLayoutPlus.ts`：当前增强网格最重要的组合式入口。
- `useGridCore.ts`：基础布局状态和事件处理集中点。
- `useGridHistory.ts`：撤销/重做语义，需要与布局变更节流策略配合。
- `useGridResponsive.ts`：断点映射和布局转换边界。

## 审查建议

- 审查 hooks 时重点看响应式引用是否会意外共享、watch 是否可能产生循环更新。
- 布局变更应明确区分用户交互、程序化导入和断点转换，避免历史记录污染。
- 性能优化参数应与渲染层事件频率一起验证，不能只看单个 hook。
