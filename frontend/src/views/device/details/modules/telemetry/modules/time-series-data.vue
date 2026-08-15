<script setup lang="ts">
import { defineAsyncComponent, onMounted, reactive, ref, watch } from 'vue'
import { NDatePicker, NSelect, NSpace } from 'naive-ui'
import { useFullscreen } from '@vueuse/core'
import dayjs from 'dayjs'
import { telemetryDataHistoryList } from '@/service/api/device'
import { $t } from '@/locales'
import { message } from '@/utils/common/discrete'
import {
  applyAggregationWeight,
  applyAggregationWindowChange,
  applyChartSeriesType,
  buildTelemetryCsv,
  calculateTimeWeight,
  projectTelemetryHistory,
  resolveNavigatedCustomRange,
  type AggregateFunction,
  type AggregateWindow,
  type SelectedOptionState,
  type TelemetryHistoryPoint,
  type TimeRange
} from './time-series-data-state'
import { useLoading } from '~/packages/hooks'

const ChartComponent = defineAsyncComponent(() => import('./ChartComponent.vue'))

interface Props {
  deviceId: string
  theKey: string
  theName: string
  theUnit: string
}

const props = defineProps<Props>()

const tableData = ref<TelemetryHistoryPoint[]>([])
const chartRef = ref()
const { isFullscreen, toggle } = useFullscreen(chartRef)
const datePickerValue = ref<[number, number] | null>(null)
const avgValue = ref<number | null>(null)
const maxValue = ref<number | null>(null)
const minValue = ref<number | null>(null)
const latestTimestamp = ref<number | null>(null)
const totalSampleCount = ref(0)
const validSampleCount = ref(0)
const { loading, startLoading, endLoading } = useLoading()

const selectedOption = ref<SelectedOptionState>({
  device_id: props.deviceId,
  key: props.theKey,
  aggregate_window: 'no_aggregate',
  time_range: 'last_1h',
  start_time: undefined,
  end_time: undefined,
  aggregate_function: undefined
})

const columns = [
  {
    title: $t('common.time'),
    key: 'x',
    render: (row: TelemetryHistoryPoint) => dayjs(row.x).format('YYYY-MM-DD HH:mm:ss')
  },
  {
    title: `${props.theName || props.theKey}${props.theUnit ? `(${props.theUnit})` : ''}`,
    key: 'y'
  }
]

const pagination = reactive({
  page: 1,
  pageSize: 10,
  pageCount: 1,
  onChange: (page: number) => {
    pagination.page = page
  }
})

