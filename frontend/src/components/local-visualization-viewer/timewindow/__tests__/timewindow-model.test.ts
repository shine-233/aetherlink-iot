import { describe, expect, it } from 'vitest'
import {
  calculateAutoGroupingInterval,
  DEFAULT_TIMEWINDOW_CONFIG,
  mergeTimewindow,
  normalizeRealtimeDuration,
  REALTIME_INTERVAL_MAP,
  resolveQuickHistoryRange,
  resolveTimewindow
} from '../timewindow-model'
import type { TimewindowConfig } from '../types'

describe('Timewindow 2.0 Model', () => {
  describe('normalizeRealtimeDuration', () => {
    it('normalizes string interval labels correctly', () => {
      expect(normalizeRealtimeDuration('1m')).toBe(60 * 1000)
      expect(normalizeRealtimeDuration('15m')).toBe(15 * 60 * 1000)
      expect(normalizeRealtimeDuration('1h')).toBe(3600 * 1000)
      expect(normalizeRealtimeDuration('1d')).toBe(86400 * 1000)
      expect(normalizeRealtimeDuration('30d')).toBe(30 * 86400 * 1000)
    })

    it('accepts explicit positive millisecond values', () => {
      expect(normalizeRealtimeDuration(45000)).toBe(45000)
    })

    it('falls back to 1h for undefined or invalid input', () => {
      expect(normalizeRealtimeDuration(undefined)).toBe(REALTIME_INTERVAL_MAP['1h'])
      expect(normalizeRealtimeDuration(0)).toBe(REALTIME_INTERVAL_MAP['1h'])
      expect(normalizeRealtimeDuration(-500)).toBe(REALTIME_INTERVAL_MAP['1h'])
      // @ts-expect-error test fallback
      expect(normalizeRealtimeDuration('invalid_key')).toBe(REALTIME_INTERVAL_MAP['1h'])
    })
  })

  describe('calculateAutoGroupingInterval', () => {
    it('calculates optimal grouping intervals based on duration span', () => {
      // <= 5 min -> 1s
      expect(calculateAutoGroupingInterval(60 * 1000)).toBe(1000)
      expect(calculateAutoGroupingInterval(5 * 60 * 1000)).toBe(1000)

      // <= 15 min -> 5s
      expect(calculateAutoGroupingInterval(10 * 60 * 1000)).toBe(5000)

      // <= 1 hour -> 15s
      expect(calculateAutoGroupingInterval(30 * 60 * 1000)).toBe(15000)
      expect(calculateAutoGroupingInterval(60 * 60 * 1000)).toBe(15000)

      // <= 2 hours -> 30s
      expect(calculateAutoGroupingInterval(90 * 60 * 1000)).toBe(30000)

      // <= 6 hours -> 1 min
      expect(calculateAutoGroupingInterval(4 * 3600 * 1000)).toBe(60000)

      // <= 24 hours -> 5 min
      expect(calculateAutoGroupingInterval(12 * 3600 * 1000)).toBe(300000)

      // <= 7 days -> 1 hour
      expect(calculateAutoGroupingInterval(3 * 86400 * 1000)).toBe(3600000)

      // <= 30 days -> 2 hours
      expect(calculateAutoGroupingInterval(15 * 86400 * 1000)).toBe(7200000)

      // > 30 days -> 1 day
      expect(calculateAutoGroupingInterval(60 * 86400 * 1000)).toBe(86400000)
    })
  })

  describe('resolveQuickHistoryRange', () => {
    const fixedNow = new Date(2026, 8, 16, 14, 30, 0, 0).getTime() // 2026-09-16 14:30:00 (Wednesday)

    it('resolves "today" range from 00:00:00 to now', () => {
      const { startTime, endTime } = resolveQuickHistoryRange('today', fixedNow)
      const expectedStart = new Date(2026, 8, 16, 0, 0, 0, 0).getTime()
      expect(startTime).toBe(expectedStart)
      expect(endTime).toBe(fixedNow)
    })

    it('resolves "yesterday" range covering the full previous day', () => {
      const { startTime, endTime } = resolveQuickHistoryRange('yesterday', fixedNow)
      const expectedStart = new Date(2026, 8, 15, 0, 0, 0, 0).getTime()
      const expectedEnd = new Date(2026, 8, 15, 23, 59, 59, 999).getTime()
      expect(startTime).toBe(expectedStart)
      expect(endTime).toBe(expectedEnd)
    })

    it('resolves "this_week" starting from Monday 00:00:00', () => {
      const { startTime, endTime } = resolveQuickHistoryRange('this_week', fixedNow)
      // 2026-09-16 is Wednesday, Monday was 2026-09-14
      const expectedStart = new Date(2026, 8, 14, 0, 0, 0, 0).getTime()
      expect(startTime).toBe(expectedStart)
      expect(endTime).toBe(fixedNow)
    })

    it('resolves "this_month" starting from day 1 00:00:00', () => {
      const { startTime, endTime } = resolveQuickHistoryRange('this_month', fixedNow)
      const expectedStart = new Date(2026, 8, 1, 0, 0, 0, 0).getTime()
      expect(startTime).toBe(expectedStart)
      expect(endTime).toBe(fixedNow)
    })

    it('resolves "last_7d" rolling range', () => {
      const { startTime, endTime } = resolveQuickHistoryRange('last_7d', fixedNow)
      expect(endTime).toBe(fixedNow)
      expect(startTime).toBe(fixedNow - 7 * 86400 * 1000)
    })
  })

  describe('resolveTimewindow', () => {
    const fixedNow = 1726480000000

    it('resolves default realtime config (1h, avg, auto grouping)', () => {
      const resolved = resolveTimewindow(undefined, fixedNow)
      expect(resolved.isRealtime).toBe(true)
      expect(resolved.endTime).toBe(fixedNow)
      expect(resolved.startTime).toBe(fixedNow - 3600 * 1000)
      expect(resolved.aggregationFunc).toBe('avg')
      expect(resolved.groupingInterval).toBe(15000) // 1h auto interval is 15s
      expect(resolved.timezone).toBe('browser')
    })

    it('resolves realtime config with custom interval and explicit grouping', () => {
      const config: TimewindowConfig = {
        type: 'realtime',
        realtime: { interval: '6h' },
        aggregation: { func: 'max', interval: 60000 }
      }
      const resolved = resolveTimewindow(config, fixedNow)
      expect(resolved.isRealtime).toBe(true)
      expect(resolved.startTime).toBe(fixedNow - 6 * 3600 * 1000)
      expect(resolved.endTime).toBe(fixedNow)
      expect(resolved.aggregationFunc).toBe('max')
      expect(resolved.groupingInterval).toBe(60000)
    })

    it('resolves history fixedRange accurately', () => {
      const start = fixedNow - 100000
      const end = fixedNow - 10000
      const config: TimewindowConfig = {
        type: 'history',
        history: { fixedRange: { startTime: start, endTime: end } },
        aggregation: { func: 'sum' }
      }
      const resolved = resolveTimewindow(config, fixedNow)
      expect(resolved.isRealtime).toBe(false)
      expect(resolved.startTime).toBe(start)
      expect(resolved.endTime).toBe(end)
      expect(resolved.aggregationFunc).toBe('sum')
      expect(resolved.groupingInterval).toBeGreaterThan(0)
    })

    it('handles aggregationFunc = "none" with zero grouping interval', () => {
      const config: TimewindowConfig = {
        type: 'realtime',
        realtime: { interval: '1m' },
        aggregation: { func: 'none' }
      }
      const resolved = resolveTimewindow(config, fixedNow)
      expect(resolved.aggregationFunc).toBe('none')
      expect(resolved.groupingInterval).toBe(0)
    })
  })

  describe('mergeTimewindow', () => {
    it('returns default config when both are empty', () => {
      const merged = mergeTimewindow(null, null)
      expect(merged.type).toBe('realtime')
      expect(merged.realtime?.interval).toBe('1h')
    })

    it('allows widget config to override global settings', () => {
      const globalConfig: TimewindowConfig = {
        type: 'realtime',
        realtime: { interval: '1h' },
        aggregation: { func: 'avg' },
        timezone: 'UTC'
      }
      const widgetConfig: TimewindowConfig = {
        type: 'realtime',
        realtime: { interval: '15m' },
        aggregation: { func: 'max' }
      }
      const merged = mergeTimewindow(globalConfig, widgetConfig)
      expect(merged.realtime?.interval).toBe('15m')
      expect(merged.aggregation?.func).toBe('max')
      expect(merged.timezone).toBe('UTC') // inherited from global
    })
  })
})
