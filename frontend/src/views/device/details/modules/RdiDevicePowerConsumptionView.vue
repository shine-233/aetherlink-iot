<!--
  文件用途：REQ-48 客户可见的 RDI 用电量统计 Tab。
  关键输入：只接收当前设备 ID；温标偏好、时段和统计状态由模块内部管理。
  数据来源：复用 useRdiHistory 的 electricity_consumption 遥测查询，
  由后端 /rdi/devices/:id/history 提供累计用电量点位（RDI 遥测契约里包含该字段）。
  维护注意：
  1. 只加载 electricity_consumption 一条曲线，避免和 History Data tab 内的多曲线视图冲突；
  2. 切换设备时清空 hasLoaded 并重置系列过滤，防止旧数据串页；
  3. 自定义时段与 REQ-04 保持一致，不做本地 30 天硬限，由 useRdiHistory 分页/上限治理。
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

defineOptions({
  name: 'RdiDevicePowerConsumptionView',
  inheritAttrs: false
})

const appStore = useAppStore()
const text = computed(() => labels[appStore.locale] || labels['en-US'])
const t = (key: LabelKey) => text.value[key]

const temperatureUnit = useRdiTemperatureUnit()
const hasLoaded = ref(false)
let loadSequence = 0

const {
  energyLoading,
  energyRange,
  energyCustomRange,
  historyChartSeriesKeys,
  energyStats,
  historyChartOptions,
  formatEnergyValue,
  loadEnergyStatistics,
  hasHistoryChartData,
  energyStatisticsAvailable
} = useRdiHistory(
  () => props.id,
  () => temperatureUnit.value,
  t
)

// The customer-facing power tab only tracks cumulative electricity consumption.
historyChartSeriesKeys.value = ['electricity_consumption']
// Default to Today so the tab lands on the most recent 24 hours by default.
energyRange.value = 'last_24h'

const rangeOptions = computed(() => [
  { label: t('today'), value: 'last_24h' },
  { label: t('thisWeek'), value: 'last_7d' },
  { label: t('thisMonth'), value: 'last_30d' },
  { label: t('customRange'), value: 'custom' }
])

async function load() {
  if (!props.id) return
  const sequence = ++loadSequence
  historyChartSeriesKeys.value = ['electricity_consumption']
  await loadEnergyStatistics()
  if (sequence === loadSequence) hasLoaded.value = true
}

onMounted(() => {
  void load()
})

watch(
  () => props.id,
  (nextId, previousId) => {
    if (nextId === previousId) return
    loadSequence += 1
    hasLoaded.value = false
    historyChartSeriesKeys.value = ['electricity_consumption']
    if (nextId) void load()
  }
)
</script>

<template>
  <section class="rdi-power-consumption-view">
    <header class="rdi-power-header">
      <div class="rdi-power-title">{{ t('powerUsage') }}</div>
      <div class="rdi-power-description">{{ t('powerUsageDescription') }}</div>
    </header>

    <div class="rdi-power-toolbar">
      <NSelect v-model:value="energyRange" :options="rangeOptions" class="rdi-power-select" />
      <NDatePicker
        v-if="energyRange === 'custom'"
        v-model:value="energyCustomRange"
        type="datetimerange"
        class="rdi-power-date-range"
      />
      <NButton :loading="energyLoading" @click="load">{{ t('load') }}</NButton>
    </div>

    <div class="rdi-power-stats">
      <div class="rdi-power-stat">
        <span>{{ t('cumulativeUsage') }}</span>
        <strong>{{ energyStatisticsAvailable ? formatEnergyValue(energyStats.delta) : '--' }}</strong>
      </div>
      <div class="rdi-power-stat">
        <span>{{ t('latest') }}</span>
        <strong>{{ energyStatisticsAvailable ? formatEnergyValue(energyStats.latest) : '--' }}</strong>
      </div>
      <div class="rdi-power-stat">
        <span>{{ t('minMax') }}</span>
        <strong v-if="energyStatisticsAvailable">
          {{ formatEnergyValue(energyStats.min) }} / {{ formatEnergyValue(energyStats.max) }}
        </strong>
        <strong v-else>-- / --</strong>
      </div>
      <div class="rdi-power-stat">
        <span>{{ t('dataPoints') }}</span>
        <strong>{{ energyStatisticsAvailable ? energyStats.sample_count : '--' }}</strong>
      </div>
    </div>

    <NSpin :show="energyLoading">
      <div class="rdi-power-chart">
        <ChartComponent v-if="hasLoaded && hasHistoryChartData" :initial-options="historyChartOptions" />
        <NEmpty v-else :description="t('empty')" />
      </div>
    </NSpin>
  </section>
</template>

<style scoped>
.rdi-power-consumption-view {
  padding: 8px 0 16px;
}

.rdi-power-header {
  margin-bottom: 12px;
}

.rdi-power-title {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.3;
  color: #0f172a;
}

.rdi-power-description {
  margin-top: 4px;
  color: #667085;
  font-size: 12px;
  line-height: 1.4;
}

.rdi-power-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.rdi-power-select {
  width: 168px;
}

.rdi-power-date-range {
  width: 320px;
}

.rdi-power-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 10px;
}

.rdi-power-stat {
  display: flex;
  min-height: 56px;
  flex-direction: column;
  justify-content: center;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 8px 10px;
  background: #f8fafc;
}

.rdi-power-stat span {
  color: #667085;
  font-size: 12px;
}

.rdi-power-stat strong {
  margin-top: 4px;
  color: #0f172a;
  font-size: 16px;
}

.rdi-power-chart {
  width: 100%;
  height: 360px;
  min-height: 320px;
  margin-top: 16px;
}

@media (max-width: 900px) {
  .rdi-power-date-range,
  .rdi-power-select {
    width: 100%;
  }
}
</style>
