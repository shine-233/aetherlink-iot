import { describe, expect, it } from 'vitest'

import type { TelemetryAnomalyResult } from '@/service/api'

import {
  ANOMALY_DEFAULT_K,
  ANOMALY_MAX_DEVICES,
  DEFAULT_ANOMALY_FORM,
  buildAnomalyQuery,
  parseDeviceIds,
  summarizeAnomalyRows,
  toAnomalyRows,
  type AnomalyFormState
} from '../anomaly-model'

const RANGE: [number, number] = [1_700_000_000_000, 1_700_003_600_000]

function form(patch: Partial<AnomalyFormState> = {}): AnomalyFormState {
  return { ...DEFAULT_ANOMALY_FORM, deviceIdsText: 'dev-1', key: 'temperature', ...patch }
}

function errorKeyOf(result: ReturnType<typeof buildAnomalyQuery>): string {
  return result.ok ? `(unexpected ok)` : result.errorKey
}

describe('parseDeviceIds', () => {
  it('supports newline and comma separators with trimming', () => {
    expect(parseDeviceIds(' dev-1 \n dev-2 , dev-3 ')).toEqual(['dev-1', 'dev-2', 'dev-3'])
  })

  it('drops empty entries and dedupes while keeping order', () => {
    // 重复 ID 会让同一次检测把同一设备算两遍，结果里出现两条同名记录。
    expect(parseDeviceIds('dev-2\n\ndev-1\ndev-2\n   ')).toEqual(['dev-2', 'dev-1'])
  })

  it('returns empty array for blank input', () => {
    expect(parseDeviceIds('   \n , \n')).toEqual([])
  })
})

describe('buildAnomalyQuery validation', () => {
  it('rejects empty device list', () => {
    expect(errorKeyOf(buildAnomalyQuery({ form: form({ deviceIdsText: '  ' }), range: RANGE }))).toBe(
      'page.anomaly.errorDevicesRequired'
    )
  })

  it('rejects more than the backend device limit', () => {
    const many = Array.from({ length: ANOMALY_MAX_DEVICES + 1 }, (_, i) => `dev-${i}`).join('\n')
    expect(errorKeyOf(buildAnomalyQuery({ form: form({ deviceIdsText: many }), range: RANGE }))).toBe(
      'page.anomaly.errorDevicesTooMany'
    )
  })

  it('rejects blank key', () => {
    expect(errorKeyOf(buildAnomalyQuery({ form: form({ key: '   ' }), range: RANGE }))).toBe(
      'page.anomaly.errorKeyRequired'
    )
  })

  it('rejects missing time range', () => {
    expect(errorKeyOf(buildAnomalyQuery({ form: form(), range: null }))).toBe('page.anomaly.errorRangeRequired')
  })

  it('rejects a non-increasing time range', () => {
    expect(errorKeyOf(buildAnomalyQuery({ form: form(), range: [RANGE[1], RANGE[0]] }))).toBe(
      'page.anomaly.errorRangeOrder'
    )
    expect(errorKeyOf(buildAnomalyQuery({ form: form(), range: [RANGE[0], RANGE[0]] }))).toBe(
      'page.anomaly.errorRangeOrder'
    )
  })

  it('rejects a window below one minute', () => {
    expect(errorKeyOf(buildAnomalyQuery({ form: form({ windowMinutes: 0 }), range: RANGE }))).toBe(
      'page.anomaly.errorWindow'
    )
  })

  it('rejects bounds without min or max', () => {
    expect(errorKeyOf(buildAnomalyQuery({ form: form({ boundsMin: null, boundsMax: null }), range: RANGE }))).toBe(
      'page.anomaly.errorBoundsRequired'
    )
  })

  it('rejects bounds where min exceeds max', () => {
    expect(errorKeyOf(buildAnomalyQuery({ form: form({ boundsMin: 10, boundsMax: 1 }), range: RANGE }))).toBe(
      'page.anomaly.errorBoundsOrder'
    )
  })

  it('rejects non-positive deviation k', () => {
    const ruleType = 'deviation' as const
    expect(errorKeyOf(buildAnomalyQuery({ form: form({ ruleType, deviationK: 0 }), range: RANGE }))).toBe(
      'page.anomaly.errorDeviationK'
    )
    expect(errorKeyOf(buildAnomalyQuery({ form: form({ ruleType, deviationK: -1 }), range: RANGE }))).toBe(
      'page.anomaly.errorDeviationK'
    )
  })
})

