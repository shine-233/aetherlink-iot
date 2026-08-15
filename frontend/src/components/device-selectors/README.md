# 设备选择器组件

## 概述

这个文件夹包含了两个专门用于设备指标选择的组件：

- **DeviceMetricsSelector** - 通用的设备指标选择器
- **DeviceDispatchSelector** - 专门用于调度数据的设备指标选择器

## 组件特性

### 🎯 核心功能
- **设备选择** - 支持设备列表筛选和搜索
- **指标选择** - 按数据源类型分组的指标显示
- **自动加载** - 组件挂载时自动加载设备列表
- **智能过滤** - 根据数据类型自动过滤指标
- **完整国际化** - 支持中英文界面

### 🔧 技术特点
- **TypeScript** - 完整的类型支持
- **Vue 3 Composition API** - 现代化的Vue语法
- **Naive UI** - 统一的UI组件库
- **响应式设计** - 支持双向数据绑定
- **API适配** - 正确适配后端API数据结构

## 最近修复

### ✅ 已修复问题
1. **指标下拉无数据** - 修复了API数据结构适配问题
2. **国际化不完整** - 添加了完整的国际化翻译
3. **设备自动加载** - 确保组件挂载时自动加载设备列表
4. **功能精简** - 移除了不必要的输入框，专注核心功能
5. **JavaScript错误** - 修复了`undefined`的`forEach`调用错误

### 🔧 技术改进
- 正确适配API返回的数据结构：`[{ data_source_type, options: [{ key, label, data_type }] }]`
- 完善了指标选择逻辑，支持分组显示
- 添加了完整的错误处理和调试信息
- 增强了数据验证，防止`undefined`和`null`值导致的错误
- 改进了API响应数据的类型检查

## 组件对比

| 特性 | DeviceMetricsSelector | DeviceDispatchSelector |
|------|----------------------|----------------------|
| 用途 | 通用设备指标选择 | 调度数据专用 |
| 数据类型选择 | ❌ | ✅ |
| 指标名称输入 | ✅ | ❌ |
| 聚合函数选择 | ✅ | ❌ |
| 发送数据输入 | ❌ | ❌ |

## 使用方法

### DeviceMetricsSelector

```vue
<template>
  <DeviceMetricsSelector
    v-model="deviceMetrics"
    :device-options="deviceOptions"
    :show-metrics-name="true"
    :show-aggregate-function="true"
    @device-change="onDeviceChange"
    @metrics-change="onMetricsChange"
  />
</template>

<script setup>
import DeviceMetricsSelector from '@/components/device-selectors/DeviceMetricsSelector.vue'

const deviceMetrics = ref({
  deviceId: '',
  metricsId: '',
  metricsName: '',
  aggregateFunction: ''
})
</script>
```

### DeviceDispatchSelector

```vue
<template>
  <DeviceDispatchSelector
    v-model="dispatchConfig"
    :device-options="deviceOptions"
    @device-change="onDeviceChange"
    @data-type-change="onDataTypeChange"
    @metrics-change="onMetricsChange"
  />
</template>

<script setup>
import DeviceDispatchSelector from '@/components/device-selectors/DeviceDispatchSelector.vue'

const dispatchConfig = ref({
  deviceId: '',
  deviceName: '',
  dataType: '',
  metricsId: '',
  metricsName: ''
})
</script>
```

## 数据格式

### DeviceMetricsSelector 数据格式

```typescript
interface DeviceMetricsValue {
  deviceId?: string        // 设备ID
  metricsId?: string       // 指标ID
  metricsName?: string     // 指标名称
  aggregateFunction?: string // 聚合函数
}
```

### DeviceDispatchSelector 数据格式

```typescript
interface DeviceDispatchValue {
  deviceId?: string        // 设备ID
  deviceName?: string      // 设备名称
  dataType?: string        // 数据类型 (attributes/telemetry/command)
  metricsId?: string       // 指标ID
  metricsName?: string     // 指标名称
}
```

## Props 配置

### DeviceMetricsSelector Props

