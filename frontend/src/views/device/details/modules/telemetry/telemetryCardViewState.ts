import { computed, onUnmounted, ref, toValue, watch, type MaybeRefOrGetter } from 'vue'

export const TELEMETRY_CARD_SORT_MODE = {
  default: 'default',
  name: 'name',
  lastUpdate: 'lastUpdate'
} as const

export const TELEMETRY_CARD_FRESHNESS_FILTER = {
  all: 'all',
  attention: 'attention',
  stale: 'stale',
  missingTimestamp: 'missingTimestamp'
} as const

export const TELEMETRY_CARD_FRESHNESS_STATUS = {
  fresh: 'fresh',
  stale: 'stale',
  missingTimestamp: 'missingTimestamp',
  invalidTimestamp: 'invalidTimestamp'
} as const

const DEFAULT_STALE_MS = 15 * 60 * 1000
const DEFAULT_DISPLAY_LIMIT = 24
const DEFAULT_MAX_RENDER_LIMIT = 240
const DEFAULT_SEARCH_DEBOUNCE_MS = 180

export type TelemetryCardSortMode = (typeof TELEMETRY_CARD_SORT_MODE)[keyof typeof TELEMETRY_CARD_SORT_MODE]
export type TelemetryCardFreshnessFilter =
  (typeof TELEMETRY_CARD_FRESHNESS_FILTER)[keyof typeof TELEMETRY_CARD_FRESHNESS_FILTER]
export type TelemetryCardFreshnessStatus =
  (typeof TELEMETRY_CARD_FRESHNESS_STATUS)[keyof typeof TELEMETRY_CARD_FRESHNESS_STATUS]

type TelemetryCardFreshnessType = 'default' | 'success' | 'warning' | 'error' | 'info'

export type TelemetryCardFreshness = {
  ageMs: number | null
  i18nKey: string
  status: TelemetryCardFreshnessStatus
  tagType: TelemetryCardFreshnessType
  timestampMs: number | null
}

export type TelemetryCardRecord = DeviceManagement.telemetryData & {
  key?: string
  label?: string
  ts?: string | number | null
}

type TelemetryCardViewOptions = {
  displayLimit?: number
  maxRenderLimit?: number
  now?: () => number
  searchDebounceMs?: number
  staleMs?: number
}

// Freshness computation only needs the clock + stale threshold; keep this narrow
// so callers do not have to supply display/render/debounce fields they don't use.
type TelemetryFreshnessOptions = {
  now: () => number
  staleMs: number
}

const normalizeText = (value: unknown) =>
  String(value || '')
    .trim()
    .toLowerCase()

const telemetrySearchText = (telemetry: TelemetryCardRecord) =>
  [telemetry.label, telemetry.key, telemetry.unit].map(normalizeText).join(' ')

const telemetryTimestamp = (telemetry: TelemetryCardRecord) => {
  if (!telemetry.ts) return 0
  const raw = new Date(telemetry.ts).getTime()
  return Number.isFinite(raw) ? raw : 0
}

const compareTelemetryName = (left: TelemetryCardRecord, right: TelemetryCardRecord) =>
  (left.label || left.key || '').localeCompare(right.label || right.key || '')

export const getTelemetryFreshness = (
  telemetry: TelemetryCardRecord,
  options: TelemetryFreshnessOptions = {
    now: Date.now,
    staleMs: DEFAULT_STALE_MS
  }
): TelemetryCardFreshness => {
  if (!telemetry.ts) {
    return {
      ageMs: null,
      i18nKey: 'custom.device_details.telemetryFreshnessNoTimestamp',
      status: TELEMETRY_CARD_FRESHNESS_STATUS.missingTimestamp,
      tagType: 'default',
      timestampMs: null
    }
  }

  const timestampMs = new Date(telemetry.ts).getTime()

  if (!Number.isFinite(timestampMs)) {
    return {
      ageMs: null,
      i18nKey: 'custom.device_details.telemetryFreshnessInvalidTimestamp',
      status: TELEMETRY_CARD_FRESHNESS_STATUS.invalidTimestamp,
      tagType: 'warning',
      timestampMs: null
    }
  }

  const ageMs = Math.max(options.now() - timestampMs, 0)

  if (ageMs > options.staleMs) {
    return {
      ageMs,
      i18nKey: 'custom.device_details.telemetryFreshnessStale',
      status: TELEMETRY_CARD_FRESHNESS_STATUS.stale,
      tagType: 'warning',
      timestampMs
    }
  }

  return {
    ageMs,
    i18nKey: 'custom.device_details.telemetryFreshnessFresh',
    status: TELEMETRY_CARD_FRESHNESS_STATUS.fresh,
    tagType: 'success',
    timestampMs
  }
}

const matchesFreshnessFilter = (
  telemetry: TelemetryCardRecord,
  filter: TelemetryCardFreshnessFilter,
  freshnessOptions: TelemetryFreshnessOptions
) => {
  if (filter === TELEMETRY_CARD_FRESHNESS_FILTER.all) return true

  const freshness = getTelemetryFreshness(telemetry, freshnessOptions)

  if (filter === TELEMETRY_CARD_FRESHNESS_FILTER.attention) {
    return freshness.status !== TELEMETRY_CARD_FRESHNESS_STATUS.fresh
  }

  if (filter === TELEMETRY_CARD_FRESHNESS_FILTER.stale) {
    return freshness.status === TELEMETRY_CARD_FRESHNESS_STATUS.stale
  }

  return (
    freshness.status === TELEMETRY_CARD_FRESHNESS_STATUS.missingTimestamp ||
    freshness.status === TELEMETRY_CARD_FRESHNESS_STATUS.invalidTimestamp
  )
}

