# Data Architecture Device Selectors

## 目录职责

维护设备、指标、属性和统一设备配置选择组件，为 HTTP 参数和动态参数编辑流程生成设备相关参数。

## 文件关系

- `DeviceParameterSelector.vue` 编排不同选择模式，是设备参数选择的主入口。
- `DeviceIdSelector.vue`、`DeviceMetricSelector.vue` 和 `UnifiedDeviceConfigSelector.vue` 负责不同复杂度的选择表单。
- 选择结果会进入 `utils/device-parameter-generator.ts` 和 `types/device-parameter-group.ts` 定义的参数组结构。

## 重点文件

- `UnifiedDeviceConfigSelector.vue`：统一设备参数选择和去重逻辑。
- `DeviceParameterSelector.vue`：设备选择模式编排入口。
- `DeviceSelectionModeChooser.vue`：选择模式展示和切换入口。

## 审查建议

- 修改选择结果字段时，同步检查动态参数编辑器、HTTP 配置和设备参数生成器。
- 设备 ID、指标和属性参数不能重复生成或丢失角色信息。
- 组件测试应覆盖加载失败、空列表、模式切换和已有选择回显。
