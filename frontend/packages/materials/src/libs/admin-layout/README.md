# AdminLayout 布局组件

## 目录定位
本目录提供 `@aetherlink/materials` 中的后台布局组件，负责页头、页签、侧边栏、移动端遮罩、内容区和页脚的基础布局结构。

## 主要文件
- `index.vue`：AdminLayout 的 Vue 单文件组件入口。
- `index.ts`：导出组件和布局常量。
- `shared.ts`：维护滚动容器 ID、最大层级和 CSS 变量生成逻辑。
- `index.module.css`：布局样式和响应式/状态类。
- `index.module.css.d.ts`：CSS Module 类型声明。

## 依赖关系
依赖 Vue、同包 `../../types` 的布局类型，以及本目录 CSS Module。外部调用方通过 `index.ts` 使用组件与常量。

## 审查发现
布局结构清晰，但样式类、类型和 CSS 变量之间缺少中文说明；`index.module.css` 原有文件头仅为不明确的 `@type` 注释，已作为损坏注释处理。

## 重构建议
后续可把布局区域的插槽契约补进组件文档，并为 CSS 变量生成增加快照或类型驱动测试。

## 验证建议
优先执行 `pnpm exec eslint packages/materials/src/libs/admin-layout --ext .ts,.vue`。样式变更建议在实际页面中检查桌面端、移动端侧栏和折叠状态。
