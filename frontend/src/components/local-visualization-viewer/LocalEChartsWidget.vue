<script setup lang="ts">
import { computed, watch } from 'vue'
import { createEChartsHook } from '@/hooks/chart/use-echarts'
import { buildChartOption } from './data'
import type { ChartWidgetConfig, LocalViewerFields } from './types'

const props = defineProps<{
  type: 'line-chart' | 'bar-chart'
  config: ChartWidgetConfig
  fields: LocalViewerFields
}>()

const built = computed(() => buildChartOption(props.type, props.config, props.fields))
const { domRef, updateOptions } = createEChartsHook(() => built.value.option, {}, {
  hideLoadingAfterDefaultRender: true,
  requiredExtensions: ['LineChart', 'BarChart', 'TitleComponent', 'TooltipComponent', 'GridComponent']
})

watch(
  () => built.value.option,
  option => updateOptions(() => option),
  { deep: true }
)
</script>

<template>
  <div class="local-chart-widget">
    <div v-if="!built.available" class="local-widget-unavailable">Unavailable</div>
    <div v-else ref="domRef" class="local-chart-canvas" aria-label="Local dashboard chart"></div>
  </div>
</template>

<style scoped>
.local-chart-widget,
.local-chart-canvas {
  width: 100%;
  height: 100%;
  min-height: 80px;
}

.local-widget-unavailable {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #8c8c8c;
}
</style>
