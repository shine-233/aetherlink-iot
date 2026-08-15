export interface FilterParams {
  aggregate_function?: 'avg' | 'max' | 'min' | 'sum' | 'diff'
  aggregate_window?:
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
  end_time?: number
  start_time?: number
  time_range?:
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
}

export interface TimeSeriesItem {
  x: number
  x2?: number
  y: number
}

export type AggregateWindowValue = NonNullable<FilterParams['aggregate_window']>
export type TimeRangeKey = NonNullable<FilterParams['time_range']>

export type AggregateWindowOptionBase = {
  label: string
  value: AggregateWindowValue
  seconds: number
}

export type AggregateWindowOption = AggregateWindowOptionBase & {
  disabled: boolean
}

export const windowToSeconds = (window: string): number => {
  if (window === 'no_aggregate') return -1
  const match = window.match(/^(\d+)([smhdmo]{1,2})$/)
  if (!match) return Infinity
  const value = parseInt(match[1], 10)
  const unit = match[2]
  switch (unit) {
    case 's':
      return value
    case 'm':
      return value * 60
    case 'h':
      return value * 60 * 60
    case 'd':
      return value * 24 * 60 * 60
    case 'mo':
      return value * 30 * 24 * 60 * 60
    default:
      return Infinity
  }
}

export const createAggregateWindowOptions = (t: (key: string) => string): AggregateWindowOptionBase[] =>
  ([
    { label: t('common.notAggre'), value: 'no_aggregate', seconds: windowToSeconds('no_aggregate') },
    { label: t('common.seconds30'), value: '30s', seconds: windowToSeconds('30s') },
    { label: t('common.minute1'), value: '1m', seconds: windowToSeconds('1m') },
    { label: t('common.minute2'), value: '2m', seconds: windowToSeconds('2m') },
    { label: t('common.minutes5'), value: '5m', seconds: windowToSeconds('5m') },
    { label: t('common.minutes10'), value: '10m', seconds: windowToSeconds('10m') },
    { label: t('common.minutes30'), value: '30m', seconds: windowToSeconds('30m') },
    { label: t('common.hours1'), value: '1h', seconds: windowToSeconds('1h') },
    { label: t('common.hours3'), value: '3h', seconds: windowToSeconds('3h') },
    { label: t('common.hours6'), value: '6h', seconds: windowToSeconds('6h') },
    { label: t('common.days1'), value: '1d', seconds: windowToSeconds('1d') },
    { label: t('common.days7'), value: '7d', seconds: windowToSeconds('7d') },
    { label: t('common.months1'), value: '1mo', seconds: windowToSeconds('1mo') }
  ] as AggregateWindowOptionBase[]).sort((a, b) => a.seconds - b.seconds)

export const timeRangeMinWindowSeconds: Record<TimeRangeKey, number> = {
  last_5m: -1,
  last_15m: -1,
  last_30m: -1,
  last_1h: -1,
  last_3h: windowToSeconds('30s'),
  last_6h: windowToSeconds('1m'),
  last_12h: windowToSeconds('2m'),
  last_24h: windowToSeconds('5m'),
  last_3d: windowToSeconds('10m'),
  last_7d: windowToSeconds('30m'),
  last_15d: windowToSeconds('1h'),
  last_30d: windowToSeconds('3h'),
  last_60d: windowToSeconds('6h'),
  last_90d: windowToSeconds('1d'),
  last_6m: windowToSeconds('7d'),
  last_1y: windowToSeconds('1mo'),
  custom: -1
}

