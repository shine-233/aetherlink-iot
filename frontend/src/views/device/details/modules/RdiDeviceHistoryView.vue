<!--
  文件用途：RDI 设备顶层历史数据视图，承接客户要求的独立 History Data tab。
  关键输入：只接收当前设备 ID；时间范围、序列选择、统计和导出状态由模块内部管理。
  主要副作用：挂载或设备切换时查询最近一小时历史，用户可重新加载或发起 CSV/Excel 导出。
  维护注意：该模块是 RDI chart/history seam 的专用 adapter，不应重新依赖 ThingsVis 模板图表能力。
-->
<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref, watch } from 'vue'
import { useAppStore } from '@/store/modules/app'
import { labels } from './rdi/constants/rdi-labels'
import type { LabelKey } from './rdi/constants/rdi-labels'
import { useRdiHistory } from './rdi/composables/useRdiHistory'
import { useRdiTemperatureUnit } from './rdi/composables/useRdiTemperatureUnit'

const ChartComponent = defineAsyncComponent(() => import('./telemetry/modules/ChartComponent.vue'))

const props = defineProps<{
  id: string
}>()

const appStore = useAppStore()
const text = computed(() => labels[appStore.locale] || labels['en-US'])
const t = (key: LabelKey) => text.value[key]

const temperatureUnit = useRdiTemperatureUnit()
const hasLoadedHistory = ref(false)
let historyViewLoadSequence = 0

const {
  energyLoading,
  historyExportLoading,
  energyRange,
  energyCustomRange,
  historyChartSeriesKeys,
  historyExportKey,
  historyExportFormat,
  energyStats,
  historyChartOptions,
  energyRangeOptions,
  historyChartSeriesOptions,
  historyExportKeyOptions,
  historyExportFormatOptions,
  failedHistorySeriesLabels,
  partialHistorySeriesLabels,
  gappedHistorySeriesLabels,
  hasHistoryFailures,
  hasHistoryChartData,
  energyStatisticsAvailable,
  formatEnergyValue,
  loadEnergyStatistics,
  exportHistoryData
} = useRdiHistory(
  () => props.id,
  () => temperatureUnit.value,
  t
)

async function loadHistory() {
  if (!props.id) return
  const loadingDeviceId = props.id
  const loadingSequence = ++historyViewLoadSequence
  await loadEnergyStatistics()
  if (loadingDeviceId === props.id && loadingSequence === historyViewLoadSequence) {
    hasLoadedHistory.value = true
  }
}

onMounted(() => {
  void loadHistory()
})

watch(
  () => props.id,
  (nextId, previousId) => {
    if (nextId === previousId) return
    historyViewLoadSequence += 1
    hasLoadedHistory.value = false
    if (nextId) void loadHistory()
  }
)
</script>

<template>
  <section class="rdi-device-history-view">
    <div class="rdi-history-toolbar">
      <NSelect v-model:value="energyRange" :options="energyRangeOptions" class="rdi-history-select" />
      <NSelect
        v-model:value="historyChartSeriesKeys"
        multiple
        :options="historyChartSeriesOptions"
        :placeholder="t('historyKey')"
        class="rdi-history-select rdi-history-series-select"
        max-tag-count="responsive"
      />
      <NSelect v-model:value="historyExportKey" :options="historyExportKeyOptions" class="rdi-history-select" />
      <NSelect
        v-model:value="historyExportFormat"
        :options="historyExportFormatOptions"
        :aria-label="t('exportFormat')"
        class="rdi-history-select"
      />
      <NDatePicker
        v-if="energyRange === 'custom'"
        v-model:value="energyCustomRange"
        type="datetimerange"
        class="rdi-history-date-range"
      />
      <NButton :loading="energyLoading" @click="loadHistory">{{ t('load') }}</NButton>
      <NButton :loading="historyExportLoading" @click="exportHistoryData">{{ t('exportData') }}</NButton>
    </div>

    <div class="rdi-history-stats">
      <div class="rdi-history-stat">
        <span>{{ t('latest') }}</span>
        <strong>{{ energyStatisticsAvailable ? formatEnergyValue(energyStats.latest) : '--' }}</strong>
      </div>
      <div class="rdi-history-stat">
        <span>{{ t('delta') }}</span>
        <strong>{{ energyStatisticsAvailable ? formatEnergyValue(energyStats.delta) : '--' }}</strong>
      </div>
      <div class="rdi-history-stat">
        <span>{{ t('minMax') }}</span>
        <strong v-if="energyStatisticsAvailable">
          {{ formatEnergyValue(energyStats.min) }} / {{ formatEnergyValue(energyStats.max) }}
        </strong>
        <strong v-else>-- / --</strong>
      </div>
      <div class="rdi-history-stat">
        <span>{{ t('dataPoints') }}</span>
        <strong>{{ energyStatisticsAvailable ? energyStats.sample_count : '--' }}</strong>
      </div>
    </div>

    <div class="rdi-history-evidence">
      <NAlert v-if="failedHistorySeriesLabels.length" type="error" :title="t('historyLoadFailed')">
        {{ failedHistorySeriesLabels.join(', ') }}
      </NAlert>
      <NAlert v-if="partialHistorySeriesLabels.length" type="warning" :title="t('historyPartialData')">
        {{ partialHistorySeriesLabels.join(', ') }}
      </NAlert>
      <NAlert v-if="gappedHistorySeriesLabels.length" type="warning" :title="t('historyGapDetected')">
        {{ gappedHistorySeriesLabels.join(', ') }}. {{ t('historyGapNotConnected') }}
      </NAlert>
    </div>

    <NSpin :show="energyLoading">
      <div class="rdi-history-chart">
        <ChartComponent v-if="hasLoadedHistory && hasHistoryChartData" :initial-options="historyChartOptions" />
        <NEmpty
          v-else
          :description="hasLoadedHistory && hasHistoryFailures ? t('historyLoadFailed') : t('empty')"
        />
      </div>
    </NSpin>
  </section>
</template>

<style scoped>
.rdi-device-history-view {
  padding: 8px 0 16px;
}

.rdi-history-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.rdi-history-select {
  width: 168px;
}

.rdi-history-series-select {
  width: 260px;
}

.rdi-history-date-range {
  width: 320px;
}

.rdi-history-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 10px;
}

.rdi-history-stat {
  display: flex;
  min-height: 56px;
  flex-direction: column;
  justify-content: center;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 8px 10px;
  background: #f8fafc;
}

.rdi-history-stat span {
  color: #667085;
  font-size: 12px;
}

.rdi-history-stat strong {
  margin-top: 4px;
  font-size: 16px;
}

.rdi-history-evidence {
  display: grid;
  gap: 8px;
  margin-top: 12px;
}

.rdi-history-chart {
  width: 100%;
  height: 360px;
  min-height: 320px;
  margin-top: 16px;
}

@media (max-width: 900px) {
  .rdi-history-date-range,
  .rdi-history-select,
  .rdi-history-series-select {
    width: 100%;
  }
}
</style>
