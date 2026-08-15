<!--
  文件用途：提供遥测历史数据列表的时间、聚合和导出筛选器。
  核心职责：维护筛选参数，调用 telemetryDataHistoryList 拉取或导出数据，并向父组件同步数据、加载态与筛选条件。
  关键约束：时间范围与聚合粒度会共同影响查询结果和导出结果，调整默认值时需同步确认接口兼容性。
  维护提示：若后续继续扩展筛选项，优先复用现有校验与触发链路，避免在多个 watch 中分散业务判断。
-->
<script setup lang="ts">
import { reactive, ref, watch, onMounted, computed } from 'vue'
import { telemetryDataHistoryList } from '@/service/api/device'
import { useLoading } from '~/packages/hooks'
import { createLogger } from '@/utils/logger'
import { NSpace, NDatePicker, NButton, NTooltip, NPopselect, NIcon } from 'naive-ui'
import { LayersOutline, CalendarOutline, StatsChartOutline, SwapHorizontalOutline } from '@vicons/ionicons5'
import { $t } from '@/locales'
import { getBaseServerUrl } from '@/utils/common/tool'
import {
  applyAggregateWindowValidation as applyAggregateWindowValidationRule,
  buildAggregateWindowOptions,
  buildTelemetryHistoryParams as buildTelemetryHistoryRequestParams,
  canFetchWithCurrentFilters as canFetchWithFilterState,
  cloneFilterParams,
  createAggregateWindowOptions,
  getMinWindowSecondsForFilter,
  type AggregateWindowOption,
  type FilterParams,
  type TimeSeriesItem
} from './telemetryHistoryFilterState'

const logger = createLogger('TelemetryFilter')

// 定义 Props
const props = withDefaults(
  defineProps<{
    deviceId: string
    theKey: string
    showExportButton?: boolean // 控制是否显示导出按钮
    displayMode?: 'detailed' | 'simple' // detailed 显示按钮，simple 显示图标
  }>(),
  {
    showExportButton: false,
    displayMode: 'detailed'
  }
)

// 定义 Emits
const emit = defineEmits<{
  (event: 'update:data', data: TimeSeriesItem[]): void // 数据更新事件
  (event: 'update:loading', isLoading: boolean): void // 加载状态更新事件
  (event: 'update:filterParams', params: FilterParams): void // 筛选参数更新事件
}>()

const allAggregateWindowOptions = createAggregateWindowOptions($t)

const { loading: isLoading, startLoading, endLoading } = useLoading(false)
const { loading: isExporting, startLoading: startExporting, endLoading: endExporting } = useLoading(false) // 导出流程单独维护加载态
const timeSeriesData = ref<TimeSeriesItem[]>([])

// 初始化筛选条件的响应式状态
const filterParams = reactive<FilterParams>({
  time_range: 'last_1h', // 默认时间范围
  aggregate_window: 'no_aggregate' // 默认聚合间隔
})

// 用于绑定日期时间范围选择器
const dateRangeRef = ref<[number, number] | null>(null)

// --- 下拉选项 ---
const timeRangeOptions = ref([
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
])

const aggregateFunctionOptions = ref([
  { label: $t('common.average'), value: 'avg' },
  { label: $t('generate.max-value'), value: 'max' },
  { label: $t('generate.min-value'), value: 'min' },
  { label: $t('generate.sum'), value: 'sum' },
  { label: $t('generate.diff'), value: 'diff' }
])

// --- 计算属性 ---
const currentMinWindowSeconds = computed(() =>
  getMinWindowSecondsForFilter(filterParams.time_range, dateRangeRef.value)
)

const aggregateWindowOptions = computed<AggregateWindowOption[]>(() =>
  buildAggregateWindowOptions(allAggregateWindowOptions, currentMinWindowSeconds.value)
)

const showAggregateFunction = computed(() => filterParams.aggregate_window !== 'no_aggregate')

// 当前选中项文案
const selectedTimeRangeLabel = computed(() => {
  return timeRangeOptions.value.find((opt) => opt.value === filterParams.time_range)?.label ?? $t('common.timeFrame')
})

