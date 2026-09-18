# TB-11 HTML 容器 Widget：安全净化白名单与渲染闭环证据

> 日期：2026-09-17
> 仓库：`aetherlink-iot`
> 对应路线图：§7.1 TB-11（对标 ThingsBoard 4.3.1.2 `#15556` HTML Card / Custom HTML Widget）
> 验证层级：单元测试 + 15+ XSS 向量对抗测试 + 可视化小部件端到端渲染 + 看板编辑器保存门禁 + vue-tsc 强类型检查
> 证据归档：`src/components/local-visualization-viewer/sanitizer.test.ts`、`src/components/local-visualization-viewer/normalizer.test.ts`、`src/components/local-visualization-viewer/data.test.ts`、`src/components/local-visualization-viewer/LocalVisualizationViewer.test.ts`、`src/views/visualization/native-board-editor/editor-model.test.ts`

---

## 一、能力与安全准则概述

在 IoT 看板与自定义数据大屏中，HTML 容器 Widget 允许用户编写自定义 HTML 排版与 CSS 样式，并动态插值设备遥测或属性。
按照 `ROADMAP.md` 既定原则：**"必须先定 HTML 净化（XSS）策略，否则等于开放一个存储型 XSS 面；安全前置不满足就不做"**。

本交付坚决执行 **Fail-Closed 递归白名单净化策略**，构建了纯原生、零外部 npm 依赖的高性能安全沙箱与样式隔离体系：
1. **标签与结构白名单**：
   - 允许常用安全排版与表格标签：`div`, `span`, `p`, `b`, `strong`, `i`, `em`, `h1`–`h6`, `table`, `thead`, `tbody`, `tr`, `th`, `td`, `ul`, `ol`, `li`, `br`, `hr`, `pre`, `code`, `img`, `a` 等；
   - 绝不放行危险活动与脚本标签：`<script>`, `<iframe>`, `<object>`, `<embed>`, `<applet>`, `<base>`, `<link>`, `<meta>`, `<form>`, `<input>`, `<button>`, `<select>`, `<canvas>`, `<svg>`, `<math>` 等，遇此类标签连同子孙节点彻底剥除。
2. **事件与属性严格清洗**：
   - 无条件剔除所有以 `on` 开头的属性（如 `onclick`, `onerror`, `onload`, `onmouseover` 等）；
   - 仅允许安全白名单属性（`class`, `id`, `style`, `title`, `src`, `href`, `width`, `height`, `alt`, `colspan`, `rowspan` 等）；
   - DOM Clobbering 与原型污染防护：严格拦截 `window`, `document`, `location`, `__proto__`, `constructor` 等关键 ID。
3. **URL 协议安全验证**：
   - 链接与资源地址严格限制为 `http:`, `https:`, `mailto:`, `tel:` 或安全相对路径（`/`, `./`, `../`, `#`, `?`）；
   - 拦截 `javascript:`, `vbscript:`, `data:text/html`；仅允许安全静态位图 base64 格式；
   - `target="_blank"` 链接自动强制注入 `rel="noopener noreferrer"`。
4. **CSS 隔离与净化（Scoped CSS）**：
   - 清洗内联与自定义 CSS 中的 `expression()`, `behavior:`, `url(javascript:)`, `@import`, `@charset`；
   - 支持小部件级 `css`，引擎自动挂载 `[data-widget-id="..."]` 作用域前缀，彻底隔绝向外部页面发生样式污染。
5. **动态遥测插值与二次投毒防御**：
   - 支持 `{{field}}` 与 `${field}` 动态遥测/属性占位符以及 `{{value}}` 主字段绑定；
   - 动态插值完成后统一通过 `sanitizeHtml` 处理，即使遥测数据本身携带恶意代码注入向量，也会被安全清洗，绝无执行可能。
6. **四面一致全链路接入**：
   - 数据模型（`types.ts`、`HtmlWidgetConfig`、`LOCAL_VIEWER_LIMITS.htmlLength = 20000`）；
   - 归一化网关（`normalizer.ts` 支持 `html` / `html-container` / `html-card` 别名解析与长度防御）；
   - 渲染组件（`LocalWidgetRenderer.vue` 挂载 Scoped 样式与安全 HTML）；
   - 动态表单（`DynamicWidgetForm.vue` 与 `form-schema.ts` 提供专属 HTML/CSS 代码编辑抽屉与校验）；
   - 原生看板编辑器（`index.vue`、`editor-model.ts` 支持拖拽新建、配置流转、JSON 序列化保存门禁）。

