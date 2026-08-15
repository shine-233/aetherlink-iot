import dayjs from 'dayjs'

export type TimeRange =
  | 'custom'
  | 'last_5m'
  | 'last_15m'
  | 'last_30m'
  | 'last_1h'
  | 'last_3h'
  | 'last_6h'
  | 'last_12h'
  | 'last_24h'
  | 'last_3d'
  | 'last_7d'
  | 'last_15d'
  | 'last_30d'
  | 'last_60d'
  | 'last_90d'
  | 'last_6m'
  | 'last_1y'

export type AggregateWindow =
  | 'no_aggregate'
  | '30s'
  | '1m'
  | '2m'
  | '5m'
  | '10m'
  | '30m'
  | '1h'
  | '3h'
  | '6h'
  | '1d'
  | '7d'
  | '1mo'

export type AggregateFunction = 'avg' | 'max' | 'min' | 'sum' | 'diff'
export type ChartSeriesType = 'line' | 'bar' | 'scatter'

export interface SelectedOptionState {
  device_id: string
  key: string
  aggregate_window: AggregateWindow
  time_range: TimeRange
  start_time: number | undefined
  end_time: number | undefined
  aggregate_function: AggregateFunction | undefined
}

export interface AggregationIntervalOption {
  label: string
  value: AggregateWindow
  disabled: boolean
}

export interface TelemetryHistoryPoint {
  x: number
  y?: number | null
}

export interface TelemetryHistorySummary {
  avgValue: number | null
  maxValue: number | null
  minValue: number | null
  latestTimestamp: number | null
  totalSampleCount: number
  validSampleCount: number
}

export interface TelemetryHistoryProjection {
  tableData: TelemetryHistoryPoint[]
  seriesData: Array<[number | null, number | null]>
  summary: TelemetryHistorySummary
}

export type TelemetryCsvCell = string | number | null | undefined

export type TimeNavigationDirection = 'prevMonth' | 'prevDay' | 'prevHour' | 'nextHour' | 'nextDay' | 'nextMonth'

export function applyAggregationWeight(
  options: AggregationIntervalOption[],
  selectedOption: SelectedOptionState,
  weight: number
) {
  const nextOptions = options.map((item, index) => ({
    ...item,
    disabled: index < weight
  }))

  const fallbackIndex = Math.min(Math.max(weight, 0), Math.max(nextOptions.length - 1, 0))
  const nextWindow = nextOptions[fallbackIndex]?.value ?? selectedOption.aggregate_window
  const nextFunction = nextWindow === 'no_aggregate' ? undefined : (selectedOption.aggregate_function ?? 'avg')

  return {
    aggregationIntervalOptions: nextOptions,
    selectedOption: {
      ...selectedOption,
      aggregate_window: nextWindow,
      aggregate_function: nextFunction
    }
  }
}

export function resolveNavigatedCustomRange(
  startTime: number | undefined,
  direction: TimeNavigationDirection | string,
  now = dayjs().valueOf()
) {
  const baseStart = startTime ?? dayjs(now).startOf('hour').valueOf()
  let startDate = dayjs(baseStart)
  let endTime: number

  switch (direction) {
    case 'prevMonth':
      startDate = startDate.subtract(1, 'month').startOf('month')
      endTime = startDate.endOf('month').valueOf()
      break
    case 'prevDay':
      startDate = startDate.subtract(1, 'day').startOf('day')
      endTime = startDate.endOf('day').valueOf()
      break
    case 'prevHour':
      startDate = startDate.subtract(1, 'hour').startOf('hour')
      endTime = startDate.endOf('hour').valueOf()
      break
    case 'nextHour':
      startDate = startDate.add(1, 'hour').startOf('hour')
      endTime = startDate.endOf('hour').valueOf()
      break
    case 'nextDay':
      startDate = startDate.add(1, 'day').startOf('day')
      endTime = startDate.endOf('day').valueOf()
      break
    case 'nextMonth':
      startDate = startDate.add(1, 'month').startOf('month')
      endTime = startDate.endOf('month').valueOf()
      break
    default:
      endTime = startDate.endOf('hour').valueOf()
      break
  }

  const resolvedStart = startDate.valueOf()
  return {
    selectedOption: {
      start_time: resolvedStart,
      end_time: endTime,
      time_range: 'custom' as const
    },
    datePickerValue: [resolvedStart, endTime] as [number, number]
  }
}

