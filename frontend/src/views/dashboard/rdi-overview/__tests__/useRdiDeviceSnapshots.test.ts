import { ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  deviceList: vi.fn(),
  deviceAlarmStatus: vi.fn(),
  telemetryDataCurrentKeys: vi.fn(),
  rdiDeviceConfig: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceList: api.deviceList,
  deviceAlarmStatus: api.deviceAlarmStatus,
  telemetryDataCurrentKeys: api.telemetryDataCurrentKeys
}))

vi.mock('@/service/api/rdi', () => ({ rdiDeviceConfig: api.rdiDeviceConfig }))

import { useRdiDeviceSnapshots } from '../useRdiDeviceSnapshots'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

function createSnapshots(activeSystemsOnly = ref(false), isMasterAccount = ref(false)) {
  return useRdiDeviceSnapshots({ activeSystemsOnly, isMasterAccount })
}

describe('useRdiDeviceSnapshots', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.deviceList.mockResolvedValue({ data: { list: [], total: 0 } })
    api.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })
    api.deviceAlarmStatus.mockResolvedValue({ data: { alarm: false } })
    api.rdiDeviceConfig.mockResolvedValue({ error: null, data: { system_info: {} } })
  })

  afterEach(() => {
    vi.useRealTimers()
    delete (window as any).requestIdleCallback
    delete (window as any).cancelIdleCallback
  })

  it('uses responsive default, active-system, and master request parameters', async () => {
    const activeSystemsOnly = ref(false)
    const isMasterAccount = ref(false)
    const snapshots = createSnapshots(activeSystemsOnly, isMasterAccount)

    await snapshots.fetchDeviceSnapshots()
    expect(api.deviceList).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 12,
      include_rdi_system_info_summary: true
    })

    activeSystemsOnly.value = true
    await snapshots.fetchDeviceSnapshots()
    expect(api.deviceList).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 12,
      include_rdi_system_info_summary: true,
      warn_status: 'Y'
    })

    isMasterAccount.value = true
    await snapshots.fetchDeviceSnapshots()
    expect(api.deviceList).toHaveBeenLastCalledWith({
      page: 1,
      page_size: 12,
      include_rdi_system_info_summary: true,
      warn_status: 'Y',
      all_tenants: true
    })
  })

  it('hydrates missing RDI system info but trusts an explicitly empty summary', async () => {
    api.deviceList.mockResolvedValue({
      data: {
        list: [
          { id: 'fallback', pid_number: 'PID-1', system_info: {} },
          { id: 'explicit-empty', pid_number: 'PID-2', rdi_system_info_summary: {} }
        ]
      }
    })
    api.rdiDeviceConfig.mockResolvedValue({
      error: null,
      data: { system_info: { controller_serial_number: 'CONFIG-SN', installation_location: 'Plant A' } }
    })
    const snapshots = createSnapshots()

    await snapshots.fetchDeviceSnapshots()

    expect(api.rdiDeviceConfig).toHaveBeenCalledTimes(1)
    expect(api.rdiDeviceConfig).toHaveBeenCalledWith('fallback')
    expect(snapshots.deviceSnapshots.value[0]).toMatchObject({ serialNumber: 'CONFIG-SN', installLocation: 'Plant A' })
    expect(snapshots.deviceSnapshots.value[1].serialNumber).toBe('--')
  })

  it('degrades telemetry and alarm failures without dropping the snapshot', async () => {
    api.deviceList.mockResolvedValue({ data: { list: [{ id: 'device-1', name: 'Device 1', is_online: 1 }] } })
    api.telemetryDataCurrentKeys.mockRejectedValue(new Error('telemetry unavailable'))
    api.deviceAlarmStatus.mockRejectedValue(new Error('alarm unavailable'))
    const snapshots = createSnapshots()

    await snapshots.fetchDeviceSnapshots()

    expect(snapshots.deviceSnapshots.value[0]).toMatchObject({ id: 'device-1', telemetry: {}, alarm: null })
    expect(snapshots.snapshotLoading.value).toBe(false)
  })

  it('keeps only the latest request result and loading ownership', async () => {
    const first = deferred<any>()
    const second = deferred<any>()
    api.deviceList.mockImplementationOnce(() => first.promise).mockImplementationOnce(() => second.promise)
    const snapshots = createSnapshots()

    const firstFetch = snapshots.fetchDeviceSnapshots()
    const secondFetch = snapshots.fetchDeviceSnapshots()
    second.resolve({ data: { list: [{ id: 'latest' }], total: 1 } })
    await secondFetch
    expect(snapshots.deviceSnapshots.value[0].id).toBe('latest')
    expect(snapshots.snapshotLoading.value).toBe(false)

    first.resolve({ data: { list: [{ id: 'stale' }], total: 99 } })
    await firstFetch
    expect(snapshots.deviceSnapshots.value[0].id).toBe('latest')
    expect(snapshots.snapshotTotal.value).toBe(1)
    expect(snapshots.snapshotLoading.value).toBe(false)
  })

  it('changes page before requesting the next server page', async () => {
    const snapshots = createSnapshots()

    snapshots.changeSnapshotPage(3)
    await vi.waitFor(() => expect(api.deviceList).toHaveBeenCalledTimes(1))

    expect(snapshots.snapshotPage.value).toBe(3)
    expect(api.deviceList).toHaveBeenCalledWith({
      page: 3,
      page_size: 12,
      include_rdi_system_info_summary: true
    })
  })

  it('schedules with the 350ms fallback and cancel prevents the request', async () => {
    vi.useFakeTimers()
    const snapshots = createSnapshots()

    snapshots.scheduleDeviceSnapshotsRefresh()
    expect(snapshots.snapshotLoading.value).toBe(true)
    await vi.advanceTimersByTimeAsync(349)
    expect(api.deviceList).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    expect(api.deviceList).toHaveBeenCalledTimes(1)

    snapshots.scheduleDeviceSnapshotsRefresh()
    snapshots.cancelScheduledDeviceSnapshots()
    await vi.advanceTimersByTimeAsync(350)
    expect(api.deviceList).toHaveBeenCalledTimes(1)
  })

  it('uses and cancels idle callbacks when available', () => {
    const requestIdleCallback = vi.fn(() => 17)
    const cancelIdleCallback = vi.fn()
    ;(window as any).requestIdleCallback = requestIdleCallback
    ;(window as any).cancelIdleCallback = cancelIdleCallback
    const snapshots = createSnapshots()

    snapshots.scheduleDeviceSnapshotsRefresh()
    expect(requestIdleCallback).toHaveBeenCalledWith(expect.any(Function), { timeout: 1200 })
    snapshots.cancelScheduledDeviceSnapshots()
    expect(cancelIdleCallback).toHaveBeenCalledWith(17)
  })

  it('dispose invalidates an in-flight request', async () => {
    const pending = deferred<any>()
    api.deviceList.mockReturnValue(pending.promise)
    const snapshots = createSnapshots()

    const fetch = snapshots.fetchDeviceSnapshots()
    snapshots.dispose()
    pending.resolve({ data: { list: [{ id: 'after-dispose' }], total: 1 } })
    await fetch

    expect(snapshots.deviceSnapshots.value).toEqual([])
  })
})
