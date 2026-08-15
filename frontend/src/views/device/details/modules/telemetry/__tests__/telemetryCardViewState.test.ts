import { describe, expect, it } from 'vitest'
import { nextTick, ref } from 'vue'
import {
  getTelemetryFreshness,
  TELEMETRY_CARD_FRESHNESS_FILTER,
  TELEMETRY_CARD_FRESHNESS_STATUS,
  TELEMETRY_CARD_SORT_MODE,
  useTelemetryCardViewState
} from '../telemetryCardViewState'

const NOW = new Date('2026-07-05T02:00:00.000Z').getTime()

const record = (key: string, ts?: string | number | null, label = key) =>
  ({
    key,
    label,
    ts,
    value: 1,
    unit: 'C'
  }) as DeviceManagement.telemetryData & { key: string; label: string; ts?: string | number | null }

describe('telemetryCardViewState', () => {
  it('classifies fresh, stale, missing, and invalid telemetry timestamps', () => {
    expect(
      getTelemetryFreshness(record('fresh', '2026-07-05T01:55:00.000Z'), {
        now: () => NOW,
        staleMs: 15 * 60 * 1000
      }).status
    ).toBe(TELEMETRY_CARD_FRESHNESS_STATUS.fresh)

    expect(
      getTelemetryFreshness(record('stale', '2026-07-05T01:40:00.000Z'), {
        now: () => NOW,
        staleMs: 15 * 60 * 1000
      }).status
    ).toBe(TELEMETRY_CARD_FRESHNESS_STATUS.stale)

    expect(getTelemetryFreshness(record('missing', null)).status).toBe(TELEMETRY_CARD_FRESHNESS_STATUS.missingTimestamp)
    expect(getTelemetryFreshness(record('invalid', 'not-a-date')).status).toBe(
      TELEMETRY_CARD_FRESHNESS_STATUS.invalidTimestamp
    )
  })

  it('filters telemetry by search text and freshness state', async () => {
    const telemetryData = ref([
      record('temp', '2026-07-05T01:59:00.000Z', 'Temperature'),
      record('pressure', '2026-07-05T01:30:00.000Z', 'Pressure'),
      record('humidity', null, 'Humidity')
    ])
    const state = useTelemetryCardViewState(telemetryData, {
      now: () => NOW,
      staleMs: 15 * 60 * 1000,
      searchDebounceMs: 0
    })

    state.telemetryFreshnessFilter.value = TELEMETRY_CARD_FRESHNESS_FILTER.attention
    expect(state.visibleTelemetryData.value.map((item) => item.key)).toEqual(['pressure', 'humidity'])
    expect(state.attentionTelemetryCount.value).toBe(2)

    state.telemetrySearchQuery.value = 'press'
    await nextTick()
    expect(state.visibleTelemetryData.value.map((item) => item.key)).toEqual(['pressure'])
  })

  it('sorts visible telemetry by name and last update', () => {
    const telemetryData = ref([
      record('temp', '2026-07-05T01:45:00.000Z', 'Temperature'),
      record('battery', '2026-07-05T01:59:00.000Z', 'Battery'),
      record('co2', '2026-07-05T01:50:00.000Z', 'CO2')
    ])
    const state = useTelemetryCardViewState(telemetryData, {
      now: () => NOW,
      staleMs: 15 * 60 * 1000
    })

    state.telemetrySortMode.value = TELEMETRY_CARD_SORT_MODE.name
    expect(state.visibleTelemetryData.value.map((item) => item.key)).toEqual(['battery', 'co2', 'temp'])

    state.telemetrySortMode.value = TELEMETRY_CARD_SORT_MODE.lastUpdate
    expect(state.visibleTelemetryData.value.map((item) => item.key)).toEqual(['battery', 'co2', 'temp'])
  })

  it('clears search, sort, and freshness filters together', () => {
    const telemetryData = ref([record('temp', null)])
    const state = useTelemetryCardViewState(telemetryData)

    state.telemetrySearchQuery.value = 'temp'
    state.telemetrySortMode.value = TELEMETRY_CARD_SORT_MODE.name
    state.telemetryFreshnessFilter.value = TELEMETRY_CARD_FRESHNESS_FILTER.attention

    expect(state.hasTelemetryCardFilters.value).toBe(true)

    state.clearTelemetryCardFilters()

    expect(state.telemetrySearchQuery.value).toBe('')
    expect(state.telemetrySortMode.value).toBe(TELEMETRY_CARD_SORT_MODE.default)
    expect(state.telemetryFreshnessFilter.value).toBe(TELEMETRY_CARD_FRESHNESS_FILTER.all)
    expect(state.hasTelemetryCardFilters.value).toBe(false)
  })
})
