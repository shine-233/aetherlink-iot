import { describe, expect, it } from 'vitest'
import dayjs from 'dayjs'

import {
  applyChartSeriesType,
  applyAggregationWeight,
  applyAggregationWindowChange,
  buildTelemetryCsv,
  calculateTimeWeight,
  escapeCsvCell,
  projectTelemetryHistory,
  resolveNavigatedCustomRange,
  type AggregationIntervalOption,
  type SelectedOptionState
} from '../time-series-data-state'

function option(value: AggregationIntervalOption['value']): AggregationIntervalOption {
  return { label: value, value, disabled: false }
}

function selectedOption(): SelectedOptionState {
  return {
    device_id: 'device-1',
    key: 'temp',
    aggregate_window: 'no_aggregate',
    time_range: 'last_1h',
    start_time: undefined,
    end_time: undefined,
    aggregate_function: undefined
  }
}

describe('time-series-data-state', () => {
  it('disables windows below the weight threshold and picks the first allowed window', () => {
    const options = [option('no_aggregate'), option('30s'), option('1m'), option('2m'), option('5m')]

    const result = applyAggregationWeight(options, selectedOption(), 3)

    expect(result.aggregationIntervalOptions.map((item) => item.disabled)).toEqual([true, true, true, false, false])
    expect(result.selectedOption.aggregate_window).toBe('2m')
    expect(result.selectedOption.aggregate_function).toBe('avg')
  })

  it('keeps no_aggregate aligned with an undefined aggregate function', () => {
    const options = [option('no_aggregate'), option('30s'), option('1m')]

    const result = applyAggregationWeight(options, selectedOption(), 0)

    expect(result.selectedOption.aggregate_window).toBe('no_aggregate')
    expect(result.selectedOption.aggregate_function).toBeUndefined()
  })

  it('moves the custom range to the expected calendar boundary', () => {
    const baseTime = dayjs('2026-06-15T10:30:00Z').valueOf()

    const moved = resolveNavigatedCustomRange(baseTime, 'prevDay')

    expect(moved.selectedOption.start_time).toBe(dayjs(baseTime).subtract(1, 'day').startOf('day').valueOf())
    expect(moved.selectedOption.end_time).toBe(dayjs(baseTime).subtract(1, 'day').endOf('day').valueOf())
    expect(moved.selectedOption.time_range).toBe('custom')
    expect(moved.datePickerValue).toEqual([
      dayjs(baseTime).subtract(1, 'day').startOf('day').valueOf(),
      dayjs(baseTime).subtract(1, 'day').endOf('day').valueOf()
    ])
  })

  it('maps custom ranges to the same weight thresholds used by the component', () => {
    const start = dayjs('2026-01-01T00:00:00Z').valueOf()
    const end = dayjs('2026-08-01T00:00:00Z').valueOf()

    expect(calculateTimeWeight(start, end)).toBe(12)
  })

  it('auto-fills avg when enabling aggregation and clears it when switching back', () => {
    const next = applyAggregationWindowChange(selectedOption(), '1m')
    expect(next.aggregate_window).toBe('1m')
    expect(next.aggregate_function).toBe('avg')

    const reset = applyAggregationWindowChange(next, 'no_aggregate')
    expect(reset.aggregate_window).toBe('no_aggregate')
    expect(reset.aggregate_function).toBeUndefined()
  })

  it('projects history points into sorted table rows, summary values, and nullable chart pairs', () => {
    const projection = projectTelemetryHistory([
      { x: 1000, y: 20 },
      { x: 3000, y: 40 },
      { x: 2000, y: 'bad' as unknown as number }
    ])

    expect(projection.tableData).toEqual([
      { x: 3000, y: 40 },
      { x: 2000, y: 'bad' },
      { x: 1000, y: 20 }
    ])
    expect(projection.summary).toEqual({
      avgValue: 30,
      maxValue: 40,
      minValue: 20,
      latestTimestamp: 3000,
      totalSampleCount: 3,
      validSampleCount: 2
    })
    expect(projection.seriesData).toEqual([
      [1000, 20],
      [3000, 40],
      [2000, null]
    ])
  })

  it('clears summary values and keeps an empty projection when history is empty', () => {
    const projection = projectTelemetryHistory([])

    expect(projection.tableData).toEqual([])
    expect(projection.seriesData).toEqual([])
    expect(projection.summary).toEqual({
      avgValue: null,
      maxValue: null,
      minValue: null,
      latestTimestamp: null,
      totalSampleCount: 0,
      validSampleCount: 0
    })
  })

  it('keeps freshness metadata even when the latest sample is not numeric', () => {
    const projection = projectTelemetryHistory([
      { x: 1000, y: 18 },
      { x: 5000, y: undefined },
      { x: 3000, y: 22 }
    ])

    expect(projection.summary.latestTimestamp).toBe(5000)
    expect(projection.summary.totalSampleCount).toBe(3)
    expect(projection.summary.validSampleCount).toBe(2)
  })

  it('switches the first chart series type and only warns on duplicate clicks', () => {
    const initial = [{ type: 'line', data: [[1, 2]] }]

    const toBar = applyChartSeriesType(initial, 'bar', 'common.alreadyToChart')
    expect(toBar.series?.[0].type).toBe('bar')
    expect(toBar.duplicateMessageKey).toBeUndefined()

    const duplicate = applyChartSeriesType(toBar.series, 'bar', 'common.alreadyToChart')
    expect(duplicate.series?.[0].type).toBe('bar')
    expect(duplicate.duplicateMessageKey).toBe('common.alreadyToChart')
  })

  it('treats missing chart series as a no-op', () => {
    const next = applyChartSeriesType(undefined, 'scatter', 'common.alreadyScatterPlot')

    expect(next.series).toBeUndefined()
    expect(next.duplicateMessageKey).toBeUndefined()
  })

  it('escapes telemetry CSV cells with commas, quotes, and line breaks', () => {
    expect(escapeCsvCell('plain')).toBe('plain')
    expect(escapeCsvCell('a,b')).toBe('"a,b"')
    expect(escapeCsvCell('say "hi"')).toBe('"say ""hi"""')
    expect(escapeCsvCell('line\nbreak')).toBe('"line\nbreak"')
    expect(escapeCsvCell('=cmd|A1')).toBe("'=cmd|A1")
    expect(escapeCsvCell(-1)).toBe('-1')
    expect(escapeCsvCell(null)).toBe('')
  })

  it('builds telemetry CSV with escaped rows', () => {
    const csv = buildTelemetryCsv([
      ['time', 'value,with,commas'],
      ['2026-07-05 10:00:00', '12"3']
    ])

    expect(csv).toBe('time,"value,with,commas"\n2026-07-05 10:00:00,"12""3"')
  })
})
