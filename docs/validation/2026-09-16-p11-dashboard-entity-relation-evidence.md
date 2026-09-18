# P1.1 通用 Entity Relations 图谱在看板端集成 运行期证据

> 日期：2026-09-16
> 对标基准：ThingsBoard（按实体关系动态关联数据源机制 Entity from relations）
> 状态：**已全面闭环 (done)**
> 质量判定：`npm run typecheck` **0 错误**；前端可视化与看板全量单元测试 **26 files / 289 tests 100% 全部通过**；自动化 API 契约测试（`46_entity_relations.test.js`）**26/26 100% 全部通过**。

---

## 一、交付物全景

### 1. 纯计算与图谱拓扑解析引擎（entity-relation）
- `frontend/src/components/local-visualization-viewer/entity-relation/types.ts`：
  - 定义了 `EntityRelationSourceConfig`（`enabled`, `rootType`, `rootId`, `direction`, `relationType`, `targetType`, `targetKey`, `aggregation`）；
  - 定义了 `EntityRelationEdge`、`ResolvedTargetValue`、`ResolveEntityRelationResult` 等标准接口。
- `frontend/src/components/local-visualization-viewer/entity-relation/resolver.ts`：
  - `isEntityRelationConfigured`：配置完备性防守校验；
  - `generateRelationFieldKey`：确定性动态字段键生成器，将实体关系配置映射为安全字段键名；
  - `filterRelationTargetIds`：纯函数拓扑边过滤，精准支持正向（起点→目标）与反向（目标→起点）遍历，自动去重并保序；
  - `aggregateNumericValues`：多实体遥测数值聚合（支持 `latest`、`avg`、`sum`、`max`、`min`、`count` 六种策略）；
  - `resolveEntityRelationValue`：综合解析函数，结合关系边和遥测字典输出解析结果与标量/序列数据。
- `frontend/src/components/local-visualization-viewer/entity-relation/__tests__/resolver.test.ts`：
  - 覆盖方向过滤、类型过滤、无效值防守、多策略数值聚合等 18 项单测，全部通过。

### 2. 渲染白名单与防注入归一化（normalizer & data）
- `frontend/src/components/local-visualization-viewer/types.ts`：
  - 在 `TextWidgetConfig`、`MetricWidgetConfig`、`ChartWidgetConfig` 及 `NormalizedLocalWidget` 中接入可选的 `entityRelation` 结构。
- `frontend/src/components/local-visualization-viewer/normalizer.ts`：
  - 严守 Fail-Closed 安全准则，属性键名天然避开 `FORBIDDEN_KEY`；
  - `FIELD_PATTERN` 安全放行领先下划线字段键名；
  - `normalizeEntityRelation`：深度校验实体类型白名单、拓扑方向及聚合策略，返回只读冻结配置；
  - `normalizeConfig`：小部件配置层在启用实体关系时，自动推导并填充目标字段键名；
  - `normalizeWidget`：支持小部件顶层与配置层的实体关系透传归一化。
- `frontend/src/components/local-visualization-viewer/data.ts`：
  - 在 `resolveText`、`resolveMetric`、`chartData` 中原生支持实体关系动态字段键的自动回退与解析。
- `frontend/src/components/local-visualization-viewer/normalizer.test.ts`：
  - 补充 26 组小部件归一化单测（覆盖文本/指标/图表实体关系、非法类型防御、非法聚合防御等），全部通过。

### 3. 动态表单配置交互增强（dynamic-form）
- `frontend/src/components/local-visualization-viewer/dynamic-form/types.ts`：
  - `DynamicWidgetFormData` 扩充 `entityRelation?: EntityRelationSourceConfig`。
- `frontend/src/components/local-visualization-viewer/dynamic-form/form-schema.ts`：
  - `validateWidgetForm`：新增实体关系表单校验（起点 ID、关系类型、目标 Key 必填性），在启用实体关系时对指标字段实现免手动录入豁免；
  - `convertFormToWidgetConfig` / `convertWidgetConfigToForm`：支持双向模型转换与深拷贝保全。
