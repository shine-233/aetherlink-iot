/**
 * 文件用途：Timewindow 2.0 纯逻辑模型层。
 *
 * 核心逻辑：
 * 1. 相对时间窗口换算（Realtime ms 转换）；
 * 2. 自然日/周/月历史区间计算与对齐；
 * 3. 聚合采样分组粒度自动推导（对标 ThingsBoard 智能分组算法，自动将数据点适配至 50~300 个）；
 * 4. 看板全局时间窗口与小部件局部配置合并；
 * 5. 纯函数设计，无副作用，100% 具备测试可观测性。
 */

import type {
  AggregationConfig,
  AggregationFunc,
  QuickHistoryInterval,
  RealtimeIntervalLabel,
  ResolvedTimeRange,
  TimewindowConfig
} from './types'

export const REALTIME_INTERVAL_MAP: Record<RealtimeIntervalLabel, number> = {
  '1m': 60 * 1000,
  '5m': 5 * 60 * 1000,
  '15m': 15 * 60 * 1000,
  '30m': 30 * 60 * 1000,
  '1h': 60 * 60 * 1000,
  '2h': 2 * 60 * 60 * 1000,
  '6h': 6 * 60 * 60 * 1000,
  '12h': 12 * 60 * 60 * 1000,
  '1d': 24 * 60 * 60 * 1000,
  '7d': 7 * 24 * 60 * 60 * 1000,
  '30d': 30 * 24 * 60 * 60 * 1000
}

export const DEFAULT_TIMEWINDOW_CONFIG: TimewindowConfig = {
  type: 'realtime',
  realtime: { interval: '1h' },
  aggregation: { func: 'avg', interval: 0 },
  refreshInterval: 10000,
  timezone: 'browser'
}

/**
 * 将相对时间窗口标签或数值归一化为毫秒数
 */
export function normalizeRealtimeDuration(interval?: RealtimeIntervalLabel | number): number {
  if (typeof interval === 'number' && interval > 0) {
    return interval
  }
  if (typeof interval === 'string' && interval in REALTIME_INTERVAL_MAP) {
    return REALTIME_INTERVAL_MAP[interval as RealtimeIntervalLabel]
  }
  return REALTIME_INTERVAL_MAP['1h']
}

/**
 * 自动推导数据聚合分组粒度（毫秒）。
 * 算法依据时间跨度动态平滑，避免密集打点拖慢浏览器，同时保证曲线连续可读。
 */
export function calculateAutoGroupingInterval(rangeMs: number): number {
  const duration = Math.max(1000, rangeMs)
  if (duration <= 5 * 60 * 1000) return 1000 // <= 5分钟：1秒粒度
  if (duration <= 15 * 60 * 1000) return 5 * 1000 // <= 15分钟：5秒粒度
  if (duration <= 60 * 60 * 1000) return 15 * 1000 // <= 1小时：15秒粒度
  if (duration <= 2 * 60 * 60 * 1000) return 30 * 1000 // <= 2小时：30秒粒度
  if (duration <= 6 * 60 * 60 * 1000) return 60 * 1000 // <= 6小时：1分钟粒度
  if (duration <= 24 * 60 * 60 * 1000) return 5 * 60 * 1000 // <= 1天：5分钟粒度
  if (duration <= 7 * 24 * 60 * 60 * 1000) return 60 * 60 * 1000 // <= 7天：1小时粒度
  if (duration <= 30 * 24 * 60 * 60 * 1000) return 2 * 60 * 60 * 1000 // <= 30天：2小时粒度
  return 24 * 60 * 60 * 1000 // > 30天：1天粒度
}

/**
 * 计算快捷历史区间的精确起止时间戳
 */
