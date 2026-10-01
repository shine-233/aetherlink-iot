import { defineComponent, h, reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  alarmHistory: vi.fn(),
  socketStart: vi.fn(),
  socketStop: vi.fn(),
  onEvent: null as null | (() => void)
}))

vi.mock('@/service/api/alarm', () => ({ alarmHistory: hoisted.alarmHistory }))
vi.mock('@/hooks/alarm/useAlarmStatusSocket', () => ({
  useAlarmStatusSocket: (cb: () => void) => {
    hoisted.onEvent = cb
    return { start: hoisted.socketStart, stop: hoisted.socketStop }
  }
}))

import {
  ALARM_REALTIME_REFRESH_DEBOUNCE_MS,
  formatAlarmRangeQuery,
  useAlarmHistoryQuery,
  type AlarmHistoryQueryProps
} from '../useAlarmHistoryQuery'
import { buildAlarmAuditLogRoute, buildAlarmDeviceReadyCheckRoute } from '../alarm-detail-routes'

const mountQuery = (props: AlarmHistoryQueryProps) => {
  let api!: ReturnType<typeof useAlarmHistoryQuery>
  const wrapper = mount(
    defineComponent({
      setup() {
        api = useAlarmHistoryQuery(props)
        return () => h('div')
      }
    })
  )
  return { wrapper, api }
}

describe('useAlarmHistoryQuery', () => {
  beforeEach(() => {
    hoisted.alarmHistory.mockReset()
    hoisted.socketStart.mockReset()
    hoisted.socketStop.mockReset()
    hoisted.alarmHistory.mockResolvedValue({ data: { total: 3, list: [{ id: 'a' }] } })
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads on mount with the focused fleet device and pagination', async () => {
    const { api, wrapper } = mountQuery(
      reactive({ fleetContext: { deviceIds: ['dev-1', 'dev-2'], currentPageCount: 2, requestedTotal: 5 } as any })
    )
    await flushPromises()
    expect(hoisted.socketStart).toHaveBeenCalledTimes(1)
    expect(hoisted.alarmHistory).toHaveBeenCalledWith(
      expect.objectContaining({ device_id: 'dev-1', page: 1, page_size: 10 })
    )
    expect(api.tableData.value).toEqual([{ id: 'a' }])
    expect(api.pagination.itemCount).toBe(3)
    expect(api.fleetDeviceCount.value).toBe(2)
    wrapper.unmount()
  })

  it('drops stale responses so an older request cannot overwrite a newer one', async () => {
    let resolveFirst!: (v: unknown) => void
    hoisted.alarmHistory
      .mockReturnValueOnce(new Promise((resolve) => (resolveFirst = resolve)))
      .mockResolvedValueOnce({ data: { total: 1, list: [{ id: 'new' }] } })
    const { api, wrapper } = mountQuery({})
    api.handleSearch()
    await flushPromises()
    resolveFirst({ data: { total: 9, list: [{ id: 'old' }] } })
    await flushPromises()
    expect(api.tableData.value).toEqual([{ id: 'new' }])
    expect(api.loading.value).toBe(false)
    wrapper.unmount()
  })

  it('page size change resets page and selection', async () => {
    const { api, wrapper } = mountQuery({})
    await flushPromises()
    api.selectedAlarmRowKeys.value = ['a']
    api.pagination.page = 4
    api.pagination.onUpdatePageSize?.(20)
    await flushPromises()
    expect(api.pagination.page).toBe(1)
    expect(api.pagination.pageSize).toBe(20)
    expect(api.selectedAlarmRowKeys.value).toEqual([])
    expect(hoisted.alarmHistory).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, page_size: 20 }))
    wrapper.unmount()
  })

  it('debounces realtime events and cancels the pending refresh on unmount', async () => {
    vi.useFakeTimers()
    const { wrapper } = mountQuery({})
    await flushPromises()
    expect(hoisted.alarmHistory).toHaveBeenCalledTimes(1)

    hoisted.onEvent?.()
    hoisted.onEvent?.()
    vi.advanceTimersByTime(ALARM_REALTIME_REFRESH_DEBOUNCE_MS)
    await flushPromises()
    expect(hoisted.alarmHistory).toHaveBeenCalledTimes(2)

    hoisted.onEvent?.()
    wrapper.unmount()
    expect(hoisted.socketStop).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(ALARM_REALTIME_REFRESH_DEBOUNCE_MS * 2)
    await flushPromises()
    expect(hoisted.alarmHistory).toHaveBeenCalledTimes(2)
  })

  it('formats empty and populated ranges', () => {
    expect(formatAlarmRangeQuery(null)).toEqual({ start_time: '', end_time: '' })
    const out = formatAlarmRangeQuery([0, 1000])
    expect(out.start_time).toMatch(/^1970-01-01T/)
    expect(out.end_time).toMatch(/^1970-01-01T/)
  })
})

describe('alarm-detail-routes', () => {
  it('builds a ready-check route only when a device id exists', () => {
    expect(buildAlarmDeviceReadyCheckRoute({}, 'h1')).toBeNull()
    expect(buildAlarmDeviceReadyCheckRoute({ device_id: ' d1 ' }, 'h1')).toEqual({
      path: '/device/details',
      query: { d_id: 'd1', tab: 'ready-check', source: 'alarm', alarm_history_id: 'h1' }
    })
  })

  it('builds the audit log window around alarm creation time', () => {
    const route = buildAlarmAuditLogRoute('2026-06-20T12:00:00Z')
    expect(route.name).toBe('system-management-user_system-log')
    expect(route.query.method).toBe('PUT')
    expect(route.query.path).toBe('/api/v1/alarm/info/history')
    expect(new Date(route.query.start_time).toISOString()).toBe('2026-06-20T11:00:00.000Z')
  })
})