- `frontend/src/components/local-visualization-viewer/dynamic-form/DynamicWidgetForm.vue`：
  - 新增 "实体关系数据源" 独立配置抽屉 Tab；
  - 提供启用开关、起点实体类型（设备/资产/网关/客户）、起点 ID、拓扑方向（起点→目标 / 目标→起点）、常用关系类型预设与自由输入、目标实体类型、目标遥测 Key 与六种聚合策略选择。
- `frontend/src/components/local-visualization-viewer/dynamic-form/__tests__/form-schema.test.ts`：
  - 补充实体关系动态表单校验与双向转换测试，全部通过。

### 4. 看板编辑器模型保全（editor-model）
- `frontend/src/views/visualization/native-board-editor/editor-model.ts`：
  - `canonicalConfig`：在保存清洗阶段完整保留 `entityRelation`；
  - `canonicalDashboard`：在保存门禁中放行启用了实体关系的动态绑定图表（区别于老式的纯静态数据图表限制）；
  - `EditorWidget`：接口显式声明 `entityRelation`。
- `frontend/src/views/visualization/native-board-editor/editor-model.test.ts`：
  - 补充实体关系配置跨加载、编辑、保存序列化全流程往返（roundtrip）保全测试，全部通过。

### 5. 看板端数据自动加载器与呈现层集成（useEntityRelationDataLoader）
- `frontend/src/views/visualization/native-board/useEntityRelationDataLoader.ts`：
  - 自动扫描看板中所有启用了 `entityRelation` 的小部件；
  - 按 `(rootType, rootId, direction, relationType)` 进行边查询并发合并（调用 `listEntityRelations` API）；
  - 自动提取目标实体 ID 并批量拉取实时遥测（调用 `telemetryDataCurrent` API）；
  - 经由 `resolveEntityRelationValue` 计算出标量与图表序列数组，注入到响应式 `fields` 对象中；
  - 包含网络异常软兜底保护与基于 `getCurrentInstance()` 的生命周期定时器安全注销。
- `frontend/src/views/visualization/native-board/index.vue`：
  - 挂载 `useEntityRelationDataLoader`，将动态计算的 `dynamicFields` 传递给 `<LocalVisualizationViewer :fields="dynamicFields" />`，实现看板实时动态渲染。
- `frontend/src/views/visualization/native-board/__tests__/useEntityRelationDataLoader.test.ts`：
  - 4 项单测（空配置兜底、指标聚合加载、图表序列加载、接口异常韧性），全部通过。

---

## 二、测试验证矩阵

### 1. 前端类型检查
- 命令：`npm run typecheck`
- 结果：**0 errors, exit code 0**

### 2. 前端单元与集成测试（全量看板相关）
- 命令：`npx vitest run src/components/local-visualization-viewer src/views/visualization`
- 结果：**26 test files passed, 289 tests passed (100%)**
  - `entity-relation/__tests__/resolver.test.ts`：18 passed
  - `dynamic-form/__tests__/form-schema.test.ts`：6 passed
  - `normalizer.test.ts`：26 passed
  - `data.test.ts`：6 passed
  - `native-board-editor/editor-model.test.ts`：16 passed
  - `native-board/__tests__/useEntityRelationDataLoader.test.ts`：4 passed
  - `native-board/__tests__/index.test.ts`：12 passed

### 3. API 契约与多租户拓扑全量验证（46_entity_relations）
- 命令：`node run_tests.js -m 46_entity_relations`
- 结果：**26/26 passed, 耗时 1.15s, passRate 100%**
  - 覆盖参数边界、白名单校验、自环防护、有向图隔离、幂等性、级联保护/级联删除策略、跨租户严密隔离等。

---

## 三、结论
通用 Entity Relations 图谱在看板端的集成已在数据模型、拓扑解析、安全归一化、动态表单配置、编辑器保全及运行时加载器六个层面完成端到端闭环，全面对标 ThingsBoard `Entity from relations` 机制。质量门禁 100% 达标，具备完全结案条件。
