# Common Data Architecture Components

## 目录职责

维护数据架构配置表单中复用度最高的通用组件，包括动态参数编辑、HTTP 配置步骤、组件属性选择、配置导入导出区和设备参数补充入口。

## 文件关系

- `DynamicParameterEditor.vue` 是动态参数编辑的中心，供 HTTP 请求头、查询参数、路径参数等步骤复用；当前已将添加入口配置、新增参数、参数行列表、设备选择转换、设备参数组、设备配置提交计划、模板切换和接口模板导入纯逻辑拆出，并把添加参数抽屉 UI 编排拆到 `DynamicParameterAddDrawer.vue`、单行参数 UI 拆到 `DynamicParameterInlineRow.vue`，组件主要保留列表状态、业务 mutation、抽屉挂载、日志和 emit 同步。
- `HttpConfigStep1.vue` 到 `HttpConfigStep4.vue` 组成 HTTP 配置分步流程，通常由 `components/modals/HttpConfigForm.vue` 编排。
- `ConfigurationImportExportView.vue` 调用 `utils/ConfigurationImportExport.ts`，不应自行实现 schema 规则。
- `templates/` 提供参数值模板注册能力。

## 重点文件

- `DynamicParameterEditor.vue`：多模式参数编辑和校验的核心组件。
- `DynamicParameterAddDrawer.vue`：新增参数抽屉，封装手动、属性绑定和设备配置三种新增参数表单。
- `DynamicParameterInlineRow.vue`：参数行展示和事件壳，负责渲染启用、key、模板、value 和操作按钮，并向父组件发出更新、删除、模板切换和设备组编辑事件。
- `dynamicParameterEditorState.ts`：动态参数编辑器的纯状态辅助层，维护新增参数配置、稳定 ID、设备参数 key 和动态绑定推断。
- `dynamicParameterEditorAddOptions.ts`：添加入口配置模块，维护添加参数下拉选项、接口模板入口、推荐模板读取和添加入口动作计划。
- `dynamicParameterEditorNewParam.ts`：新增参数流程的纯逻辑模块，维护默认参数、key 校验、手动/属性/设备模式填充和快捷新增预设。
- `dynamicParameterEditorParameterList.ts`：参数列表行级操作模块，维护参数增删改、key/value 更新、重复 key 校验、删除后编辑索引计划、追加后聚焦索引和渲染前稳定 ID 补齐。
- `dynamicParameterEditorDeviceSelection.ts`：设备选择结果转换模块，维护从设备输入生成参数、槽位限制和设备 dispatch 模板字段映射。
- `dynamicParameterEditorDeviceGroup.ts`：设备参数组的纯逻辑模块，维护参数列表替换/去重、统一设备配置提交计划、设备配置生成参数提交计划、设备参数组替换/删除提交计划、设备组标签和编辑回显预设。
- `dynamicParameterEditorTemplate.ts`：参数模板切换的纯逻辑模块，维护模板默认值、动态/静态元数据补齐、模板变化动作计划、下拉模板选项和自定义输入能力判断。
- `dynamicParameterEditorTemplateImport.ts`：接口模板导入模块，维护默认占位参数、当前接口参数合并和导入后聚焦索引计算。
- `HttpConfigStep1.vue` - `HttpConfigStep4.vue`：HTTP 配置向导的主要步骤。
- `ConfigurationImportExportView.vue`：配置导入导出的 UI 入口。

## 审查建议

- 审查时重点看参数字段、默认值、emit 结构和导入导出工具是否保持一致。
- 修改 HTTP 步骤前要确认路径参数、查询参数、请求头和脚本上下文的执行器兼容性。
- `DynamicParameterEditor.vue` 体量仍然较大，后续重构应继续围绕设备参数分组抽屉和模板切换区逐步切分。
- 已先把无 UI 副作用的参数状态 helper 抽到 `dynamicParameterEditorState.ts`，把添加入口配置和动作计划抽到 `dynamicParameterEditorAddOptions.ts`，把新增参数创建/校验 helper 抽到 `dynamicParameterEditorNewParam.ts`，把参数行增删改、key 校验和编辑/聚焦计划抽到 `dynamicParameterEditorParameterList.ts`，把设备选择结果转换抽到 `dynamicParameterEditorDeviceSelection.ts`，把设备参数组计算、设备配置提交计划和设备参数组替换/删除提交计划抽到 `dynamicParameterEditorDeviceGroup.ts`，把模板切换动作计划抽到 `dynamicParameterEditorTemplate.ts`，把接口模板导入结果计算抽到 `dynamicParameterEditorTemplateImport.ts`，并把行级 UI 事件壳抽到 `DynamicParameterInlineRow.vue`；修改时要避免把 message、emit、nextTick、logger 等 UI 副作用重新塞回纯逻辑模块。