export function resolveQuickHistoryRange(
  quick: QuickHistoryInterval,
  nowTs: number = Date.now()
): { startTime: number; endTime: number } {
  const now = new Date(nowTs)

  switch (quick) {
    case 'today': {
      const start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0, 0)
      return { startTime: start.getTime(), endTime: nowTs }
    }
    case 'yesterday': {
      const start = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 0, 0, 0, 0)
      const end = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 23, 59, 59, 999)
      return { startTime: start.getTime(), endTime: end.getTime() }
    }
    case 'this_week': {
      // 周一作为一周的起始
      const day = now.getDay()
      const diffToMonday = day === 0 ? 6 : day - 1
      const start = new Date(now.getFullYear(), now.getMonth(), now.getDate() - diffToMonday, 0, 0, 0, 0)
      return { startTime: start.getTime(), endTime: nowTs }
    }
    case 'prev_week': {
      const day = now.getDay()
      const diffToMonday = day === 0 ? 6 : day - 1
      const start = new Date(now.getFullYear(), now.getMonth(), now.getDate() - diffToMonday - 7, 0, 0, 0, 0)
      const end = new Date(now.getFullYear(), now.getMonth(), now.getDate() - diffToMonday - 1, 23, 59, 59, 999)
      return { startTime: start.getTime(), endTime: end.getTime() }
    }
    case 'this_month': {
      const start = new Date(now.getFullYear(), now.getMonth(), 1, 0, 0, 0, 0)
      return { startTime: start.getTime(), endTime: nowTs }
    }
    case 'prev_month': {
      const start = new Date(now.getFullYear(), now.getMonth() - 1, 1, 0, 0, 0, 0)
      const end = new Date(now.getFullYear(), now.getMonth(), 0, 23, 59, 59, 999)
      return { startTime: start.getTime(), endTime: end.getTime() }
    }
    case 'last_7d': {
      return { startTime: nowTs - 7 * 24 * 60 * 60 * 1000, endTime: nowTs }
    }
    case 'last_30d': {
      return { startTime: nowTs - 30 * 24 * 60 * 60 * 1000, endTime: nowTs }
    }
    default:
      return { startTime: nowTs - 3600 * 1000, endTime: nowTs }
  }
}

/**
 * 解析 Timewindow 配置为标准的 ResolvedTimeRange
 */
export function resolveTimewindow(
  config?: Partial<TimewindowConfig> | null,
  nowTs: number = Date.now()
): ResolvedTimeRange {
  const merged: TimewindowConfig = {
    ...DEFAULT_TIMEWINDOW_CONFIG,
    ...config,
    aggregation: {
      ...DEFAULT_TIMEWINDOW_CONFIG.aggregation!,
      ...config?.aggregation
    }
  }

  const isRealtime = merged.type === 'realtime'
  let startTime = 0
  let endTime = nowTs

  if (isRealtime) {
    const duration = normalizeRealtimeDuration(merged.realtime?.interval)
    startTime = nowTs - duration
    endTime = nowTs
  } else if (merged.history?.fixedRange && merged.history.fixedRange.startTime > 0) {
    startTime = merged.history.fixedRange.startTime
    endTime = Math.max(startTime + 1000, merged.history.fixedRange.endTime)
  } else if (merged.history?.quickInterval) {
    const range = resolveQuickHistoryRange(merged.history.quickInterval, nowTs)
    startTime = range.startTime
    endTime = range.endTime
  } else {
    // 历史兜底：过去 1 小时
    startTime = nowTs - 3600 * 1000
    endTime = nowTs
  }

  const rangeDuration = Math.max(1000, endTime - startTime)
  const aggregation: AggregationConfig = merged.aggregation || { func: 'avg', interval: 0 }
  const aggregationFunc: AggregationFunc = aggregation.func || 'avg'

  let groupingInterval = 0
  if (aggregationFunc !== 'none') {
    groupingInterval =
      aggregation.interval && aggregation.interval > 0
        ? aggregation.interval
        : calculateAutoGroupingInterval(rangeDuration)
  }

  return {
    startTime,
    endTime,
    groupingInterval,
    aggregationFunc,
    timezone: merged.timezone || 'browser',
    isRealtime
  }
}

/**
 * 合并全局与单小部件局部 Timewindow 配置
 */
export function mergeTimewindow(
  globalConfig?: TimewindowConfig | null,
  widgetConfig?: TimewindowConfig | null
): TimewindowConfig {
  if (widgetConfig) {
    return {
      ...(globalConfig || DEFAULT_TIMEWINDOW_CONFIG),
      ...widgetConfig,
      aggregation: {
        ...(globalConfig?.aggregation || DEFAULT_TIMEWINDOW_CONFIG.aggregation!),
        ...widgetConfig.aggregation
      }
    }
  }
  return globalConfig || DEFAULT_TIMEWINDOW_CONFIG
}
