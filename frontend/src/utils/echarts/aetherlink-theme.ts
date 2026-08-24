/**
 * 文件用途：注册 AetherLink IoT 品牌 ECharts 主题，全局生效。
 * 核心逻辑：定义品牌色板、渐变面积图、圆角柱状图、柔和网格线与坐标轴样式。
 * 关键注意事项：主题在 echarts/core 初始化后通过 registerTheme 注册一次即可。
 */
import type { EChartsOption } from 'echarts'

const BRAND = {
  primary: '#2080f0',
  primaryLight: '#4098fc',
  primaryDark: '#1063c4',
  success: '#18a058',
  warning: '#f0a020',
  error: '#d03050',
  info: '#2080f0',
  textPrimary: '#333639',
  textSecondary: '#666e75',
  borderColor: '#e0e6ed',
  bgColor: 'transparent',
  gradientArea: ['rgba(32,128,240,0.25)', 'rgba(32,128,240,0.02)'],
  gradientBar: ['#4098fc', '#2080f0'],
}

export const aetherLinkTheme: EChartsOption = {
  color: [BRAND.primary, BRAND.success, BRAND.warning, BRAND.error, '#858eb4', '#14c9c9', '#f5b5ff', '#00b2ff'],
  backgroundColor: BRAND.bgColor,
  textStyle: { color: BRAND.textSecondary },
  title: { textStyle: { color: BRAND.textPrimary, fontSize: 15, fontWeight: 600 } },
  legend: {
    textStyle: { color: BRAND.textSecondary, fontSize: 12 },
    itemWidth: 14,
    itemHeight: 8,
    icon: 'roundRect',
  },
  tooltip: {
    backgroundColor: 'rgba(255,255,255,0.96)',
    borderColor: BRAND.borderColor,
    textStyle: { color: BRAND.textPrimary, fontSize: 13 },
    extraCssText: 'box-shadow: 0 4px 16px rgba(0,0,0,0.08); border-radius: 8px;',
  },
  grid: {
    left: '3%',
    right: '4%',
    bottom: '6%',
    containLabel: true,
  },
  xAxis: {
    axisLine: { lineStyle: { color: BRAND.borderColor } },
    axisTick: { show: false },
    axisLabel: { color: BRAND.textSecondary, fontSize: 11 },
    splitLine: { show: false },
  },
  yAxis: {
    axisLine: { show: false },
    axisTick: { show: false },
    axisLabel: { color: BRAND.textSecondary, fontSize: 11 },
    splitLine: { lineStyle: { color: BRAND.borderColor, type: 'dashed', opacity: 0.6 } },
  },
} as any

/** 面积图 series 默认样式（渐变+平滑） */
export function areaSeriesStyle(color = BRAND.primary) {
  return {
    smooth: true,
    lineStyle: { width: 2.5 },
    areaStyle: {
      color: {
        type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
        colorStops: [
          { offset: 0, color: color + '40' },
          { offset: 1, color: color + '05' },
        ],
      },
    },
  }
}

/** 柱状图 series 默认样式（圆角+渐变） */
export function barSeriesStyle() {
  return {
    barMaxWidth: 28,
    itemStyle: {
      borderRadius: [4, 4, 0, 0],
      color: {
        type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
        colorStops: [
          { offset: 0, color: BRAND.primaryLight },
          { offset: 1, color: BRAND.primary },
        ],
      },
    },
  }
}