| 属性名 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `modelValue` | `Object` | `{}` | 双向绑定的数据对象 |
| `deviceOptions` | `Array` | `[]` | 设备选项列表 |
| `disabled` | `Boolean` | `false` | 是否禁用 |
| `showMetricsName` | `Boolean` | `true` | 是否显示指标名称输入框 |
| `showAggregateFunction` | `Boolean` | `false` | 是否显示聚合函数选择 |
| `isNoAggregate` | `Boolean` | `false` | 是否为不聚合状态 |

### DeviceDispatchSelector Props

| 属性名 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `modelValue` | `Object` | `{}` | 双向绑定的数据对象 |
| `deviceOptions` | `Array` | `[]` | 设备选项列表 |
| `disabled` | `Boolean` | `false` | 是否禁用 |

## Events

### DeviceMetricsSelector Events

| 事件名 | 参数 | 说明 |
|--------|------|------|
| `update:modelValue` | `value: DeviceMetricsValue` | 数据变化时触发 |
| `device-change` | `deviceId: string, device: DeviceOption` | 设备选择变化时触发 |
| `metrics-change` | `metricsId: string, metrics: MetricsOption` | 指标选择变化时触发 |

### DeviceDispatchSelector Events

| 事件名 | 参数 | 说明 |
|--------|------|------|
| `update:modelValue` | `value: DeviceDispatchValue` | 数据变化时触发 |
| `device-change` | `deviceId: string, device: DeviceOption` | 设备选择变化时触发 |
| `data-type-change` | `dataType: string` | 数据类型变化时触发 |
| `metrics-change` | `metricsId: string, metrics: MetricsOption` | 指标选择变化时触发 |

## 方法

两个组件都暴露了以下方法：

| 方法名 | 参数 | 返回值 | 说明 |
|--------|------|--------|------|
| `loadDeviceOptions` | - | `Promise<void>` | 加载设备列表 |
| `reset` | - | `void` | 重置组件状态 |

## 样式定制

组件支持通过CSS变量进行样式定制：

```css
.device-metrics-selector,
.device-dispatch-selector {
  --selector-border-color: #e5e7eb;
  --selector-hover-color: #f0f9ff;
  --group-header-bg: #f5f5f5;
}
```

## 注意事项

1. **设备选项格式**: 设备选项必须包含 `id` 和 `name` 字段
2. **指标数据格式**: 指标数据必须包含 `key`、`label`、`data_type` 等字段
3. **API依赖**: 组件依赖 `getDeviceSourceList` 和 `getDeviceMetricList` API
4. **国际化**: 组件使用 `$t` 进行国际化，确保相关翻译文件存在
5. **自动加载**: 如果没有提供 `deviceOptions`，组件会自动加载设备列表
6. **数据结构**: API返回的指标数据结构为 `[{ data_source_type, options: [{ key, label, data_type }] }]`

## 测试

当前仓库没有 `src/views/test/*SelectorTest.vue` 这两个测试页面。现有的可回放消费和
测试入口是 `src/core/data-architecture/components/common/DynamicParameterComponentDrawer.vue`
以及相邻的 `DynamicParameterEditor.test.ts`；如果要增加独立 selector 测试页，需另行
记录 `CREATE_SUPPORT_ARTIFACT` 决策，不能把不存在的页面当作现有覆盖率证据。

## 更新日志

### v1.3.0 (最新)
- ✅ 彻底修复Naive UI组件`undefined`错误
- ✅ 为所有`NSelect`组件添加`|| []`保护
- ✅ 在`processedMetricsOptions`中添加try-catch错误处理
- ✅ 确保组件在任何情况下都不会传递`undefined`给Naive UI

### v1.2.0
- ✅ 修复JavaScript错误：`undefined`的`forEach`调用
- ✅ 增强数据验证和类型检查
- ✅ 改进API响应数据处理
- ✅ 防止空值和异常数据导致的错误

### v1.1.0
- ✅ 修复指标下拉无数据问题
- ✅ 完善国际化翻译
- ✅ 优化API数据结构适配
- ✅ 改进错误处理机制

### v1.0.0
- 🎉 初始版本发布
- ✅ 基础设备指标选择功能
- ✅ 支持分组显示
- ✅ 双向数据绑定
