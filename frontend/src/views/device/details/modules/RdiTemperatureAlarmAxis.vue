<!--
  文件用途: RDI 温度告警轴展示组件。
  核心逻辑: 根据上下限、当前值和温度单位绘制温度区间、告警阈值与当前位置。
  关键注意事项: 数值范围和单位转换错误会导致告警可视化误导，空值需要稳定降级。
  重构建议: 将坐标计算抽为纯函数并覆盖上下限反转、华氏单位和无数据场景。
-->
<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { ECharts } from 'echarts/core'
import { createEChartsInstance, registerEChartsExtensions } from '@/utils/echarts/echarts-manager'

const props = withDefaults(
  defineProps<{
    lower: number
    upper: number
    current?: number | string | null
    min?: number
    max?: number
    unit?: 'C' | 'F'
    lowerLabel?: string
    upperLabel?: string
    currentLabel?: string
  }>(),
  {
    min: -40,
    max: 125,
    unit: 'C',
    current: null,
    lowerLabel: 'Lower',
    upperLabel: 'Upper',
    currentLabel: 'Current'
  }
)

const emit = defineEmits<{
  'update:lower': [value: number]
  'update:upper': [value: number]
}>()

const chartRef = ref<HTMLElement | null>(null)

let chart: ECharts | null = null
let resizeObserver: ResizeObserver | null = null

function clamp(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value))
}

function normalizeAxisValue(value: unknown, fallback: number) {
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : fallback
}

function roundAxisValue(value: number) {
  return Math.round(value)
}

function displayTemperature(value: number) {
  const converted = props.unit === 'F' ? (value * 9) / 5 + 32 : value
  return `${converted.toFixed(0)}°${props.unit}`
}

function currentNumeric() {
  const numeric = Number(props.current)
  return Number.isFinite(numeric) ? clamp(numeric, props.min, props.max) : null
}

function toPixel(value: number) {
  if (!chart) return 0
  return Number(chart.convertToPixel({ xAxisIndex: 0 }, value))
}

function fromPixel(x: number) {
  if (!chart) return props.min
  const value = Number(chart.convertFromPixel({ xAxisIndex: 0 }, x))
  return clamp(value, props.min, props.max)
}

function emitBoundary(kind: 'lower' | 'upper', pixelX: number) {
  const lower = normalizeAxisValue(props.lower, props.min)
  const upper = normalizeAxisValue(props.upper, props.max)
  const next = roundAxisValue(fromPixel(pixelX))

  if (kind === 'lower') {
    emit('update:lower', Math.min(next, upper))
    return
  }

  emit('update:upper', Math.max(next, lower))
}

function dragHandler(kind: 'lower' | 'upper') {
  return function handleDrag(event: { target?: { x?: number; position?: number[] } }) {
    const target = event.target || {}
    const pixelX = Array.isArray(target.position) ? target.position[0] : Number(target.x)
    if (Number.isFinite(pixelX)) {
      emitBoundary(kind, pixelX)
    }
  }
}

