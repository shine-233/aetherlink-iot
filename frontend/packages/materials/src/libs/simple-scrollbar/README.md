# SimpleScrollbar 滚动条组件

## 目录定位
本目录提供 `@aetherlink/materials` 的简易滚动容器组件，是对 `simplebar-vue` 的薄封装。

## 主要文件
- `index.vue`：引入 `simplebar-vue` 和样式，并透传默认插槽。
- `index.ts`：导出组件默认入口。

## 依赖关系
依赖 `simplebar-vue` 及其默认 CSS。上层通过 materials 包入口或本目录入口使用。

## 审查发现
当前实现是低风险薄封装，缺少目录级说明和文件头，后续维护者不易判断这里是否承载额外滚动行为。

## 重构建议
如需扩展滚动事件、尺寸监听或主题样式，建议先明确组件 props 和事件契约，避免把业务滚动逻辑直接塞入薄封装。

## 验证建议
优先执行 `pnpm exec eslint packages/materials/src/libs/simple-scrollbar --ext .ts,.vue`。行为变更时手动检查内容溢出、滚动条显示和插槽渲染。
