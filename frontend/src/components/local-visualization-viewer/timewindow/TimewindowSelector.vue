<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NButton,
  NButtonGroup,
  NDatePicker,
  NFormItem,
  NIcon,
  NPopover,
  NRadio,
  NRadioGroup,
  NSelect,
  NSpace,
  NTabPane,
  NTabs
} from 'naive-ui'
import { DEFAULT_TIMEWINDOW_CONFIG, REALTIME_INTERVAL_MAP, resolveTimewindow } from './timewindow-model'
import type {
  AggregationFunc,
  QuickHistoryInterval,
  RealtimeIntervalLabel,
  RefreshInterval,
  ResolvedTimeRange,
  TimewindowConfig,
  TimewindowType
} from './types'

const props = withDefaults(
  defineProps<{
    modelValue?: TimewindowConfig
    disabled?: boolean
    size?: 'small' | 'medium' | 'large'
  }>(),
  {
    modelValue: () => ({ ...DEFAULT_TIMEWINDOW_CONFIG }),
    disabled: false,
    size: 'small'
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: TimewindowConfig): void
  (e: 'change', value: TimewindowConfig, resolved: ResolvedTimeRange): void
}>()

const visible = ref(false)

// 内部草稿状态
const activeType = ref<TimewindowType>('realtime')
const selectedRealtimeInterval = ref<RealtimeIntervalLabel>('1h')
const selectedRefreshInterval = ref<RefreshInterval>(10000)
const historyMode = ref<'quick' | 'custom'>('quick')
const selectedQuickInterval = ref<QuickHistoryInterval>('today')
const customDateRange = ref<[number, number]>([Date.now() - 3600 * 1000, Date.now()])
const selectedAggregationFunc = ref<AggregationFunc>('avg')
const selectedGroupingInterval = ref<number>(0)
const selectedTimezone = ref<string>('browser')

const REALTIME_OPTIONS: { label: string; value: RealtimeIntervalLabel }[] = [
  { label: '1分钟', value: '1m' },
  { label: '5分钟', value: '5m' },
  { label: '15分钟', value: '15m' },
  { label: '30分钟', value: '30m' },
  { label: '1小时', value: '1h' },
  { label: '2小时', value: '2h' },
  { label: '6小时', value: '6h' },
  { label: '12小时', value: '12h' },
  { label: '1天', value: '1d' },
  { label: '7天', value: '7d' },
  { label: '30天', value: '30d' }
]

const QUICK_HISTORY_OPTIONS: { label: string; value: QuickHistoryInterval }[] = [
  { label: '今天', value: 'today' },
  { label: '昨天', value: 'yesterday' },
  { label: '本周', value: 'this_week' },
  { label: '上周', value: 'prev_week' },
  { label: '本月', value: 'this_month' },
  { label: '上月', value: 'prev_month' },
  { label: '过去7天', value: 'last_7d' },
  { label: '过去30天', value: 'last_30d' }
]

const REFRESH_OPTIONS: { label: string; value: RefreshInterval }[] = [
  { label: '不自动刷新', value: 0 },
  { label: '1 秒', value: 1000 },
  { label: '5 秒', value: 5000 },
  { label: '10 秒', value: 10000 },
  { label: '30 秒', value: 30000 },
  { label: '1 分钟', value: 60000 }
]

const AGGREGATION_OPTIONS: { label: string; value: AggregationFunc }[] = [
  { label: '原始数据 (无聚合)', value: 'none' },
  { label: '平均值 (AVG)', value: 'avg' },
  { label: '最小值 (MIN)', value: 'min' },
  { label: '最大值 (MAX)', value: 'max' },
  { label: '累加求和 (SUM)', value: 'sum' },
  { label: '数据计数 (COUNT)', value: 'count' }
]

