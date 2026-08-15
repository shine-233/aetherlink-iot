<!--
  文件用途:
  遥测图表的聚合条件选择器，负责收集时间范围、聚合窗口、统计函数三类查询条件。

  数据流:
  1. 父组件通过 props 传入设备标识和指标 key。
  2. 组件内部维护 aggregation_data，作为当前查询条件的唯一来源。
  3. 任一选项变更后，通过 update:value 把完整条件对象回传给父组件。

  使用注意:
  1. time_range、aggregate_window、aggregate_function 的枚举值需要与后端查询接口保持一致。
  2. 自定义时间范围会直接写入 start_time/end_time，调用方应按时间戳语义消费。
  3. 该组件通过“禁用低粒度聚合窗口”来约束查询规模，后续改动要同步校验权重映射。

  静态审查建议:
  1. onChangeTime / checkDateRange / aggregationTtemToFalse 共同维护状态，建议重点检查是否存在隐式覆盖字段的问题。
  2. timeWeighting 与 aggregationIntervalOptions 依赖索引顺序耦合，审查时应确认新增选项不会打乱禁用逻辑。
  3. defineEmits 和回传对象当前未细化类型，后续可补强事件载荷类型以降低调用方误用风险。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { TimeOutline, StatsChartOutline, DiscOutline } from '@vicons/ionicons5'
import { addYears, differenceInDays, differenceInHours, differenceInMonths } from 'date-fns'
import { $t } from '@/locales'
import { message } from '@/utils/common/discrete'

// 对外只暴露完整聚合条件，父组件无需感知内部局部状态。
const emit = defineEmits<{
  (event: 'update:value', value): void
}>()

// 设备与指标标识来自父层上下文，用于初始化查询条件对象。
const props = defineProps<{
  device_id: string
  thekey: string
}>()

interface AggregationData {
  device_id: string
  key: string
  aggregate_window: string
  time_range: string
  start_time?: number
  end_time?: number
  aggregate_function?: string
}

// 当前查询条件的单一事实来源，所有回传给父组件的数据都从这里读取。
const aggregation_data = ref<AggregationData>({
  device_id: props.device_id,
  key: props.thekey,
  aggregate_window: 'no_aggregate',
  time_range: 'last_1h'
})

// 仅用于日期组件展示；真正提交给父组件的仍是 aggregation_data 内的时间戳字段。
const dateRange = ref<[number, number] | null>(null)

// 时间范围选项决定默认查询跨度，也会间接影响允许选择的聚合粒度。
const timeOptions = [
  { label: $t('common.custom'), value: 'custom' },
  { label: $t('common.last_5m'), value: 'last_5m' },
  { label: $t('common.last_15m'), value: 'last_15m' },
  { label: $t('common.last_30m'), value: 'last_30m' },
  { label: $t('common.lastHours1'), value: 'last_1h' },
  { label: $t('common.lastHours3'), value: 'last_3h' },
  { label: $t('common.lastHours6'), value: 'last_6h' },
  { label: $t('common.lastHours12'), value: 'last_12h' },
  { label: $t('common.lastHours24'), value: 'last_24h' },
  { label: $t('common.lastDays3'), value: 'last_3d' },
  { label: $t('common.lastDays7'), value: 'last_7d' },
  { label: $t('common.lastDays15'), value: 'last_15d' },
  { label: $t('common.lastDays30'), value: 'last_30d' },
  { label: $t('common.lastDays60'), value: 'last_60d' },
  { label: $t('common.lastDays90'), value: 'last_90d' },
  { label: $t('common.halfYear'), value: 'last_6m' },
  { label: $t('common.lastYears1'), value: 'last_1y' }
]

// 该映射值本质上是聚合窗口选项的索引阈值，而不是独立业务枚举。
const timeWeighting = {
  custom: 0,
  last_5m: 0,
  last_15m: 0,
  last_30m: 0,
  last_1h: 0,
  last_3h: 1,
  last_6h: 2,
  last_12h: 3,
  last_24h: 4,
  last_3d: 5,
  last_7d: 6,
  last_15d: 7,
  last_30d: 8,
  last_60d: 9,
  last_90d: 10,
  last_6m: 11,
  last_1y: 12
}

// 选项数组的顺序与 timeWeighting 的阈值判断存在耦合，调整顺序时要同步检查禁用逻辑。
const aggregationIntervalOptions = [
  { label: $t('common.notAggre'), value: 'no_aggregate', disabled: false },
  { label: $t('common.seconds30'), value: '30s', disabled: false },
  { label: $t('common.minute1'), value: '1m', disabled: false },
  { label: $t('common.minute2'), value: '2m', disabled: false },
  { label: $t('common.minutes5'), value: '5m', disabled: false },
  { label: $t('common.minutes10'), value: '10m', disabled: false },
  { label: $t('common.minutes30'), value: '30m', disabled: false },
  { label: $t('common.hours1'), value: '1h', disabled: false },
  { label: $t('common.hours3'), value: '3h', disabled: false },
  { label: $t('common.hours6'), value: '6h', disabled: false },
  { label: $t('common.days1'), value: '1d', disabled: false },
  { label: $t('common.days7'), value: '7d', disabled: false },
  { label: $t('common.months1'), value: '1mo', disabled: false }
]

// 聚合函数只在存在 aggregate_window 时才有意义。
const statisticsOptions = [
  { label: $t('common.average'), value: 'avg', disabled: false },
  { label: $t('generate.max-value'), value: 'max', disabled: false },
  { label: $t('generate.min-value'), value: 'min', disabled: false },
  { label: $t('generate.sum'), value: 'sum', disabled: false },
  { label: $t('generate.diff'), value: 'diff', disabled: false }
]

