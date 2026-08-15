# materials 类型定义

## 目录定位
本目录集中维护 `@aetherlink/materials` 组件共享类型，目前主要服务 AdminLayout 布局组件。

## 主要文件
- `index.ts`：定义 AdminLayout 的页头、页签、侧栏、内容、页脚、CSS 变量和 props 类型。

## 依赖关系
当前无运行时依赖，类型被 `libs/admin-layout` 引用。类型变更会影响布局组件调用方的 props 约束。

## 审查发现
类型字段较完整，但注释以英文为主，缺少目录说明；后续新增组件类型时容易把不同组件契约混放。

## 重构建议
如果 materials 组件继续增加，建议按组件拆分类型文件，再由 `index.ts` 聚合导出，降低单文件膨胀。

## 验证建议
优先执行 `pnpm exec eslint packages/materials/src/types --ext .ts`。类型变更后建议运行相关组件的类型检查或最小使用样例。