const GROUPING_OPTIONS = [
  { label: '自动适应', value: 0 },
  { label: '1 秒', value: 1000 },
  { label: '5 秒', value: 5000 },
  { label: '15 秒', value: 15000 },
  { label: '30 秒', value: 30000 },
  { label: '1 分钟', value: 60000 },
  { label: '5 分钟', value: 300000 },
  { label: '1 小时', value: 3600000 },
  { label: '1 天', value: 86400000 }
]

const TIMEZONE_OPTIONS = [
  { label: '浏览器本地时区', value: 'browser' },
  { label: '标准世界时 (UTC)', value: 'utc' },
  { label: '北京时间 (UTC+8)', value: 'Asia/Shanghai' }
]

function syncFromProps() {
  const current = props.modelValue || DEFAULT_TIMEWINDOW_CONFIG
  activeType.value = current.type || 'realtime'

  if (current.realtime?.interval && current.realtime.interval in REALTIME_INTERVAL_MAP) {
    selectedRealtimeInterval.value = current.realtime.interval as RealtimeIntervalLabel
  } else {
    selectedRealtimeInterval.value = '1h'
  }

  selectedRefreshInterval.value = current.refreshInterval !== undefined ? current.refreshInterval : 10000

  if (current.history?.fixedRange && current.history.fixedRange.startTime > 0) {
    historyMode.value = 'custom'
    customDateRange.value = [current.history.fixedRange.startTime, current.history.fixedRange.endTime]
  } else {
    historyMode.value = 'quick'
    selectedQuickInterval.value = current.history?.quickInterval || 'today'
  }

  selectedAggregationFunc.value = current.aggregation?.func || 'avg'
  selectedGroupingInterval.value = current.aggregation?.interval || 0
  selectedTimezone.value = current.timezone || 'browser'
}

watch(() => props.modelValue, syncFromProps, { immediate: true, deep: true })

const currentDisplayLabel = computed(() => {
  const current = props.modelValue || DEFAULT_TIMEWINDOW_CONFIG
  if (current.type === 'realtime') {
    const item = REALTIME_OPTIONS.find((o) => o.value === current.realtime?.interval)
    const label = item ? item.label : '1小时'
    const aggLabel = current.aggregation?.func === 'none' ? '原始' : current.aggregation?.func?.toUpperCase() || 'AVG'
    return `实时: 过去 ${label} (${aggLabel})`
  }

  if (current.history?.fixedRange && current.history.fixedRange.startTime > 0) {
    const s = new Date(current.history.fixedRange.startTime).toLocaleDateString()
    const e = new Date(current.history.fixedRange.endTime).toLocaleDateString()
    return `历史: ${s} ~ ${e}`
  }

  const q = QUICK_HISTORY_OPTIONS.find((o) => o.value === current.history?.quickInterval)
  return `历史: ${q ? q.label : '今天'}`
})

function applyChanges() {
  const nextConfig: TimewindowConfig = {
    type: activeType.value,
    aggregation: {
      func: selectedAggregationFunc.value,
      interval: selectedGroupingInterval.value
    },
    refreshInterval: selectedRefreshInterval.value,
    timezone: selectedTimezone.value
  }

  if (activeType.value === 'realtime') {
    nextConfig.realtime = {
      interval: selectedRealtimeInterval.value
    }
  } else if (historyMode.value === 'custom') {
    nextConfig.history = {
      fixedRange: {
        startTime: customDateRange.value[0],
        endTime: customDateRange.value[1]
      }
    }
  } else {
    nextConfig.history = {
      quickInterval: selectedQuickInterval.value
    }
  }

  const resolved = resolveTimewindow(nextConfig)
  emit('update:modelValue', nextConfig)
  emit('change', nextConfig, resolved)
  visible.value = false
}

function handleCancel() {
  syncFromProps()
  visible.value = false
}
</script>

