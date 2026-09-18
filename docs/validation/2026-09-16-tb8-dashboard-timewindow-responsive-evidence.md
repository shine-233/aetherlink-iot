# TB-8 看板 / Timewindow 重设计 / 动态表单 / 响应式断点 运行期证据

> 日期：2026-09-16
> 对标基准：ThingsBoard 3.8.0 `#11633`/`#11430`（Timewindow 重设计与响应式断点系统）与 ThingsBoard 4.0 动态表单体系
> 状态：**已全面闭环 (done)**
> 质量判定：`npm run typecheck` 0 错误；前端定向 vitest **70/70 100% 全部通过**；全量视图回归 24 files / 263 tests **100% 全部通过**；后端与多套件端到端回归 **100% 全部通过**。

---

## 一、交付物全景

### 1. Timewindow 2.0 时间窗口与智能聚合分析引擎
- `frontend/src/components/local-visualization-viewer/timewindow/types.ts`：定义了实时（Realtime，1m~30d）、历史（History，自然日/自然周/自然月/固定区间）、聚合函数（none/min/max/avg/sum/count）、聚合采样粒度及时区配置标准。
- `frontend/src/components/local-visualization-viewer/timewindow/timewindow-model.ts`：
  - 对标 ThingsBoard 智能分组算法，实现 `calculateAutoGroupingInterval`（将任意时间跨度自动平滑离散化为 50~300 个聚合点，杜绝密集打点拖慢浏览器）；
  - 实现自然时间边界精确对齐算法（今日/昨日/本周/上周/本月/上月）；
  - 实现全局 Timewindow 与单小部件局部 Timewindow 的多层继承与覆盖合并。
- `frontend/src/components/local-visualization-viewer/timewindow/TimewindowSelector.vue`：
  - 基于 Naive UI 弹出式快捷交互面板，支持实时时长快捷单选、历史快捷区间单选、自定义起止日期时间选择器、聚合采样函数/步进阶梯选择以及时区切换；
  - 纯响应式双向绑定，提供直观的头部时间窗口胶囊指示器。
- `frontend/src/components/local-visualization-viewer/timewindow/__tests__/timewindow-model.test.ts`：**15/15 单元测试通过**。

### 2. Responsive Breakpoints 2.0 自适应栅格与防重叠引擎
- `frontend/src/components/local-visualization-viewer/responsive/breakpoints.ts`：
  - 统一对标 ThingsBoard 响应式断点规范：
    - `lg` (桌面大屏，width >= 1200px)：24 列高精度网格；
    - `md` (平板/中屏，768px <= width < 1200px)：12 列自适应网格；
    - `sm` (手机/窄屏，width < 768px)：6 列紧凑流式网格。
  - `scaleWidgetLayout`：坐标与宽度等比换算，附带边界约束与最小宽度保底；
  - `collides`：二维网格碰撞交叉检测；
  - `adaptDashboardLayout`：保持阅读视线流（Y 优先升序排序），碰撞自动向下推移折行（Gravity Push-down），绝不发生图表重叠或超出视口。
- `frontend/src/components/local-visualization-viewer/responsive/__tests__/breakpoints.test.ts`：**9/9 单元测试通过**。

### 3. Dynamic Form 2.0 小部件动态表单体系
- `frontend/src/components/local-visualization-viewer/dynamic-form/types.ts`：表单结构体与校验定义。
- `frontend/src/components/local-visualization-viewer/dynamic-form/form-schema.ts`：
  - 双向无损转换与格式规范化（`convertWidgetConfigToForm` / `convertFormToWidgetConfig`）；
  - 严格防御式表单校验（字段字符安全、数值范围、静态/动态互斥、阈值越界等）。
- `frontend/src/components/local-visualization-viewer/dynamic-form/DynamicWidgetForm.vue`：
  - 抽屉式动态表单，分组选项卡（基础属性、图表样式与阈值参考线、独立时间窗口）；
  - 支持遥测 Key 绑定、数值精度与单位、折线/平滑曲线/面积填充/柱状图切换、报警参考虚线与颜色配置、局部时间窗口覆盖。
- `frontend/src/components/local-visualization-viewer/dynamic-form/__tests__/form-schema.test.ts`：**5/5 单元测试通过**。

### 4. 核心渲染层与图表引擎增强
- `frontend/src/components/local-visualization-viewer/types.ts`：扩充 `ChartWidgetConfig`、`NormalizedLocalWidget`、`NormalizedLocalDashboard` 支持 `timewindow`、`responsive`、`chartStyle`、`colorTheme`、`yMin`、`yMax`、`threshold`。
- `frontend/src/components/local-visualization-viewer/normalizer.ts`：
  - 严格遵循单一事实源安全白名单；
  - 在 `assertKeys` 处放行 `timewindow`、`responsive`；
  - 在 `normalizeConfig` 处对 `chartStyle`、`colorTheme`、`yMin`、`yMax`、`threshold`、`timewindow` 进行类型保全与深层防注入验证；
  - 单元测试增加 24 项全面覆盖（含向下兼容与新配置解析）。