describe('buildAnomalyQuery success', () => {
  it('builds a bounds query and converts the window to milliseconds', () => {
    const result = buildAnomalyQuery({ form: form({ windowMinutes: 5, boundsMin: 0, boundsMax: 100 }), range: RANGE })
    expect(result.ok).toBe(true)
    if (!result.ok) return
    expect(result.query).toEqual({
      device_ids: ['dev-1'],
      key: 'temperature',
      start_time: RANGE[0],
      end_time: RANGE[1],
      window_ms: 300_000,
      aggregate: 'avg',
      rule: { type: 'bounds', min: 0, max: 100 }
    })
  })

  it('omits the unused bound instead of sending a placeholder', () => {
    const result = buildAnomalyQuery({ form: form({ boundsMin: null, boundsMax: 100 }), range: RANGE })
    expect(result.ok).toBe(true)
    if (!result.ok) return
    // 只设上限时不得把 min 塞成 0——0 是合法下限，会把语义改成"双边界"。
    expect(result.query.rule).toEqual({ type: 'bounds', min: undefined, max: 100 })
  })

  it('falls back to the default k when deviation k is left blank', () => {
    const result = buildAnomalyQuery({
      form: form({ ruleType: 'deviation' as const, deviationK: null }),
      range: RANGE
    })
    expect(result.ok).toBe(true)
    if (!result.ok) return
    expect(result.query.rule).toEqual({ type: 'deviation', k: ANOMALY_DEFAULT_K })
  })
})

describe('toAnomalyRows', () => {
  const result: TelemetryAnomalyResult = {
    key: 'temperature',
    aggregate: 'avg',
    window_ms: 300_000,
    rule: { type: 'deviation', k: 3 },
    devices: [
      { device_id: 'failed', error: 'permission denied', total: 0, anomalies: [], rate: 0 },
      { device_id: 'empty', total: 0, anomalies: [], rate: 0 },
      { device_id: 'noisy', total: 10, anomalies: [{ index: 3, value: 99, reason: 'above max' }], rate: 0.1 },
      { device_id: 'calm', total: 10, anomalies: [], rate: 0 }
    ]
  }

  it('orders statuses by error > no-data > anomaly > clean', () => {
    expect(toAnomalyRows(result).map((row) => row.status)).toEqual(['error', 'no-data', 'anomaly', 'clean'])
  })

  it('keeps "no data" distinct from "no anomaly"', () => {
    const rows = toAnomalyRows(result)
    expect(rows[1].status).toBe('no-data')
    expect(rows[3].status).toBe('clean')
    // 两者都必须有 total 才能区分：total=0 表示窗口内没有数据。
    expect(rows[1].total).toBe(0)
    expect(rows[3].total).toBe(10)
  })

  it('does not let a per-device error masquerade as clean', () => {
    const rows = toAnomalyRows(result)
    expect(rows[0].status).toBe('error')
    expect(rows[0].error).toBe('permission denied')
  })

  it('tolerates a null or malformed result', () => {
    expect(toAnomalyRows(null)).toEqual([])
    expect(toAnomalyRows({} as TelemetryAnomalyResult)).toEqual([])
  })
})

describe('summarizeAnomalyRows', () => {
  it('counts each status separately and sums hits', () => {
    const rows = toAnomalyRows({
      key: 'k',
      aggregate: 'avg',
      window_ms: 1000,
      rule: { type: 'bounds' },
      devices: [
        { device_id: 'a', total: 1, anomalies: [{ index: 0, value: 1, reason: 'r' }], rate: 1 },
        { device_id: 'b', total: 1, anomalies: [{ index: 0, value: 1, reason: 'r' }], rate: 1 },
        { device_id: 'c', total: 1, anomalies: [], rate: 0 },
        { device_id: 'd', total: 0, anomalies: [], rate: 0 },
        { device_id: 'e', error: 'boom', total: 0, anomalies: [], rate: 0 }
      ]
    })
    expect(summarizeAnomalyRows(rows)).toEqual({
      devices: 5,
      anomaly: 2,
      clean: 1,
      noData: 1,
      failed: 1,
      totalHits: 2
    })
  })
})