---

## 二、测试套件与安全向量验证结果

### 2.1 自动化测试矩阵（166 项测试 100% 全绿）

| 模块 | 测试文件 | 测试用例数 | 关键验证点 | 结果 |
| --- | --- | --- | --- | --- |
| 安全核心 | `sanitizer.test.ts` | 17 | 15+ 种 XSS 攻击向量清洗（`<script>`, `onerror`, `javascript:`, SVG/MathML, DOM Clobbering, CSS 表达式, `@import`, Scoped CSS 前缀重写） | **17/17 PASS** |
| 规范化网关 | `normalizer.test.ts` | 30 | `html` 别名映射、配置清洗、超长拦截（20000 字符限制）、阻断 normalize 时的 `javascript:` 伪协议 | **30/30 PASS** |
| 数据插值 | `data.test.ts` | 11 | `{{field}}` / `${field}` 变量插值、`field` 缺失 fallback 兜底、遥测数据二次投毒 XSS 清洗 | **11/11 PASS** |
| 渲染器组件 | `LocalVisualizationViewer.test.ts` | 6 | 真实 Vue 容器内挂载 HTML 小部件、动态数据绑定渲染、Scoped CSS 隔离类挂载 | **6/6 PASS** |
| 表单校验 | `form-schema.test.ts` | 8 | HTML 小部件空内容校验、表单模型与配置双向无损转换 | **8/8 PASS** |
| 编辑器模型 | `editor-model.test.ts` | 16 | HTML 小部件添加（w=8, h=4 默认规格）、保存门禁、JSON 序列化与反序列化回路 | **16/16 PASS** |
| 编辑器交互 | `__tests__/index.test.ts` | 19 | 编辑器面板小部件类型选择、增删与布局持久化 | **19/19 PASS** |
| 既有能力保护 | `converter.test.ts`、`resolver.test.ts`、`breakpoints.test.ts` 等 | 59 | 既有单位换算、实体拓扑关联、响应式断点回归 | **59/59 PASS** |

### 2.2 运行期命令输出

```bash
vitest run "src/components/local-visualization-viewer" "src/views/visualization/native-board-editor"

 ✓ src/components/local-visualization-viewer/dynamic-form/__tests__/form-schema.test.ts (8 tests) 127ms
 ✓ src/components/local-visualization-viewer/responsive/__tests__/breakpoints.test.ts (9 tests) 128ms
 ✓ src/components/local-visualization-viewer/data.test.ts (11 tests) 149ms
 ✓ src/components/local-visualization-viewer/units/converter.test.ts (15 tests) 190ms
 ✓ src/components/local-visualization-viewer/timewindow/__tests__/timewindow-model.test.ts (15 tests) 205ms
 ✓ src/components/local-visualization-viewer/sanitizer.test.ts (17 tests) 252ms
 ✓ src/components/local-visualization-viewer/entity-relation/__tests__/resolver.test.ts (18 tests) 268ms
 ✓ src/components/local-visualization-viewer/normalizer.test.ts (30 tests) 422ms
 ✓ src/components/local-visualization-viewer/LocalEChartsWidget.test.ts (2 tests) 40ms
 ✓ src/views/visualization/native-board-editor/__tests__/index.test.ts (19 tests) 305ms
 ✓ src/components/local-visualization-viewer/LocalVisualizationViewer.test.ts (6 tests) 131ms
 ✓ src/views/visualization/native-board-editor/editor-model.test.ts (16 tests) 247ms

 Test Files  12 passed (12)
      Tests  166 passed (166)
   Duration  7.37s
```

### 2.3 严格 TypeScript 类型检查

```bash
pnpm run typecheck
> cross-env NODE_OPTIONS=--max-old-space-size=4096 vue-tsc --noEmit --skipLibCheck

# Exited with code 0 (0 errors)
```

---

## 三、结案结论

TB-11（HTML 容器 Widget）安全净化前置准则完全落实，前端渲染、配置、动态表单与编辑器全链路 100% 贯通，测试全绿，类型安全，符合生产交付标准。
ROADMAP.md 中 TB-11 状态正式更新为 **`已闭环`**。