// 根据时间跨度禁用过细的聚合窗口，并在必要时修正聚合函数的默认值。
// 静态审查时应关注该方法会直接改写 aggregate_window，避免与用户当前选择产生意外覆盖。
const aggregationTtemToFalse = (weight: number) => {
  aggregationIntervalOptions.forEach((item, index) => {
    if (index < weight) {
      item.disabled = true
    } else {
      item.disabled = false
    }
    if (index < weight + 1) {
      aggregation_data.value.aggregate_window = item.value
      if (aggregation_data.value.aggregate_window !== 'no_aggregate' && !aggregation_data.value.aggregate_function) {
        aggregation_data.value.aggregate_function = 'avg'
      }
      if (aggregation_data.value.aggregate_window === 'no_aggregate') {
        aggregation_data.value.aggregate_function = undefined
      }
    }
  })
}

// 时间范围变更入口：
// 1. 非自定义时清空手选时间戳；
// 2. 根据时间范围重新计算可用聚合粒度；
// 3. 向父组件同步完整条件对象。
function onChangeTime(v) {
  if (v !== 'custom') {
    aggregation_data.value.start_time = undefined
    aggregation_data.value.end_time = undefined
    dateRange.value = null
  }
  aggregationTtemToFalse(timeWeighting[v])
  if (v) emit('update:value', aggregation_data.value)
}

// 聚合窗口变更时维护统计函数必填关系。
// no_aggregate 表示原始点位，不应携带 aggregate_function。
function onChangeAggregation(v) {
  if (v !== 'no_aggregate' && !aggregation_data.value.aggregate_function) {
    aggregation_data.value.aggregate_function = 'avg'
  }
  if (v === 'no_aggregate') {
    aggregation_data.value.aggregate_function = undefined
  }
  emit('update:value', aggregation_data.value)
}

// 统计函数仅在已有聚合窗口时生效，这里只负责把最新条件透传给父组件。
function onChangeStatistics() {
  emit('update:value', aggregation_data.value)
}

// 将自定义时间跨度映射到权重区间，供聚合窗口禁用逻辑使用。
// 使用注意：返回值与 aggregationIntervalOptions 的顺序强耦合。
function getWeightNumber(diffHours, diffDays, diffMonths) {
  if (diffHours <= 1) return timeWeighting.last_1h
  if (diffHours <= 3) return timeWeighting.last_3h
  if (diffHours <= 6) return timeWeighting.last_6h
  if (diffHours <= 12) return timeWeighting.last_12h
  if (diffHours <= 24) return timeWeighting.last_24h
  if (diffDays <= 3) return timeWeighting.last_3d
  if (diffDays <= 7) return timeWeighting.last_7d
  if (diffDays <= 15) return timeWeighting.last_15d
  if (diffDays <= 30) return timeWeighting.last_30d
  if (diffDays <= 60) return timeWeighting.last_60d
  if (diffDays <= 90) return timeWeighting.last_90d
  if (diffMonths <= 6) return timeWeighting.last_6m
  if (diffMonths <= 12) return timeWeighting.last_1y

  return timeWeighting.last_1y
}

// 自定义时间范围校验：
// 1. 限制跨度不超过一年；
// 2. 写回开始/结束时间；
// 3. 根据跨度自动收紧可选聚合粒度。
// 静态审查建议：这里既处理校验又处理状态同步，后续若增加更多限制条件，建议拆分纯计算逻辑便于测试。
const checkDateRange = value => {
  const [start, end] = value
  if (start && end && addYears(start, 1) < end) {
    dateRange.value = null
    message.error($t('common.withinOneYear'))
  } else {
    aggregation_data.value.start_time = start
    aggregation_data.value.end_time = end

    const diffHours = differenceInHours(end, start)
    const diffDays = differenceInDays(end, start)
    const diffMonths = differenceInMonths(end, start)
    let weight = 0

    weight = getWeightNumber(diffHours, diffDays, diffMonths)
    aggregationTtemToFalse(weight)
    emit('update:value', aggregation_data.value)
  }
}

// 首次挂载时把默认查询条件同步给父组件，保证父层首次请求具备完整参数。
onMounted(() => {
  emit('update:value', aggregation_data.value)
})
</script>

<template>
  <NFlex justify="start" :size="4">
    <n-popselect
      v-model:value="aggregation_data.time_range"
      scrollable
      :options="timeOptions"
      trigger="click"
      @update:value="onChangeTime"
    >
      <n-icon size="24" :color="aggregation_data.time_range !== 'custom' ? '#0e7a0d' : ''">
        <TimeOutline />
      </n-icon>
    </n-popselect>
    <n-date-picker
      v-if="aggregation_data.time_range === 'custom'"
      v-model:value="dateRange"
      size="small"
      class="w-300px"
      type="datetimerange"
      @update:value="checkDateRange"
    />
    <n-popselect
      v-model:value="aggregation_data.aggregate_window"
      scrollable
      :options="aggregationIntervalOptions"
      trigger="click"
      @update:value="onChangeAggregation"
    >
      <n-icon size="24" :color="aggregation_data.aggregate_window !== 'no_aggregate' ? '#0e7a0d' : ''">
        <StatsChartOutline />
      </n-icon>
    </n-popselect>

    <!-- 仅在选择聚合窗口后才展示统计函数选择器。 -->
    <n-popselect
      v-if="aggregation_data.aggregate_window !== 'no_aggregate'"
      v-model:value="aggregation_data.aggregate_function"
      :options="statisticsOptions"
      trigger="click"
      @update:value="onChangeStatistics"
    >
      <n-icon size="24" :color="aggregation_data.aggregate_function ? '#0e7a0d' : ''">
        <DiscOutline />
      </n-icon>
    </n-popselect>
  </NFlex>
</template>

<style scoped></style>