<template>
  <div class="timewindow-selector inline-flex items-center">
    <NPopover v-model:show="visible" trigger="click" placement="bottom-end" class="timewindow-popover">
      <template #trigger>
        <NButton :size="size" :disabled="disabled" secondary round class="timewindow-btn">
          <template #icon>
            <NIcon>
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <circle cx="12" cy="12" r="10"></circle>
                <polyline points="12 6 12 12 16 14"></polyline>
              </svg>
            </NIcon>
          </template>
          <span>{{ currentDisplayLabel }}</span>
        </NButton>
      </template>

      <div class="w-96 p-2">
        <NTabs v-model:value="activeType" type="segment" animated size="small">
          <!-- 实时模式 -->
          <NTabPane name="realtime" tab="实时模式 (Realtime)">
            <div class="space-y-3 py-2">
              <div>
                <div class="mb-1 text-xs font-medium text-gray-500">快速相对时间窗口</div>
                <div class="grid grid-cols-4 gap-1.5">
                  <NButton
                    v-for="opt in REALTIME_OPTIONS"
                    :key="opt.value"
                    size="tiny"
                    :type="selectedRealtimeInterval === opt.value ? 'primary' : 'default'"
                    :secondary="selectedRealtimeInterval !== opt.value"
                    @click="selectedRealtimeInterval = opt.value"
                  >
                    {{ opt.label }}
                  </NButton>
                </div>
              </div>

              <NFormItem label="自动刷新周期" :show-feedback="false" size="small">
                <NSelect v-model:value="selectedRefreshInterval" :options="REFRESH_OPTIONS" size="small" />
              </NFormItem>
            </div>
          </NTabPane>

          <!-- 历史模式 -->
          <NTabPane name="history" tab="历史模式 (History)">
            <div class="space-y-3 py-2">
              <NRadioGroup v-model:value="historyMode" size="small">
                <NSpace>
                  <NRadio value="quick">快捷自然区间</NRadio>
                  <NRadio value="custom">精确自定义范围</NRadio>
                </NSpace>
              </NRadioGroup>

              <div v-if="historyMode === 'quick'" class="grid grid-cols-4 gap-1.5 pt-1">
                <NButton
                  v-for="opt in QUICK_HISTORY_OPTIONS"
                  :key="opt.value"
                  size="tiny"
                  :type="selectedQuickInterval === opt.value ? 'primary' : 'default'"
                  :secondary="selectedQuickInterval !== opt.value"
                  @click="selectedQuickInterval = opt.value"
                >
                  {{ opt.label }}
                </NButton>
              </div>

              <div v-else class="pt-1">
                <NDatePicker v-model:value="customDateRange" type="datetimerange" size="small" clearable />
              </div>
            </div>
          </NTabPane>
        </NTabs>

        <!-- 通用高级配置：聚合与时区 -->
        <div class="mt-3 border-t border-gray-100 pt-2 text-xs">
          <div class="grid grid-cols-2 gap-2">
            <NFormItem label="聚合函数" :show-feedback="false" size="small">
              <NSelect v-model:value="selectedAggregationFunc" :options="AGGREGATION_OPTIONS" size="small" />
            </NFormItem>
            <NFormItem label="采样分组粒度" :show-feedback="false" size="small">
              <NSelect
                v-model:value="selectedGroupingInterval"
                :options="GROUPING_OPTIONS"
                :disabled="selectedAggregationFunc === 'none'"
                size="small"
              />
            </NFormItem>
          </div>

          <NFormItem label="展示时区" :show-feedback="false" size="small" class="mt-2">
            <NSelect v-model:value="selectedTimezone" :options="TIMEZONE_OPTIONS" size="small" />
          </NFormItem>
        </div>

        <!-- 底部操作按钮 -->
        <div class="mt-4 flex justify-end gap-2 border-t border-gray-100 pt-2">
          <NButton size="small" @click="handleCancel">取消</NButton>
          <NButton type="primary" size="small" @click="applyChanges">应用</NButton>
        </div>
      </div>
    </NPopover>
  </div>
</template>

<style scoped>
.timewindow-btn {
  font-weight: 500;
  transition: all 0.2s ease;
}
</style>