const initialOptions = ref({
  baseOption: undefined,
  title: {
    text: props.theName || props.theKey
  },
  options: [],
  tooltip: {
    trigger: 'axis',
    formatter(params: Array<{ value: [number, number | null]; marker: string }>) {
      let result = `${dayjs(params[0].value[0]).format('YYYY-MM-DD HH:mm:ss')}<br/>`
      params.forEach((param) => {
        result += `${param.marker} ${props.theName || props.theKey}: ${param.value[1]}${props.theUnit || ''}<br/>`
      })
      return result
    }
  },
  legend: {
    data: ['test_key']
  },
  dataZoom: [
    {
      type: 'slider',
      show: true,
      xAxisIndex: [0],
      start: 0,
      end: 100
    },
    {
      type: 'inside',
      xAxisIndex: [0],
      start: 0,
      end: 100
    }
  ],
  grid: {
    left: '3%',
    right: '4%',
    bottom: '3%',
    containLabel: true
  },
  toolbox: {
    right: 44,
    feature: {
      myTool1: {
        show: true,
        title: $t('common.switchLineChart'),
        icon: 'path://M-7.5 -1.036L-5.428 -1.036L-2.714 -7.4562L-0.5545 3.6333L2.2763 -2.1158L3.1518 1.6196L7.5 1.6196M-7.5 7.4562L7.5 7.4562',
        onclick: () => {
          const nextState = applyChartSeriesType(initialOptions.value.series, 'line', 'common.alreadyCurveChart')
          initialOptions.value.series = nextState.series ?? []
          if (nextState.duplicateMessageKey) {
            message.destroyAll()
            message.info($t(nextState.duplicateMessageKey))
          }
        }
      },
      myTool2: {
        show: true,
        title: $t('common.switchBarChart'),
        icon: 'path://M-6.2277 -1.9018L-3.5491 -1.9018L-3.5491 4.8214L-6.2277 4.8214L-6.2277 -1.9018ZM-1.3527 -4.5536L1.3259 -4.5536L1.3259 4.8214L-1.3527 4.8214L-1.3527 -4.5536ZM3.5491 -7.5L6.2277 -7.5L6.2277 4.8214L3.5491 4.8214L3.5491 -7.5ZM-7.192 7.5L7.192 7.5\n',
        onclick: () => {
          const nextState = applyChartSeriesType(initialOptions.value.series, 'bar', 'common.alreadyToChart')
          initialOptions.value.series = nextState.series ?? []
          if (nextState.duplicateMessageKey) {
            window.NMessage.destroyAll()
            message.info($t(nextState.duplicateMessageKey))
          }
        }
      },
      myTool3: {
        show: true,
        title: $t('common.alreadyToChart'),
        icon: 'path://M6 6V42H42 M20 24C22.2091 24 24 22.2091 24 20C24 17.7909 22.2091 16 20 16C17.7909 16 16 17.7909 16 20C16 22.2091 17.7909 24 20 24Z M37 16C39.7614 16 42 13.7614 42 11C42 8.23858 39.7614 6 37 6C34.2386 6 32 8.23858 32 11C32 13.7614 34.2386 16 37 16Z M15 36C16.6569 36 18 34.6569 18 33C18 31.3431 16.6569 30 15 30C13.3431 30 12 31.3431 12 33C12 34.6569 13.3431 36 15 36Z M33 32C34.6569 32 36 30.6569 36 29C36 27.3431 34.6569 26 33 26C31.3431 26 30 27.3431 30 29C30 30.6569 31.3431 32 33 32Z\n',
        onclick: () => {
          const nextState = applyChartSeriesType(initialOptions.value.series, 'scatter', 'common.alreadyScatterPlot')
          initialOptions.value.series = nextState.series ?? []
          if (nextState.duplicateMessageKey) {
            window.NMessage.destroyAll()
            message.info($t(nextState.duplicateMessageKey))
          }
        }
      }
    }
  },
  xAxis: {
    boundaryGap: false,
    type: 'time' as const
  },
  yAxis: {
    type: 'value',
    scale: true
  },
  series: [
    {
      data: [] as Array<[number | null, number | null]>,
      type: 'line',
      smooth: true
    }
  ]
})