export const useTelemetryCardViewState = (
  telemetryData: MaybeRefOrGetter<TelemetryCardRecord[]>,
  options: TelemetryCardViewOptions = {}
) => {
  const freshnessOptions = {
    now: options.now || Date.now,
    staleMs: options.staleMs || DEFAULT_STALE_MS
  }
  const telemetrySearchQuery = ref('')
  const telemetrySortMode = ref<TelemetryCardSortMode>(TELEMETRY_CARD_SORT_MODE.default)
  const telemetryFreshnessFilter = ref<TelemetryCardFreshnessFilter>(TELEMETRY_CARD_FRESHNESS_FILTER.all)
  const showAllTelemetryCards = ref(false)
  const displayLimit = options.displayLimit || DEFAULT_DISPLAY_LIMIT
  const maxRenderLimit = Math.max(options.maxRenderLimit || DEFAULT_MAX_RENDER_LIMIT, displayLimit)
  const searchDebounceMs = options.searchDebounceMs ?? DEFAULT_SEARCH_DEBOUNCE_MS
  const debouncedTelemetrySearchQuery = ref('')
  let searchDebounceTimer: number | null = null

  const clearSearchDebounceTimer = () => {
    if (searchDebounceTimer === null) return
    window.clearTimeout(searchDebounceTimer)
    searchDebounceTimer = null
  }

  watch(telemetrySearchQuery, (query) => {
    clearSearchDebounceTimer()

    if (!query.trim() || searchDebounceMs <= 0 || typeof window === 'undefined') {
      debouncedTelemetrySearchQuery.value = query
      return
    }

    searchDebounceTimer = window.setTimeout(() => {
      debouncedTelemetrySearchQuery.value = query
      searchDebounceTimer = null
    }, searchDebounceMs)
  })

  onUnmounted(clearSearchDebounceTimer)

  const visibleTelemetryData = computed(() => {
    const source = [...toValue(telemetryData)]
    const query = normalizeText(debouncedTelemetrySearchQuery.value)

    const filteredBySearch = query
      ? source.filter((telemetry) => telemetrySearchText(telemetry).includes(query))
      : source
    const filtered = filteredBySearch.filter((telemetry) =>
      matchesFreshnessFilter(telemetry, telemetryFreshnessFilter.value, freshnessOptions)
    )

    if (telemetrySortMode.value === TELEMETRY_CARD_SORT_MODE.name) {
      return filtered.sort(compareTelemetryName)
    }

    if (telemetrySortMode.value === TELEMETRY_CARD_SORT_MODE.lastUpdate) {
      return filtered.sort((left, right) => telemetryTimestamp(right) - telemetryTimestamp(left))
    }

    return filtered
  })

  const visibleTelemetryCount = computed(() => visibleTelemetryData.value.length)
  const isTelemetryDisplayCapped = computed(
    () => visibleTelemetryData.value.length > (showAllTelemetryCards.value ? maxRenderLimit : displayLimit)
  )
  const telemetryRenderLimit = computed(() => (showAllTelemetryCards.value ? maxRenderLimit : displayLimit))
  const isTelemetryHardRenderCapped = computed(
    () => showAllTelemetryCards.value && visibleTelemetryData.value.length > maxRenderLimit
  )
  const displayTelemetryData = computed(() =>
    isTelemetryDisplayCapped.value
      ? visibleTelemetryData.value.slice(0, telemetryRenderLimit.value)
      : visibleTelemetryData.value
  )
  const displayTelemetryCount = computed(() => displayTelemetryData.value.length)
  const attentionTelemetryCount = computed(
    () =>
      toValue(telemetryData).filter(
        (telemetry) =>
          getTelemetryFreshness(telemetry, freshnessOptions).status !== TELEMETRY_CARD_FRESHNESS_STATUS.fresh
      ).length
  )

  const hasTelemetryCardFilters = computed(
    () =>
      telemetrySortMode.value !== TELEMETRY_CARD_SORT_MODE.default ||
      telemetryFreshnessFilter.value !== TELEMETRY_CARD_FRESHNESS_FILTER.all ||
      Boolean(telemetrySearchQuery.value.trim())
  )

  const clearTelemetryCardFilters = () => {
    clearSearchDebounceTimer()
    telemetrySearchQuery.value = ''
    debouncedTelemetrySearchQuery.value = ''
    telemetrySortMode.value = TELEMETRY_CARD_SORT_MODE.default
    telemetryFreshnessFilter.value = TELEMETRY_CARD_FRESHNESS_FILTER.all
    showAllTelemetryCards.value = false
  }

  const toggleTelemetryDisplayLimit = () => {
    showAllTelemetryCards.value = !showAllTelemetryCards.value
  }

  const getTelemetryFreshnessBadge = (telemetry: TelemetryCardRecord) =>
    getTelemetryFreshness(telemetry, freshnessOptions)

  return {
    attentionTelemetryCount,
    clearTelemetryCardFilters,
    displayTelemetryCount,
    displayTelemetryData,
    getTelemetryFreshnessBadge,
    hasTelemetryCardFilters,
    isTelemetryDisplayCapped,
    isTelemetryHardRenderCapped,
    showAllTelemetryCards,
    telemetryFreshnessFilter,
    telemetrySearchQuery,
    telemetrySortMode,
    toggleTelemetryDisplayLimit,
    visibleTelemetryCount,
    visibleTelemetryData
  }
}
