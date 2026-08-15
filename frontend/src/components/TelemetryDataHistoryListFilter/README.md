# TelemetryDataHistoryListFilter 组件说明

## 目录定位

`frontend/src/components/TelemetryDataHistoryListFilter/` 是遥测历史数据筛选器目录。`index.vue` 负责实现时间范围、聚合窗口、聚合函数和导出能力，`README.md` 只负责说明组件如何接入和使用。

## 与文件的关系

- `index.vue` 是组件实现，内部直接调用 `telemetryDataHistoryList` 接口。
- `README.md` 说明 props、事件、数据结构和使用边界。

## 组件职责

这个组件用于为某个设备、某个遥测 key 拉取历史时间序列数据，并向父组件同步筛选条件、加载状态和查询结果。它还提供导出入口，导出的文件路径会由接口返回后打开。

## 依赖关系

- 业务接口：`@/service/api/device` 下的 `telemetryDataHistoryList`
- 状态工具：`useLoading`
- 日志工具：`createLogger`
- 公共工具：`getBaseServerUrl`
- UI 依赖：`naive-ui`、`@vicons/ionicons5`
- 国际化：`$t`

## Props

| 属性 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `deviceId` | `string` | 必填 | 设备 ID。 |
| `theKey` | `string` | 必填 | 遥测数据 key。 |
| `showExportButton` | `boolean` | `false` | 是否显示导出按钮。 |
| `displayMode` | `'detailed' \| 'simple'` | `'detailed'` | 展示模式，影响按钮还是图标样式。 |

## Emits

| 事件 | 参数 | 说明 |
| --- | --- | --- |
| `update:data` | `TimeSeriesItem[]` | 查询结果变更时触发。 |
| `update:loading` | `boolean` | 加载状态变更时触发。 |
| `update:filterParams` | `FilterParams` | 筛选条件准备好时触发，父组件可据此同步查询条件。 |

## 数据结构

```ts
interface TimeSeriesItem {
  x: number
  x2?: number
  y: number
}
```

`x` 和 `x2` 表示时间戳，`y` 表示遥测值。若后端返回的是聚合结果，`x2` 通常用于表示区间结束时间或辅助计算值。

## 筛选规则

- 时间范围支持预设区间和自定义区间。
- 自定义时间范围启用后，必须同时提供开始时间和结束时间。
- 聚合窗口会根据时间范围自动约束，不合法的组合会被内部归一化。
- 当聚合窗口为 `no_aggregate` 时，不再需要聚合函数。
- 当启用导出时，组件会把 `is_export` 追加到请求参数中。

## 使用片段

```vue
<script setup lang="ts">
import { ref } from 'vue'
import TelemetryDataHistoryListFilter from '@/components/TelemetryDataHistoryListFilter/index.vue'

const deviceId = ref('device-001')
const telemetryKey = ref('temperature')
type TimeSeriesItem = {
  x: number
  x2?: number
  y: number
}

const historyData = ref<TimeSeriesItem[]>([])
const isLoading = ref(false)
const filterParams = ref<Record<string, unknown>>({})

const handleDataUpdate = (data: TimeSeriesItem[]) => {
  historyData.value = data
}

const handleLoadingUpdate = (loading: boolean) => {
  isLoading.value = loading
}

const handleFilterParamsUpdate = (params: Record<string, unknown>) => {
  filterParams.value = params
}
</script>

<template>
  <div>
    <TelemetryDataHistoryListFilter
      :device-id="deviceId"
      :the-key="telemetryKey"
      :show-export-button="true"
      display-mode="detailed"
      @update:data="handleDataUpdate"
      @update:loading="handleLoadingUpdate"
      @update:filterParams="handleFilterParamsUpdate"
    />

    <n-alert v-if="isLoading" type="info" title="数据加载中">
      正在拉取遥测历史数据，请稍候。
    </n-alert>

    <n-empty v-else-if="historyData.length === 0" description="暂无历史数据" />

    <pre v-else>{{ JSON.stringify(historyData, null, 2) }}</pre>
  </div>
</template>
```

## 使用注意事项

1. 组件内部会在挂载后自动拉取一次数据。
2. 只要 `deviceId` 或 `theKey` 为空，就会跳过请求。
3. 导出按钮依赖后端返回 `filePath` 或 `file_path`。
4. 当时间范围是自定义区间时，如果缺少起止时间，导出会被拦截。
5. 组件内部已经做了聚合窗口校验，外层不要再重复拼装无效参数。
6. 使用片段中不应保留 `Loading data...` 这类演示型英文占位文案，公开仓库建议统一改成业务化中文提示。

## 静态审查建议

- 检查 `telemetryDataHistoryList` 的返回结构是否同时兼容 `data` 和 `error`。
- 检查导出路径拼接是否始终符合当前后端基地址规则。
- 检查国际化 key 是否都已接入，并避免直接写死中英文混排按钮文案。
- 检查父组件是否真的处理了 `update:filterParams`，否则筛选条件可能只在组件内生效。

## 后续改进方向

- 把时间范围、聚合窗口和导出逻辑拆成更小的组合函数，降低单文件复杂度。
- 为导出失败和请求失败补充更细的错误提示映射。
- 把筛选参数类型导出到独立的公共类型文件，方便父组件复用。