const selectedAggregateWindowLabel = computed(() => {
  return (
    allAggregateWindowOptions.find((opt) => opt.value === filterParams.aggregate_window)?.label ??
    $t('card.aggregationScope')
  )
})

const selectedAggregateFunctionLabel = computed(() => {
  if (!showAggregateFunction.value) return ''
  return (
    aggregateFunctionOptions.value.find((opt) => opt.value === filterParams.aggregate_function)?.label ??
    $t('card.aggregationMethod')
  )
})

const openExportFile = (payload: Record<string, any>) => {
  const filePath = payload?.filePath || payload?.file_path
  if (!filePath) return false

  const baseUrlWithoutApi = getBaseServerUrl().replace('/api/v1', '/')
  window.open(`${baseUrlWithoutApi}${filePath}`, '_blank', 'noopener,noreferrer')
  return true
}

const buildTelemetryHistoryParams = (isExport: boolean) =>
  buildTelemetryHistoryRequestParams(props.deviceId, props.theKey, filterParams, isExport)

const setDisplayData = (data: TimeSeriesItem[]) => {
  timeSeriesData.value = data
  emit('update:data', timeSeriesData.value)
}

const clearDisplayData = () => {
  timeSeriesData.value = []
  emit('update:data', [])
}

const handleHistoryExportResponse = (payload: unknown) => {
  logger.info('Export successful:', payload)
  if (openExportFile(payload as Record<string, any>)) {
    window.$message?.success($t('common.operationSuccess'))
  } else {
    window.$message?.error($t('common.operationFailed'))
  }
}

const handleHistoryListResponse = (payload: unknown) => {
  setDisplayData((payload || []) as TimeSeriesItem[])
}

const showHistoryRequestError = (isExport: boolean, error: any) => {
  const message = error?.message || $t('common.error')
  if (isExport) {
    window.$message?.error(`${$t('common.operationFailed')}: ${message}`)
    return
  }

  clearDisplayData()
  window.$message?.error(`${$t('common.fetchDataFailed')}: ${message}`)
}

const beginHistoryRequest = (isExport: boolean) => {
  if (isExport) {
    startExporting()
    return
  }

  startLoading()
  emit('update:loading', true)
}

const endHistoryRequest = (isExport: boolean) => {
  if (isExport) {
    endExporting()
    return
  }

  endLoading()
  emit('update:loading', false)
}

// --- 数据获取逻辑 ---
const fetchData = async (isExport = false) => {
  if (!props.deviceId || !props.theKey) {
    logger.warn('Device ID or Key is missing, skipping fetch.')
    clearDisplayData()
    return
  }

  beginHistoryRequest(isExport)
  const params = buildTelemetryHistoryParams(isExport)

  try {
    logger.info(
      `Fetching telemetry data (${isExport ? 'Export' : 'Display'}). Params:`,
      JSON.parse(JSON.stringify(params))
    )
    const response: { data: TimeSeriesItem[] | null; error: any } = await telemetryDataHistoryList(params)
    logger.info('API Response:', response)

    if (response && response.data && !response.error) {
      if (isExport) {
        handleHistoryExportResponse(response.data)
      } else {
        handleHistoryListResponse(response.data)
      }
    } else {
      logger.error('API Error or invalid response:', response?.error)
      showHistoryRequestError(isExport, response?.error)
    }
  } catch (error: any) {
    logger.error('Fetch exception:', error)
    showHistoryRequestError(isExport, error)
  } finally {
    endHistoryRequest(isExport)
  }
}

const applyAggregateWindowValidation = () => {
  const previousWindow = filterParams.aggregate_window
  const windowChanged = applyAggregateWindowValidationRule(
    filterParams,
    aggregateWindowOptions.value,
    currentMinWindowSeconds.value
  )

  if (windowChanged) {
    logger.info(`Aggregate window '${previousWindow}' normalized to '${filterParams.aggregate_window}'.`)
  }

  return windowChanged
}

