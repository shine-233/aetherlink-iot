<!--
文件用途：RDI 概览的年度告警趋势卡（年份选择 + 月度柱状图）。
核心逻辑：年份双向绑定，切换年份时 emit change 由父级拉取趋势数据；图表组件异步加载。
-->
<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { NCard, NSelect, NSpin } from 'naive-ui'
import { $t } from '@/locales'

defineProps<{
  loading: boolean
  yearOptions: Array<{ label: string; value: number }>
  chartOptions: Record<string, unknown>
}>()

const year = defineModel<number>('year', { required: true })
defineEmits<{ change: [] }>()

const ChartComponent = defineAsyncComponent(() => import('@/components/custom/ChartComponent.vue'))
</script>

<template>
  <NCard :title="$t('rdi.overview.alarmTrendTitle')" :bordered="false">
    <NSpin :show="loading">
      <div class="alarm-trend-card">
        <div class="alarm-trend-summary">
          <div class="alarm-trend-year-control">
            <strong>{{ $t('rdi.overview.alarmTrendYear') }}</strong>
            <NSelect
              v-model:value="year"
              :options="yearOptions"
              class="alarm-trend-year-select"
              @update:value="$emit('change')"
            />
          </div>
          <span>{{ $t('rdi.overview.alarmTrendDesc') }}</span>
        </div>
        <div class="alarm-trend-chart">
          <ChartComponent :initial-options="chartOptions" />
        </div>
      </div>
    </NSpin>
  </NCard>
</template>

<style scoped>
.alarm-trend-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.alarm-trend-summary {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.alarm-trend-year-control {
  display: flex;
  align-items: center;
  gap: 10px;
}

.alarm-trend-year-select {
  width: 120px;
}

.alarm-trend-summary strong {
  color: #111827;
  font-size: 15px;
}

.alarm-trend-summary span {
  color: #6b7280;
  font-size: 13px;
}

.alarm-trend-chart {
  height: 280px;
}

@media (max-width: 560px) {
  .alarm-trend-summary {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