export const getMinWindowSecondsForDuration = (durationMs: number): number => {
  const durationHours = durationMs / (1000 * 60 * 60)
  if (durationHours < 3) return timeRangeMinWindowSeconds.last_1h
  if (durationHours < 6) return timeRangeMinWindowSeconds.last_3h
  if (durationHours < 12) return timeRangeMinWindowSeconds.last_6h
  if (durationHours < 24) return timeRangeMinWindowSeconds.last_12h
  if (durationHours < 3 * 24) return timeRangeMinWindowSeconds.last_24h
  if (durationHours < 7 * 24) return timeRangeMinWindowSeconds.last_3d
  if (durationHours < 15 * 24) return timeRangeMinWindowSeconds.last_7d
  if (durationHours < 30 * 24) return timeRangeMinWindowSeconds.last_15d
  if (durationHours < 60 * 24) return timeRangeMinWindowSeconds.last_30d
  if (durationHours < 90 * 24) return timeRangeMinWindowSeconds.last_60d
  if (durationHours < 180 * 24) return timeRangeMinWindowSeconds.last_90d
  if (durationHours < 365 * 24) return timeRangeMinWindowSeconds.last_6m
  return timeRangeMinWindowSeconds.last_1y
}

export const getMinWindowSecondsForFilter = (
  timeRange: FilterParams['time_range'],
  dateRange: [number, number] | null
): number => {
  if (timeRange === 'custom') {
    if (dateRange && dateRange.length === 2) {
      return getMinWindowSecondsForDuration(dateRange[1] - dateRange[0])
    }
    return -1
  }

  return timeRangeMinWindowSeconds[timeRange ?? 'last_1h']
}

export const buildAggregateWindowOptions = (
  options: AggregateWindowOptionBase[],
  minSeconds: number
): AggregateWindowOption[] =>
  options.map((option) => ({
    ...option,
    disabled:
      (option.value === 'no_aggregate' && minSeconds > -1) || (option.seconds !== -1 && option.seconds < minSeconds)
  }))

export const normalizeAggregateWindowSelection = (
  aggregateWindow: FilterParams['aggregate_window'],
  minSeconds: number,
  aggregateWindowOptions: AggregateWindowOption[]
): FilterParams['aggregate_window'] => {
  const currentWindowSeconds = windowToSeconds(aggregateWindow ?? 'no_aggregate')

  if (minSeconds > -1 && currentWindowSeconds === -1) {
    const firstValidOption = aggregateWindowOptions.find((opt) => !opt.disabled && opt.value !== 'no_aggregate')
    return firstValidOption ? firstValidOption.value : '30s'
  }

  if (currentWindowSeconds !== -1 && currentWindowSeconds < minSeconds) {
    const firstValidOption = aggregateWindowOptions.find((opt) => !opt.disabled)
    return firstValidOption ? firstValidOption.value : 'no_aggregate'
  }

  return aggregateWindow
}

export const applyAggregateWindowValidation = (
  filterParams: FilterParams,
  aggregateWindowOptions: AggregateWindowOption[],
  minSeconds: number
): boolean => {
  const normalizedWindow = normalizeAggregateWindowSelection(
    filterParams.aggregate_window,
    minSeconds,
    aggregateWindowOptions
  )
  const windowChanged = filterParams.aggregate_window !== normalizedWindow

  if (windowChanged) {
    filterParams.aggregate_window = normalizedWindow
  }

  if (filterParams.aggregate_window === 'no_aggregate') {
    delete filterParams.aggregate_function
  } else if (!filterParams.aggregate_function || windowChanged) {
    filterParams.aggregate_function = 'avg'
  }

  return windowChanged
}

export const canFetchWithCurrentFilters = (filterParams: FilterParams): boolean =>
  filterParams.time_range !== 'custom' || (!!filterParams.start_time && !!filterParams.end_time)

export const buildTelemetryHistoryParams = (
  deviceId: string,
  key: string,
  filterParams: FilterParams,
  isExport: boolean
): Record<string, unknown> => {
  const params: Record<string, unknown> = {
    device_id: deviceId,
    key,
    ...filterParams
  }

  if (params.time_range !== 'custom') {
    delete params.start_time
    delete params.end_time
  }
  if (params.aggregate_window === 'no_aggregate') {
    delete params.aggregate_function
  } else if (!params.aggregate_function) {
    params.aggregate_function = 'avg'
  }
  if (isExport) {
    params.is_export = true
  }

  return params
}

export const cloneFilterParams = (filterParams: FilterParams): FilterParams => JSON.parse(JSON.stringify(filterParams))