const canFetchWithCurrentFilters = () => canFetchWithFilterState(filterParams)

const emitCurrentFilterParams = () => {
  emit('update:filterParams', cloneFilterParams(filterParams))
}

// --- 监听器和生命周期钩子 ---

// 监听自定义时间范围选择
watch(
  dateRangeRef,
  (newRange) => {
    if (newRange && newRange.length === 2) {
      if (filterParams.time_range !== 'custom') {
        logger.info('Date range selected, forcing time_range to custom.')
        filterParams.time_range = 'custom'
      }
      filterParams.start_time = newRange[0]
      filterParams.end_time = newRange[1]
      if (filterParams.time_range === 'custom') {
        logger.info('Custom date range updated, triggering validation and fetch.')
        validateAndFetch()
      }
    } else if (filterParams.time_range === 'custom') {
      delete filterParams.start_time
      delete filterParams.end_time
      logger.info('Custom date range cleared. Fetch prevented.')
    }
  },
  { deep: true }
)

// 监听时间范围变化，并串联后续校验与拉取
watch(
  () => filterParams.time_range,
  (newTimeRange, oldTimeRange) => {
    logger.info(`Time range changed: ${oldTimeRange} -> ${newTimeRange}`)

    if (newTimeRange !== 'custom') {
      dateRangeRef.value = null
    }

    validateAndFetch()
  }
)

// 监听聚合粒度变化，补齐默认聚合方式
watch(
  () => filterParams.aggregate_window,
  (newWindow) => {
    logger.info(`Aggregate window changed to: ${newWindow}`)
    if (newWindow === 'no_aggregate') {
      delete filterParams.aggregate_function
    } else if (!filterParams.aggregate_function) {
      filterParams.aggregate_function = 'avg'
    }
    validateAndFetch()
  }
)

// 监听聚合方式变化，触发重新校验与拉取
watch(
  () => filterParams.aggregate_function,
  (newFunction) => {
    if (filterParams.aggregate_window !== 'no_aggregate') {
      logger.info(`Aggregate function changed to: ${newFunction}, triggering validation.`)
      validateAndFetch()
    } else {
      logger.warn(`Aggregate function changed (${newFunction}) while window is 'no_aggregate'. Ignoring.`)
    }
  }
)

// 统一入口：校验当前筛选条件并决定是否发起请求
const validateAndFetch = () => {
  applyAggregateWindowValidation()

  if (!canFetchWithCurrentFilters()) {
    logger.info('Validation complete. Waiting for custom date range selection...')
    emitCurrentFilterParams()
  } else {
    logger.info('Validation complete. Triggering fetchData and emitting filterParams.')
    emitCurrentFilterParams()
    fetchData()
  }
}

// 监听设备与字段切换
watch(
  () => [props.deviceId, props.theKey],
  (newVal, oldVal) => {
    if (newVal[0] && newVal[1] && (newVal[0] !== oldVal[0] || newVal[1] !== oldVal[1])) {
      logger.info('Device ID or Key changed, fetching data...')
      validateAndFetch()
    }
  },
  { immediate: false }
)

// 首次挂载时执行初始校验与拉取
onMounted(() => {
  logger.info('Component mounted, performing initial validation and fetch...')
  validateAndFetch()
})

// --- 事件处理 ---
// 导出按钮点击处理函数
const handleExport = () => {
  if (filterParams.time_range === 'custom' && (!filterParams.start_time || !filterParams.end_time)) {
    window.$message?.warning($t('common.rangeMustSelected'))
    return
  }
  logger.info('Export button clicked.')
  emitCurrentFilterParams()
  fetchData(true)
}
</script>

