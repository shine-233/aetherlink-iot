# hooks 组合式函数源码

## 目录定位
本目录提供 `@aetherlink/hooks` 的 Vue 组合式函数，封装常用布尔状态、加载状态、上下文注入、SVG 图标渲染和请求状态管理。

## 主要文件
- `index.ts`：统一导出当前公开 hooks。
- `use-boolean.ts`：提供布尔状态和置真/置假/切换方法。
- `use-loading.ts`：基于 `useBoolean` 封装加载状态语义。
- `use-context.ts`：封装 Vue `provide/inject` 上下文传递。
- `use-request.ts`：基于 `@aetherlink/axios` 管理请求数据、错误和加载状态。
- `use-svg-icon-render.ts`：生成 SVG 图标渲染函数配置。

## 依赖关系
依赖 `vue`，其中 `use-request.ts` 依赖 `@aetherlink/axios`，`use-loading.ts` 依赖本目录的 `use-boolean.ts`。

## 审查发现
目录提供的是低层共享 hooks，当前实现较轻，但缺少中文文件头和目录说明。`use-request.ts` 的状态和请求实例创建职责相对集中，是后续测试重点。

## 重构建议
后续可为 `use-request.ts` 增加请求生命周期测试，并评估是否把实例创建和状态管理拆成两个内部函数。

## 验证建议
优先执行 `pnpm exec eslint packages/hooks/src --ext .ts`。行为变更时建议补充 Vue hook 单测，覆盖初始状态、成功响应、失败响应和 loading 收尾。