export function calculateTimeWeight(start: number, end: number) {
  const startDate = dayjs(start)
  const endDate = dayjs(end)
  const diffInHours = endDate.diff(startDate, 'hour')
  if (diffInHours <= 1) return 0
  if (diffInHours <= 3) return 1
  if (diffInHours <= 6) return 2
  if (diffInHours <= 12) return 3
  if (diffInHours <= 24) return 4
  if (diffInHours <= 3 * 24) return 5
  if (diffInHours <= 7 * 24) return 6
  if (diffInHours <= 15 * 24) return 7
  if (diffInHours <= 30 * 24) return 8
  if (diffInHours <= 60 * 24) return 9
  if (diffInHours <= 90 * 24) return 10
  if (diffInHours <= 6 * 30 * 24) return 11
  if (diffInHours <= 365 * 24) return 12
  return 13
}

export function applyAggregationWindowChange(selectedOption: SelectedOptionState, value: AggregateWindow) {
  return {
    ...selectedOption,
    aggregate_window: value,
    aggregate_function: value === 'no_aggregate' ? undefined : (selectedOption.aggregate_function ?? 'avg')
  }
}

export function projectTelemetryHistory(points: TelemetryHistoryPoint[]): TelemetryHistoryProjection {
  const tableData = [...points].sort((a, b) => b.x - a.x)
  let sumValue = 0
  let min = Number.POSITIVE_INFINITY
  let max = Number.NEGATIVE_INFINITY
  let count = 0
  let latestTimestamp: number | null = null

  points.forEach((item) => {
    const timestamp = Number(item.x)
    if (Number.isFinite(timestamp) && (latestTimestamp === null || timestamp > latestTimestamp)) {
      latestTimestamp = timestamp
    }

    const numericValue = Number(item.y)
    if (Number.isFinite(numericValue)) {
      sumValue += numericValue
      count += 1
      if (numericValue < min) {
        min = numericValue
      }
      if (numericValue > max) {
        max = numericValue
      }
    }
  })

  return {
    tableData,
    seriesData: points.map((item) => {
      const xValue = Number(item.x)
      const yValue = Number(item.y)
      return [Number.isFinite(xValue) ? xValue : null, Number.isFinite(yValue) ? yValue : null]
    }),
    summary: {
      minValue: count > 0 ? min : null,
      maxValue: count > 0 ? max : null,
      avgValue: count > 0 ? sumValue / count : null,
      latestTimestamp,
      totalSampleCount: points.length,
      validSampleCount: count
    }
  }
}

export function applyChartSeriesType<T extends { type: string }>(
  series: T[] | undefined,
  nextType: ChartSeriesType,
  duplicateMessageKey: string
) {
  if (!series || !series[0]) {
    return {
      series,
      duplicateMessageKey: undefined
    }
  }

  if (series[0].type === nextType) {
    return {
      series,
      duplicateMessageKey
    }
  }

  return {
    series: series.map((item, index) => (index === 0 ? { ...item, type: nextType } : item)),
    duplicateMessageKey: undefined
  }
}

export function escapeCsvCell(value: TelemetryCsvCell) {
  const rawText = value === null || value === undefined ? '' : String(value)
  const text = typeof value === 'string' && /^[=\-+@]/.test(rawText.trimStart()) ? `'${rawText}` : rawText
  if (!/[",\r\n]/.test(text)) return text
  return `"${text.replace(/"/g, '""')}"`
}

export function buildTelemetryCsv(rows: TelemetryCsvCell[][]) {
  return rows.map((row) => row.map(escapeCsvCell).join(',')).join('\n')
}
