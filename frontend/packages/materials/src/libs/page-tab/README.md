# PageTab 页签组件

## 目录职责

本目录提供材料库中的 `PageTab` 页签组件，用于在面板、页面容器或嵌入式材料中展示可关闭的标签页。组件支持 Chrome 风格页签和按钮风格页签两种模式，并通过 CSS 变量统一控制激活色。

## 文件关系

- `index.ts`：组件导出入口，对外默认导出 `PageTab`。
- `index.vue`：页签总装组件，根据 `mode` 选择 `ChromeTab` 或 `ButtonTab`，并统一处理插槽、关闭事件和主题色变量。
- `shared.ts`：主题色 CSS 变量生成工具，把传入的主色转换为 hover、暗色和透明度状态。
- `chrome-tab.vue`：Chrome 风格页签外壳，组合背景 SVG、内容插槽和分割线。
- `chrome-tab-bg.vue`：Chrome 风格页签背景 SVG，提供左右镜像的标签形状。
- `button-tab.vue`：按钮风格页签外壳，提供紧凑边框样式。
- `svg-close.vue`：关闭按钮图标组件，封装 click 事件并阻止冒泡。
- `index.module.css`：两种页签模式的状态样式、暗色模式样式和关闭按钮 hover 样式。
- `index.module.css.d.ts`：CSS Modules 类型声明，保证 TypeScript 能识别样式类名。

## 重要参数

- `mode`：决定渲染 `chrome` 或 `button` 样式。
- `activeColor`：激活态主题色，会被 `shared.ts` 转换为多个 CSS 变量。
- `active`：控制页签激活样式。
- `darkMode`：控制暗色模式样式。
- `closable`：控制默认关闭按钮是否显示。
- `prefix`、`default`、`suffix`：分别用于左侧内容、主体内容和右侧内容；如果传入 `suffix`，会覆盖默认关闭按钮。

## 审查建议

- 问题描述：插槽类型和 `PageTabProps` 被多个子组件重复声明，后续扩展时容易出现字段或插槽说明不同步。
- 改进方案：把插槽类型抽到共享类型文件，或让子组件只接收必要 props，降低重复声明。
- 实施步骤：先在 `frontend/packages/materials/src/types` 增加统一插槽类型；再替换 `index.vue`、`chrome-tab.vue`、`button-tab.vue` 的本地类型；最后运行材料库类型检查和相关组件测试。
- 预期效果：减少重复代码，提高后续新增页签模式或插槽时的可维护性。

## 使用注意

- `activeColor` 应传入合法颜色值，否则颜色转换工具可能生成不可预期的 CSS 变量。
- `svg-close.vue` 内部使用 `@click.stop`，父组件需要监听 `PageTab` 的 `close` 事件，而不是依赖外层点击冒泡。
- 修改 `index.module.css` 的类名时必须同步更新 `index.module.css.d.ts`，否则 TypeScript 会出现样式类型不一致。