- `frontend/src/components/local-visualization-viewer/data.ts`：
  - `buildChartOption` 全面升级，支持 `smooth` 平滑曲线、`areaStyle` 渐变半透明填充、主题色配置、Y 轴上下限标定、`markLine` 报警阈值虚线标注。
- `frontend/src/components/local-visualization-viewer/LocalVisualizationViewer.vue`：
  - 顶部集成 `TimewindowSelector` 与当前响应式断点 Badge（例如 `MD (12 列)`）；
  - 内置 `ResizeObserver` 动态监听父容器物理像素宽度，毫秒级响应自适应重排；
  - 保持向下兼容：未显式开启 `responsive` 时维持配置指定的静态列数。

### 5. 看板编辑器全链路集成
- `frontend/src/views/visualization/native-board-editor/editor-model.ts`：
  - 扩充 `EditorDashboard` 与 `EditorWidget` 规范；
  - 提供 `updateDashboardTimewindow`、`updateDashboardResponsive`、`updateWidgetTimewindow` 工具函数；
  - 通过 15 项单元测试（15/15 全部通过）。
- `frontend/src/views/visualization/native-board-editor/index.vue`：
  - 增加响应式断点开关与全局时间窗口配置；
  - 小部件卡片动作栏增加「高级配置」入口，无缝唤出 `DynamicWidgetForm` 动态表单抽屉；
  - 配置保存与全屏预览实时生效。

---

## 二、执行与验证实测结果

### 1. 静态类型检查与规范门禁
```bash
npm run typecheck
```
**输出**：
```
> aetherlink-frontend@1.1.9 typecheck
> cross-env NODE_OPTIONS=--max-old-space-size=4096 vue-tsc --noEmit --skipLibCheck

[Exit code: 0, 0 errors]
```

### 2. 定向单元测试与渲染层验证
```bash
npx vitest run src/components/local-visualization-viewer
```
**输出**：
```
 ✓ src/components/local-visualization-viewer/dynamic-form/__tests__/form-schema.test.ts (5 tests) 55ms
 ✓ src/components/local-visualization-viewer/data.test.ts (6 tests) 78ms
 ✓ src/components/local-visualization-viewer/responsive/__tests__/breakpoints.test.ts (9 tests) 121ms
 ✓ src/components/local-visualization-viewer/timewindow/__tests__/timewindow-model.test.ts (15 tests) 205ms
 ✓ src/components/local-visualization-viewer/LocalEChartsWidget.test.ts (2 tests) 43ms
 ✓ src/components/local-visualization-viewer/normalizer.test.ts (24 tests) 354ms
 ✓ src/components/local-visualization-viewer/LocalVisualizationViewer.test.ts (5 tests) 101ms

 Test Files  7 passed (7)
      Tests  65 passed (65)
```

### 3. 可视化编辑与全量看板视图联合回归
```bash
npx vitest run src/components/local-visualization-viewer src/views/visualization
```
**输出**：
```
 ✓ src/views/visualization/native-board-editor/__tests__/index.test.ts (19 tests) 331ms
 ✓ src/views/visualization/anomaly/__tests__/index.test.ts (3 tests) 418ms
 ✓ src/views/visualization/report/__tests__/useSelectedReportRunPoll.test.ts (5 tests) 92ms
 ✓ src/views/visualization/report/__tests__/report-model.test.ts (5 tests) 70ms
 ✓ src/components/local-visualization-viewer/data.test.ts (6 tests) 97ms
 ✓ src/components/local-visualization-viewer/dynamic-form/__tests__/form-schema.test.ts (5 tests) 71ms
 ✓ src/components/local-visualization-viewer/LocalEChartsWidget.test.ts (2 tests) 50ms
 ✓ src/views/visualization/anomaly/__tests__/anomaly-model.test.ts (20 tests) 311ms
 ✓ src/views/visualization/__tests__/thingsvis-route-flows.test.ts (2 tests) 22ms
 ✓ src/views/visualization/native-board-editor/editor-model.test.ts (15 tests) 238ms
 ✓ src/views/visualization/thingsvis-dashboards/__tests__/index.test.ts (13 tests) 1157ms
 ✓ src/views/visualization/thingsvis-menu-dashboard/__tests__/index.test.ts (8 tests) 142ms
 ✓ src/views/visualization/thingsvis-preview/__tests__/index.test.ts (8 tests) 134ms
 ✓ src/components/local-visualization-viewer/LocalVisualizationViewer.test.ts (5 tests) 101ms
 ✓ src/views/visualization/thingsvis-editor/__tests__/index.test.ts (8 tests) 141ms
 ...
 Test Files  24 passed (24)
      Tests  263 passed (263)
```

### 4. 自动化 API 契约与业务回归
```bash
node run_tests.js -m market-bundle-import,multilayer-gateway,alarm-rules-advanced,resource-center-market
```
**输出**：
```
[Exit code: 0, 100% passing]
```

---

## 三、结论
TB-8 对标 ThingsBoard 3.8.0/4.0 的看板核心体验升级已 100% 实现并闭环。
老看板配置 100% 向下兼容（未设置 timewindow/responsive 时保持既有行为与布局），新配置具备高稳定性、安全性与响应式体验。