const timeOptions: Array<{ label: string; value: TimeRange }> = [
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

const timeWeighting: Record<TimeRange, number> = {
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

const aggregationIntervalOptions: Array<{ label: string; value: AggregateWindow; disabled: boolean }> = [
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

const statisticsOptions: Array<{ label: string; value: AggregateFunction }> = [
  { label: $t('common.average'), value: 'avg' },
  { label: $t('generate.max-value'), value: 'max' },
  { label: $t('generate.min-value'), value: 'min' },
  { label: $t('generate.sum'), value: 'sum' },
  { label: $t('generate.diff'), value: 'diff' }
]

const applyAggregationWeighting = (weight: number) => {
  const result = applyAggregationWeight(aggregationIntervalOptions, selectedOption.value, weight)
  aggregationIntervalOptions.splice(0, aggregationIntervalOptions.length, ...result.aggregationIntervalOptions)
  selectedOption.value = result.selectedOption
}

const navigateTime = (direction: string) => {
  const nextRange = resolveNavigatedCustomRange(selectedOption.value.start_time, direction)
  selectedOption.value = {
    ...selectedOption.value,
    ...nextRange.selectedOption
  }
  datePickerValue.value = nextRange.datePickerValue
}

watch(
  selectedOption,
  async (value) => {
    if (value.time_range === 'custom' && (!value.start_time || !value.end_time)) {
      window.NMessage.destroyAll()
      message.info($t('common.rangeMustSelected'))
      return
    }

    startLoading()
    try {
      const { data, error } = await telemetryDataHistoryList({ ...value })

      if (!error && data && initialOptions.value.series) {
        const projection = projectTelemetryHistory(data as TelemetryHistoryPoint[])
        tableData.value = projection.tableData
        avgValue.value = projection.summary.avgValue
        maxValue.value = projection.summary.maxValue
        minValue.value = projection.summary.minValue
        latestTimestamp.value = projection.summary.latestTimestamp
        totalSampleCount.value = projection.summary.totalSampleCount
        validSampleCount.value = projection.summary.validSampleCount
        initialOptions.value.series.forEach((series) => {
          series.data = projection.seriesData
        })
      }
    } finally {
      endLoading()
    }
  },
  { deep: true }
)

const onTimeRangeChange = (value: TimeRange) => {
  selectedOption.value.time_range = value
  if (value !== 'custom') {
    selectedOption.value.start_time = undefined
    selectedOption.value.end_time = undefined
    datePickerValue.value = null
  }
  applyAggregationWeighting(timeWeighting[value])
}

const onCustomDateChange = (value: [number, number] | null) => {
  if (!value) {
    return
  }

  selectedOption.value.start_time = value[0]
  selectedOption.value.end_time = value[1]
  selectedOption.value.time_range = 'custom'
  datePickerValue.value = value
  applyAggregationWeighting(calculateTimeWeight(value[0], value[1]))
}

const onAggregationChange = (value: AggregateWindow) => {
  selectedOption.value = applyAggregationWindowChange(selectedOption.value, value)
}

const onStatisticsChange = (value: AggregateFunction) => {
  selectedOption.value.aggregate_function = value
}

const initData = () => {
  selectedOption.value = { ...selectedOption.value, time_range: 'last_1h' }
}

const exportData = () => {
  const csvRows = [
    [$t('common.time'), `${props.theName || props.theKey}${props.theUnit ? `(${props.theUnit})` : ''}`],
    ...tableData.value.map((item) => [dayjs(item.x).format('YYYY-MM-DD HH:mm:ss'), item.y])
  ]
  const csvContent = buildTelemetryCsv(csvRows)
  const csvBlob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
  const csvUrl = URL.createObjectURL(csvBlob)
  const link = document.createElement('a')
  link.href = csvUrl
  link.download = 'telemetry_data.csv'
  link.click()
  URL.revokeObjectURL(csvUrl)
  link.remove()
}

onMounted(() => {
  initData()
})
</script>

<template>
  <NSpace vertical>
    <div class="w-full flex flex-row flex-wrap">
      <div class="time-range flex flex-col items-center">
        <div class="w-full flex flex-row items-center">
          <span>{{ $t('common.timeFrame') }}:</span>
          <NSelect
            v-model:value="selectedOption.time_range"
            :options="timeOptions"
            :consistent-menu-width="false"
            class="select-item mr-2"
            @update:value="onTimeRangeChange"
          />
          <NDatePicker
            v-model:value="datePickerValue"
            type="datetimerange"
            value-format="timestamp"
            class="flex-1"
            format="yyyy-MM-dd HH:mm"
            :time-picker-props="{
              format: 'HH',
              isHourDisabled: () => false,
              isMinuteDisabled: () => true,
              isSecondDisabled: () => true
            }"
            @update:value="onCustomDateChange"
          />
        </div>
        <div class="mt-2 w-full flex flex-row flex-wrap justify-between pl-72px">
          <NButton @click="navigateTime('prevMonth')">{{ $t('card.lastOneMonth') }}</NButton>
          <NButton @click="navigateTime('prevDay')">{{ $t('card.yesterday') }}</NButton>
          <NButton @click="navigateTime('prevHour')">{{ $t('card.lastOneHour') }}</NButton>
          <NButton @click="navigateTime('nextHour')">{{ $t('card.nextOneHour') }}</NButton>
          <NButton @click="navigateTime('nextDay')">{{ $t('card.tomorrow') }}</NButton>
          <NButton @click="navigateTime('nextMonth')">{{ $t('card.nextOneMonth') }}</NButton>
        </div>
      </div>
      <div class="aggregation-range flex flex-row pl-2">
        <span class="pt-1">{{ $t('card.aggregationScope') }}:</span>
        <NSelect
          v-model:value="selectedOption.aggregate_window"
          :options="aggregationIntervalOptions"
          :consistent-menu-width="false"
          class="select-item mr-2"
          @update:value="onAggregationChange"
        />
        <span v-if="selectedOption.aggregate_window !== 'no_aggregate'" class="pt-1">
          {{ $t('card.aggregationMethod') }}:
        </span>
        <NSelect
          v-if="selectedOption.aggregate_window !== 'no_aggregate'"
          v-model:value="selectedOption.aggregate_function"
          :options="statisticsOptions"
          :consistent-menu-width="false"
          class="select-item"
          @update:value="onStatisticsChange"
        />
        <NButton class="ml-auto" @click="exportData">{{ $t('card.exportData') }}</NButton>
      </div>
    </div>
    <div class="container-table-chart">
      <n-data-table
        class="telemetry-table"
        :loading="loading"
        :columns="columns"
        :data="tableData"
        :pagination="pagination"
      />
      <div ref="chartRef" class="telemetry-chart relative m-0 p-0">
        <div :class="`${isFullscreen ? 'h-full' : 'chart-height'} p-2`">
          <ChartComponent :initial-options="initialOptions" />
        </div>
        <div class="absolute right-0px top-5px">
          <FullScreen v-if="!isFullscreen" :full="isFullscreen" @click="toggle" />
        </div>
        <div class="flex flex-row justify-between pl-4 pr-4 font-bold">
          <span>{{ $t('card.average') }}: {{ avgValue !== null ? avgValue.toFixed(2) : '-' }}</span>
          <span>{{ $t('card.maxValue') }}: {{ maxValue !== null ? maxValue : '-' }}</span>
          <span>{{ $t('card.minValue') }}: {{ minValue !== null ? minValue : '-' }}</span>
        </div>
        <div class="summary-meta flex flex-row justify-between pl-4 pr-4">
          <span>
            {{ $t('card.latestPoint') }}:
            {{ latestTimestamp !== null ? dayjs(latestTimestamp).format('YYYY-MM-DD HH:mm:ss') : '-' }}
          </span>
          <span>
            {{ $t('card.validDataPoints') }}: {{ validSampleCount }}/{{ totalSampleCount }}
          </span>
        </div>
      </div>
    </div>
  </NSpace>
</template>

<style scoped>
.container-table-chart {
  display: flex;
  flex-direction: row;
}

.telemetry-chart {
  width: 60%;
}

.telemetry-table {
  width: 40%;
  min-height: 200px;
}

.chart-height {
  height: calc(100% - 24px);
  min-height: 200px;
}

.time-range {
  width: 60%;
  flex-wrap: wrap;
}

.aggregation-range {
  width: 40%;
  flex-wrap: wrap;
}

@media (max-width: 768px) {
  .container-table-chart {
    flex-direction: column;
  }

  .telemetry-chart {
    width: 100%;
  }

  .telemetry-table {
    width: 100%;
  }

  .chart-height {
    height: 400px;
  }

  .time-range {
    width: 100%;
  }

  .aggregation-range {
    width: 100%;
  }
}

.select-item {
  flex: 0;
}

.summary-meta {
  gap: 8px;
  color: var(--text-color-2);
  font-size: 12px;
}
</style>