function buildGraphics() {
  const lower = clamp(normalizeAxisValue(props.lower, props.min), props.min, props.max)
  const upper = clamp(normalizeAxisValue(props.upper, props.max), props.min, props.max)
  const start = Math.min(lower, upper)
  const end = Math.max(lower, upper)
  const axisY = Number(chart?.convertToPixel({ yAxisIndex: 0 }, 0.5) || 56)
  const minX = toPixel(props.min)
  const maxX = toPixel(props.max)
  const lowerX = toPixel(start)
  const upperX = toPixel(end)
  const current = currentNumeric()
  const currentX = current === null ? null : toPixel(current)

  const graphics: any[] = [
    {
      id: 'axis-hit-area',
      type: 'rect',
      silent: true,
      shape: { x: minX, y: axisY - 18, width: maxX - minX, height: 36 },
      style: { fill: 'rgba(148, 163, 184, 0.08)' }
    },
    {
      id: 'alarm-band',
      type: 'rect',
      silent: true,
      shape: { x: lowerX, y: axisY - 18, width: Math.max(0, upperX - lowerX), height: 36 },
      style: {
        fill: 'rgba(148, 163, 184, 0.36)',
        stroke: 'rgba(71, 85, 105, 0.42)',
        lineWidth: 1
      }
    },
    {
      id: 'lower-handle',
      type: 'circle',
      draggable: true,
      cursor: 'ew-resize',
      x: lowerX,
      y: axisY,
      shape: { r: 8 },
      style: { fill: '#2563eb', stroke: '#fff', lineWidth: 2 },
      z: 10,
      ondrag: dragHandler('lower')
    },
    {
      id: 'upper-handle',
      type: 'circle',
      draggable: true,
      cursor: 'ew-resize',
      x: upperX,
      y: axisY,
      shape: { r: 8 },
      style: { fill: '#2563eb', stroke: '#fff', lineWidth: 2 },
      z: 10,
      ondrag: dragHandler('upper')
    },
    {
      id: 'lower-label',
      type: 'text',
      silent: true,
      x: lowerX - 20,
      y: axisY + 22,
      style: {
        text: `${props.lowerLabel}: ${displayTemperature(start)}`,
        fill: '#475569',
        fontSize: 11
      }
    },
    {
      id: 'upper-label',
      type: 'text',
      silent: true,
      x: upperX - 20,
      y: axisY + 22,
      style: {
        text: `${props.upperLabel}: ${displayTemperature(end)}`,
        fill: '#475569',
        fontSize: 11
      }
    }
  ]

  if (current !== null && currentX !== null) {
    graphics.push(
      {
        id: 'current-marker',
        type: 'line',
        silent: true,
        shape: { x1: currentX, y1: axisY - 28, x2: currentX, y2: axisY + 28 },
        style: { stroke: '#ef4444', lineWidth: 2 },
        z: 8
      },
      {
        id: 'current-label',
        type: 'text',
        silent: true,
        x: clamp(currentX - 32, minX, maxX - 64),
        y: axisY - 44,
        style: {
          text: `${props.currentLabel}: ${displayTemperature(current)}`,
          fill: '#ef4444',
          fontSize: 12,
          fontWeight: 600
        }
      }
    )
  }

  return graphics
}

function renderChart() {
  if (!chart) return

  chart.setOption(
    {
      animation: false,
      grid: { left: 34, right: 34, top: 24, bottom: 36 },
      xAxis: {
        type: 'value',
        min: props.min,
        max: props.max,
        splitNumber: 5,
        axisLabel: {
          formatter: (value: number) => displayTemperature(value)
        },
        axisTick: { alignWithLabel: true },
        splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.22)' } }
      },
      yAxis: {
        type: 'value',
        min: 0,
        max: 1,
        show: false
      },
      series: [
        {
          type: 'line',
          data: [
            [props.min, 0.5],
            [props.max, 0.5]
          ],
          symbol: 'none',
          lineStyle: { width: 2, color: '#94a3b8' },
          silent: true
        }
      ],
      graphic: buildGraphics()
    },
    true
  )
}

async function initChart() {
  await nextTick()
  if (!chartRef.value || chart) return

  await registerEChartsExtensions(['graphic'])
  if (!chartRef.value || chart) return

  chart = createEChartsInstance(chartRef.value, undefined, { renderer: 'canvas' })
  renderChart()

  resizeObserver = new ResizeObserver(() => {
    chart?.resize()
    renderChart()
  })
  resizeObserver.observe(chartRef.value)
}

onMounted(() => {
  initChart()
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
  chart?.dispose()
  chart = null
})

watch(
  () => [props.lower, props.upper, props.current, props.min, props.max, props.unit],
  () => renderChart()
)
</script>

<template>
  <div ref="chartRef" class="rdi-temperature-echarts-axis"></div>
</template>

<style scoped>
.rdi-temperature-echarts-axis {
  width: 100%;
  height: 118px;
}
</style>