<template>
  <n-space align="center" wrap item-style="margin-bottom: 5px;" :size="4">
    <!-- 时间范围 -->
    <n-popselect v-model:value="filterParams.time_range" :options="timeRangeOptions" size="small" trigger="click">
      <!-- 详细模式触发器 -->
      <n-button v-if="props.displayMode === 'detailed'" size="small" tertiary>
        {{ selectedTimeRangeLabel }}
      </n-button>
      <!-- 简洁模式触发器 -->
      <n-tooltip v-else trigger="hover">
        <template #trigger>
          <div
            style="
              width: 20px;
              height: 20px;
              background-color: #60a5fa;
              border-radius: 50%;
              cursor: pointer;
              display: flex;
              align-items: center;
              justify-content: center;
            "
          >
            <n-icon size="14" color="#fff">
              <CalendarOutline />
            </n-icon>
          </div>
        </template>
        {{ selectedTimeRangeLabel }}
      </n-tooltip>
    </n-popselect>

    <!-- 自定义时间范围 -->
    <n-date-picker
      v-show="filterParams.time_range === 'custom'"
      v-model:value="dateRangeRef"
      type="datetimerange"
      clearable
      format="yyyy-MM-dd HH:mm:ss"
      :placeholder="$t('generate.timeRangeWarning')"
      size="small"
      style="min-width: 280px"
      :disabled="filterParams.time_range !== 'custom'"
    />

    <!-- 聚合粒度 -->
    <n-popselect
      v-model:value="filterParams.aggregate_window"
      :options="aggregateWindowOptions"
      size="small"
      trigger="click"
    >
      <!-- 详细模式触发器 -->
      <n-button v-if="props.displayMode === 'detailed'" size="small" tertiary>
        {{ selectedAggregateWindowLabel }}
      </n-button>
      <!-- 简洁模式触发器 -->
      <n-tooltip v-else trigger="hover">
        <template #trigger>
          <div
            style="
              width: 20px;
              height: 20px;
              background-color: #34d399;
              border-radius: 50%;
              cursor: pointer;
              display: flex;
              align-items: center;
              justify-content: center;
            "
          >
            <n-icon size="14" color="#fff">
              <LayersOutline />
            </n-icon>
          </div>
        </template>
        {{ selectedAggregateWindowLabel }}
      </n-tooltip>
    </n-popselect>

    <!-- 聚合方式 -->
    <n-popselect
      v-if="showAggregateFunction"
      v-model:value="filterParams.aggregate_function"
      :options="aggregateFunctionOptions"
      size="small"
      trigger="click"
    >
      <!-- 详细模式触发器 -->
      <n-button v-if="props.displayMode === 'detailed'" size="small" tertiary>
        {{ selectedAggregateFunctionLabel }}
      </n-button>
      <!-- 简洁模式触发器 -->
      <n-tooltip v-else trigger="hover">
        <template #trigger>
          <div
            style="
              width: 20px;
              height: 20px;
              background-color: #fbbf24;
              border-radius: 50%;
              cursor: pointer;
              display: flex;
              align-items: center;
              justify-content: center;
            "
          >
            <n-icon size="14" color="#fff">
              <StatsChartOutline />
            </n-icon>
          </div>
        </template>
        {{ selectedAggregateFunctionLabel }}
      </n-tooltip>
    </n-popselect>

    <!-- 导出 -->
    <template v-if="props.showExportButton">
      <!-- 详细模式触发器 -->
      <n-button
        v-if="props.displayMode === 'detailed'"
        :loading="isExporting"
        :disabled="isLoading || isExporting"
        type="primary"
        size="small"
        ghost
        @click="handleExport"
      >
        {{ $t('common.export') }}
      </n-button>
      <!-- 简洁模式触发器 -->
      <n-tooltip v-else trigger="hover">
        <template #trigger>
          <n-button
            text
            :loading="isExporting"
            :disabled="isLoading || isExporting"
            style="font-size: 18px; padding: 2px"
            @click="handleExport"
          >
            <n-icon :component="SwapHorizontalOutline" />
          </n-button>
        </template>
        {{ $t('common.export') }}
      </n-tooltip>
    </template>
  </n-space>
</template>

<style scoped>
/* 按钮禁用态保持基础视觉反馈 */
button[disabled] {
  opacity: 0.6;
  cursor: not-allowed;
}
</style>
