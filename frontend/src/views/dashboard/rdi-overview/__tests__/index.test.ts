/**
 * 文件用途：验证 frontend/src/views/dashboard/rdi-overview/__tests__/index 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import dayjs from 'dayjs'

const hoisted = vi.hoisted(() => ({
  alarmHistory: vi.fn(),
  alarmHistoryMonthlyTrend: vi.fn(),
  acknowledgeAlarmHistory: vi.fn(),
  resetAlarmHistory: vi.fn(),
  deviceList: vi.fn(),
  deviceAlarmStatus: vi.fn(),
  telemetryDataCurrentKeys: vi.fn(),
  rdiDeviceConfig: vi.fn(),
  getAlarmCount: vi.fn(),
  sumData: vi.fn(),
  authUserInfo: {
    authority: 'TENANT_ADMIN',
    roles: ['TENANT_ADMIN']
  },
  routerPush: vi.fn(),
  routerBack: vi.fn(),
  messageSuccess: vi.fn(),
  dialogWarning: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: hoisted.routerPush,
    back: hoisted.routerBack
  })
}))

vi.mock('@/service/api/alarm', () => ({
  alarmHistory: hoisted.alarmHistory,
  alarmHistoryMonthlyTrend: hoisted.alarmHistoryMonthlyTrend,
  acknowledgeAlarmHistory: hoisted.acknowledgeAlarmHistory,
  resetAlarmHistory: hoisted.resetAlarmHistory
}))

vi.mock('@/service/api/device', () => ({
  deviceList: hoisted.deviceList,
  deviceAlarmStatus: hoisted.deviceAlarmStatus,
  telemetryDataCurrentKeys: hoisted.telemetryDataCurrentKeys
}))

vi.mock('@/service/api/rdi', () => ({
  rdiDeviceConfig: hoisted.rdiDeviceConfig
}))

vi.mock('@/service/api/system-data', () => ({
  getAlarmCount: hoisted.getAlarmCount,
  sumData: hoisted.sumData
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({
    userInfo: hoisted.authUserInfo
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import { normalizeAlarmMonthlyTrendPoints } from '../rdiOverviewState'
import RdiOverview from '../index.vue'

interface AlarmCellVNode {
  props: {
    text?: unknown
    type?: unknown
    class?: unknown
    size?: unknown
    disabled?: unknown
    onClick: () => unknown
  }
  children: Array<AlarmCellVNode> & { default: () => unknown }
}

interface AlarmColumn {
  key: string
  title: () => unknown
  render: (row: Record<string, unknown>) => AlarmCellVNode
}

interface DeviceSnapshot {
  id: string
  name: string
  pid: string
  firmware: string
  online: boolean | null
  alarm: boolean | null
  serialNumber: string
  tenantId: string
  telemetry: Record<string, unknown>
}

interface RdiOverviewSetupState {
  systemsCardTitleKey: unknown
  systemsEmptyTitleKey: unknown
  systemsEmptyDescriptionKey: unknown
  fetchDeviceSnapshots: () => Promise<unknown>
  changeSnapshotPage: (page: number) => void
  parseAlarmRemark: (value: unknown) => unknown
  isAcknowledged: (alarm: { remark?: unknown }) => boolean
  formatTime: (value?: unknown) => unknown
  alarmStatusLabel: (status?: unknown) => unknown
  alarmTagType: (status?: unknown) => unknown
  alarmTypeLabel: (alarm: Record<string, unknown>) => unknown
  goDevice: (deviceId?: string) => void
  goBack: () => void
  normalizeDeviceRows: (payload: unknown) => unknown[]
  normalizeTelemetry: (payload: unknown) => Record<string, unknown>
  rowText: (row: Record<string, unknown>, keys: string[], fallback?: string) => unknown
  isRowOnline: (row: Record<string, unknown>) => boolean
  formatTemperature: (value?: unknown) => unknown
  formatSwitch: (value?: unknown) => unknown
  temperatureUnit: string
  stats: {
    totalDevices: number
    onlineDevices: number
    offlineDevices: number
    alarmHistoryTotal: number
    activeAlarms: number
  }
  loading: boolean
  fetchDevices: () => Promise<unknown>
  alarmDeviceTotal: number
  fetchCounts: () => Promise<unknown>
  alarmPagination: {
    itemCount: number
    page: number
    pageSize: number
    onChange: (page: number) => void
    onUpdatePageSize: (size: number) => void
  }
  queryParams: {
    alarm_status?: string
    page: number
    page_size: number
  }
  searchAlarms: () => void
  alarmColumns: AlarmColumn[]
  deviceSnapshots: DeviceSnapshot[]
  snapshotTotal: number
  snapshotLoading: boolean
  snapshotPage: number
  hasInstallationInfo: (snapshot: DeviceSnapshot) => boolean
  snapshotStatusLabel: (snapshot: DeviceSnapshot) => unknown
  snapshotStatusTagType: (snapshot: DeviceSnapshot) => unknown
  alarms: Array<{ id: string }>
  acknowledgeAlarm: (alarm: { id: string }) => Promise<unknown>
  resetAlarm: (alarm: { id?: string; name?: string; content?: string }) => void
  resetAlarmFilter: () => void
  refreshAll: () => Promise<unknown>
  fetchActiveAlarmCounts: () => Promise<unknown>
  fetchAlarms: () => Promise<unknown>
  fetchAlarmTrend: () => Promise<unknown>
  alarmTrendYear: number
  alarmTrendLoading: boolean
  alarmTrendPoints: Array<{ month: unknown; count: unknown }>
  alarmTrendChartOptions: {
    xAxis: { data: unknown }
    series: Array<{ data: unknown; name: unknown }>
  }
  alarmStatusOptions: Array<{ label: unknown; value: unknown }>
}

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props: { activeSystemsOnly?: boolean } = {}) => {
  const wrapper = shallowMount(RdiOverview, {
    props,
    global: {
      stubs: {
        NSpace: true,
        NCard: true,
        NStatistic: true,
        NButton: true,
        NTag: true,
        NSelect: true,
        NInput: true,
        NTreeSelect: true,
        NDataTable: true,
        NPagination: true,
        NSpin: true,
        NAlert: true,
        NEmpty: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as RdiOverviewSetupState

describe('rdi-overview/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.removeItem('rdi-temperature-unit')
    hoisted.authUserInfo.authority = 'TENANT_ADMIN'
    hoisted.authUserInfo.roles = ['TENANT_ADMIN']
    hoisted.sumData.mockResolvedValue({
      data: {
        device_total: 100,
        device_on: 80,
        device_offline: 20
      }
    })
    hoisted.getAlarmCount.mockResolvedValue({
      data: { alarm_device_total: 5 }
    })
    hoisted.alarmHistory.mockResolvedValue({
      data: { list: [], total: 0 }
    })
    hoisted.alarmHistoryMonthlyTrend.mockResolvedValue({
      data: { year: dayjs().year(), months: [] }
    })
    hoisted.deviceList.mockResolvedValue({
      data: { list: [] }
    })
    hoisted.deviceAlarmStatus.mockResolvedValue({ data: { alarm: false } })
    hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })
    hoisted.rdiDeviceConfig.mockResolvedValue({ error: null, data: { system_info: {} } })
    hoisted.acknowledgeAlarmHistory.mockResolvedValue({ error: null })
    hoisted.resetAlarmHistory.mockResolvedValue({ error: null })
    window.$message = {
      success: hoisted.messageSuccess,
      error: vi.fn(),
      warning: vi.fn(),
      info: vi.fn()
    } as unknown as Window['$message']
    window.$dialog = {
      warning: hoisted.dialogWarning
    } as unknown as Window['$dialog']
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  describe('alarm summary labels', () => {
    it('labels the unfiltered alarm total as history records instead of recent alarms', async () => {
      const wrapper = mountComponent()
      await flushPromises()

      const labels = wrapper.findAllComponents({ name: 'NStatistic' }).map((statistic) => statistic.attributes('label'))
      expect(labels).toContain('rdi.overview.alarmHistoryTotal')
      expect(labels).not.toContain('rdi.overview.recentAlarms')
    })
  })

  describe('system list mode', () => {
    it('keeps the dashboard route on the unfiltered all-systems list', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 } })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.systemsCardTitleKey).toBe('rdi.overview.allSystems')
      expect(setupState.systemsEmptyTitleKey).toBe('rdi.overview.noSnapshotTitle')
      expect(setupState.systemsEmptyDescriptionKey).toBe('rdi.overview.noSnapshotDesc')
      expect(hoisted.deviceList).toHaveBeenCalledWith({
        page: 1,
        page_size: 12,
        include_rdi_system_info_summary: true
      })
    })

    it('uses server-side active-system filtering and dedicated copy for the Alerts route', async () => {
      const wrapper = mountComponent({ activeSystemsOnly: true })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 } })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.systemsCardTitleKey).toBe('rdi.overview.activeSystems')
      expect(setupState.systemsEmptyTitleKey).toBe('rdi.overview.noActiveSystemsTitle')
      expect(setupState.systemsEmptyDescriptionKey).toBe('rdi.overview.noActiveSystemsDesc')
      expect(hoisted.deviceList).toHaveBeenCalledWith({
        page: 1,
        page_size: 12,
        include_rdi_system_info_summary: true,
        warn_status: 'Y'
      })
    })

    it('preserves the active-system filter when moving to another server page', async () => {
      const wrapper = mountComponent({ activeSystemsOnly: true })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 25 } })

      setupState.changeSnapshotPage(2)
      await flushPromises()

      expect(hoisted.deviceList).toHaveBeenCalledWith({
        page: 2,
        page_size: 12,
        include_rdi_system_info_summary: true,
        warn_status: 'Y'
      })
    })
  })

  describe('parseAlarmRemark', () => {
    it('returns empty object for null/undefined', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.parseAlarmRemark(null)).toEqual({})
      expect(setupState.parseAlarmRemark(undefined)).toEqual({})
    })

    it('returns the object directly when input is an object', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const obj = { acknowledged: true }
      expect(setupState.parseAlarmRemark(obj)).toEqual(obj)
    })

    it('parses valid JSON string', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.parseAlarmRemark('{"acknowledged":true}')).toEqual({ acknowledged: true })
    })

    it('returns empty object for invalid JSON string', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.parseAlarmRemark('not json')).toEqual({})
    })

    it('returns empty object for non-string non-object types', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.parseAlarmRemark(123)).toEqual({})
      expect(setupState.parseAlarmRemark(true)).toEqual({})
    })

    it('returns empty object when JSON parses to non-object', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.parseAlarmRemark('"a string"')).toEqual({})
      expect(setupState.parseAlarmRemark('42')).toEqual({})
    })
  })

  describe('isAcknowledged', () => {
    it('returns true when remark.acknowledged is true (object)', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.isAcknowledged({ remark: { acknowledged: true } })).toBe(true)
    })

    it('returns true when remark is JSON string with acknowledged true', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.isAcknowledged({ remark: '{"acknowledged":true}' })).toBe(true)
    })

    it('returns false when remark.acknowledged is not true', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.isAcknowledged({ remark: { acknowledged: false } })).toBe(false)
      expect(setupState.isAcknowledged({ remark: null })).toBe(false)
      expect(setupState.isAcknowledged({})).toBe(false)
    })
  })

  describe('formatTime', () => {
    it('formats valid time string', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const result = setupState.formatTime('2024-01-15T10:30:00Z')
      expect(result).toBe(dayjs('2024-01-15T10:30:00Z').format('YYYY-MM-DD HH:mm:ss'))
    })

    it('returns dash for undefined/empty', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.formatTime(undefined)).toBe('-')
      expect(setupState.formatTime('')).toBe('-')
    })
  })

  describe('alarmStatusLabel', () => {
    it('returns correct labels for known statuses', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.alarmStatusLabel('H')).toBe('rdi.overview.high')
      expect(setupState.alarmStatusLabel('M')).toBe('rdi.overview.medium')
      expect(setupState.alarmStatusLabel('L')).toBe('rdi.overview.low')
      expect(setupState.alarmStatusLabel('N')).toBe('rdi.overview.normal')
    })

    it('returns the status itself for unknown statuses', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.alarmStatusLabel('X')).toBe('X')
    })

    it('returns dash for empty/undefined', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.alarmStatusLabel('')).toBe('-')
      expect(setupState.alarmStatusLabel(undefined)).toBe('-')
    })
  })

  describe('alarmTagType', () => {
    it('returns error for H, warning for M, info for L, success for others', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.alarmTagType('H')).toBe('error')
      expect(setupState.alarmTagType('M')).toBe('warning')
      expect(setupState.alarmTagType('L')).toBe('info')
      expect(setupState.alarmTagType('N')).toBe('success')
      expect(setupState.alarmTagType(undefined)).toBe('success')
    })
  })

  describe('alarmTypeLabel', () => {
    it('returns mapped label for known event_type', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.alarmTypeLabel({ remark: '{"event_type":"temperature_alarm"}' })).toBe(
        'rdi.overview.temperatureAlarm'
      )
      expect(setupState.alarmTypeLabel({ remark: '{"event_type":"switch_alarm"}' })).toBe('rdi.overview.switchAlarm')
      expect(setupState.alarmTypeLabel({ remark: '{"event_type":"warranty_alarm"}' })).toBe(
        'rdi.overview.warrantyAlarm'
      )
      expect(setupState.alarmTypeLabel({ remark: '{"event_type":"PT"}' })).toBe('rdi.overview.pressureAlarm')
    })

    it('falls back to name, content, or dash for unknown event_type', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.alarmTypeLabel({ remark: '{"event_type":"unknown"}', name: 'MyAlarm' })).toBe('MyAlarm')
      expect(setupState.alarmTypeLabel({ remark: '{}', content: 'ContentAlarm' })).toBe('ContentAlarm')
      expect(setupState.alarmTypeLabel({ remark: '{}' })).toBe('-')
    })
  })

  describe('goDevice', () => {
    it('pushes device_details route with d_id query', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.goDevice('device-123')
      expect(hoisted.routerPush).toHaveBeenCalledWith({
        name: 'device_details',
        query: { d_id: 'device-123', tab: 'message' }
      })
    })

    it('does nothing when deviceId is falsy', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.goDevice('')
      setupState.goDevice(undefined)
      expect(hoisted.routerPush).toHaveBeenCalledTimes(0)
    })
  })

  describe('goBack', () => {
    it('calls router.back()', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.goBack()
      expect(hoisted.routerBack).toHaveBeenCalledTimes(1)
    })
  })

  describe('normalizeDeviceRows', () => {
    it('returns data array from payload.data', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const payload = { data: [{ id: '1' }, { id: '2' }] }
      expect(setupState.normalizeDeviceRows(payload)).toEqual([{ id: '1' }, { id: '2' }])
    })

    it('returns data.list array', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const payload = { data: { list: [{ id: '1' }] } }
      expect(setupState.normalizeDeviceRows(payload)).toEqual([{ id: '1' }])
    })

    it('returns data.data.list array', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const payload = { data: { data: { list: [{ id: '1' }] } } }
      expect(setupState.normalizeDeviceRows(payload)).toEqual([{ id: '1' }])
    })

    it('returns payload directly if it is an array', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.normalizeDeviceRows([{ id: '1' }])).toEqual([{ id: '1' }])
    })

    it('returns empty array for unrecognized shapes', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.normalizeDeviceRows({})).toEqual([])
      expect(setupState.normalizeDeviceRows(null)).toEqual([])
    })
  })

  describe('normalizeTelemetry', () => {
    it('converts list array to key-value object', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const payload = {
        data: [
          { key: 'temp', value: 25 },
          { key: 'humidity', number_v: 60 }
        ]
      }
      expect(setupState.normalizeTelemetry(payload)).toEqual({ temp: 25, humidity: 60 })
    })

    it('returns object data directly', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const payload = { data: { temp: 25, humidity: 60 } }
      expect(setupState.normalizeTelemetry(payload)).toEqual({ temp: 25, humidity: 60 })
    })

    it('returns empty object for unrecognized shapes', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.normalizeTelemetry(null)).toEqual({})
      expect(setupState.normalizeTelemetry(42)).toEqual({})
    })
  })

  describe('rowText', () => {
    it('returns first non-empty value from keys', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const row = { a: '', b: 'found', c: 'other' }
      expect(setupState.rowText(row, ['a', 'b', 'c'])).toBe('found')
    })

    it('returns fallback when all keys are empty', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const row = { a: '', b: null, c: undefined }
      expect(setupState.rowText(row, ['a', 'b', 'c'], 'fallback')).toBe('fallback')
    })

    it('uses default fallback --', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.rowText({}, ['a', 'b'])).toBe('--')
    })
  })

  describe('isRowOnline', () => {
    it('returns true for various online representations', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.isRowOnline({ is_online: true })).toBe(true)
      expect(setupState.isRowOnline({ is_online: 1 })).toBe(true)
      expect(setupState.isRowOnline({ is_online: '1' })).toBe(true)
      expect(setupState.isRowOnline({ online: 'online' })).toBe(true)
      expect(setupState.isRowOnline({ status: 1 })).toBe(true)
    })

    it('returns false for offline representations', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.isRowOnline({ is_online: false })).toBe(false)
      expect(setupState.isRowOnline({ is_online: 0 })).toBe(false)
      expect(setupState.isRowOnline({ is_online: 'offline' })).toBe(false)
      expect(setupState.isRowOnline({})).toBe(false)
    })
  })

  describe('formatTemperature', () => {
    it('formats valid number with 2 decimal places and C suffix', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.formatTemperature(25.567)).toBe('25.57 C')
      expect(setupState.formatTemperature('30')).toBe('30.00 C')
      expect(setupState.formatTemperature(0)).toBe('0.00 C')
    })

    it('formats valid number with Fahrenheit suffix when selected', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.temperatureUnit = 'F'

      expect(setupState.formatTemperature(25)).toBe('77.00 F')
    })

    it('returns -- for non-numeric values', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.formatTemperature('abc')).toBe('--')
      expect(setupState.formatTemperature(undefined)).toBe('--')
      expect(setupState.formatTemperature(null)).toBe('--')
    })
  })

  describe('formatSwitch', () => {
    it('returns high for true/1/"1"/"true"/"high"', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.formatSwitch(true)).toBe('rdi.overview.high')
      expect(setupState.formatSwitch(1)).toBe('rdi.overview.high')
      expect(setupState.formatSwitch('1')).toBe('rdi.overview.high')
      expect(setupState.formatSwitch('true')).toBe('rdi.overview.high')
      expect(setupState.formatSwitch('high')).toBe('rdi.overview.high')
    })

    it('returns low for false/0/"0"/"false"/"low"', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.formatSwitch(false)).toBe('rdi.overview.low')
      expect(setupState.formatSwitch(0)).toBe('rdi.overview.low')
      expect(setupState.formatSwitch('0')).toBe('rdi.overview.low')
      expect(setupState.formatSwitch('false')).toBe('rdi.overview.low')
      expect(setupState.formatSwitch('low')).toBe('rdi.overview.low')
    })

    it('returns -- for undefined/null/empty', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.formatSwitch(undefined)).toBe('--')
      expect(setupState.formatSwitch(null)).toBe('--')
      expect(setupState.formatSwitch('')).toBe('--')
    })

    it('returns string value for other inputs', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.formatSwitch('custom')).toBe('custom')
    })
  })

  describe('fetchDevices', () => {
    it('populates stats from sumData response', async () => {
      hoisted.sumData.mockResolvedValue({
        data: { device_total: 50, device_on: 40, device_offline: 10 }
      })
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.sumData.mockResolvedValue({
        data: { device_total: 50, device_on: 40, device_offline: 10 }
      })
      await setupState.fetchDevices()
      await flushPromises()

      expect(setupState.stats.totalDevices).toBe(50)
      expect(setupState.stats.onlineDevices).toBe(40)
      expect(setupState.stats.offlineDevices).toBe(10)
    })

    it('handles alternative field names (DeviceTotal, DeviceOn)', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.sumData.mockResolvedValue({
        data: { DeviceTotal: 30, DeviceOn: 25 }
      })
      await setupState.fetchDevices()
      await flushPromises()

      expect(setupState.stats.totalDevices).toBe(30)
      expect(setupState.stats.onlineDevices).toBe(25)
      expect(setupState.stats.offlineDevices).toBe(5)
    })

    it('sets loading to false in finally block', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.sumData.mockResolvedValue({ data: {} })
      await setupState.fetchDevices()
      await flushPromises()

      expect(setupState.loading).toBe(false)
    })
  })

  describe('fetchCounts', () => {
    it('sets alarmDeviceTotal from getAlarmCount response', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.getAlarmCount.mockResolvedValue({ data: { alarm_device_total: 7, alarm_history_total: 12 } })
      await setupState.fetchCounts()
      await flushPromises()

      expect(setupState.alarmDeviceTotal).toBe(7)
      expect(setupState.stats.alarmHistoryTotal).toBe(12)
    })

    it('handles alternative field name AlarmDeviceTotal', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.getAlarmCount.mockResolvedValue({ data: { AlarmDeviceTotal: 3, AlarmHistoryTotal: 8 } })
      await setupState.fetchCounts()
      await flushPromises()

      expect(setupState.alarmDeviceTotal).toBe(3)
      expect(setupState.stats.alarmHistoryTotal).toBe(8)
    })

    it('sets activeAlarms from active_alarm_total when the summary API provides it', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.getAlarmCount.mockResolvedValue({ data: { alarm_device_total: 7, active_alarm_total: 5 } })
      const hasActiveAlarmTotal = await setupState.fetchCounts()
      await flushPromises()

      expect(hasActiveAlarmTotal).toBe(true)
      expect(setupState.alarmDeviceTotal).toBe(7)
      expect(setupState.stats.activeAlarms).toBe(5)
    })

    it('keeps the unfiltered history total independent from the active list pagination total', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.getAlarmCount.mockResolvedValue({
        data: { alarm_device_total: 2, active_alarm_total: 3, alarm_history_total: 42 }
      })
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 3 } })

      await setupState.fetchCounts()
      await setupState.fetchAlarms()
      await flushPromises()

      expect(setupState.stats.activeAlarms).toBe(3)
      expect(setupState.stats.alarmHistoryTotal).toBe(42)
      expect(setupState.alarmPagination.itemCount).toBe(3)
    })
  })

  describe('master account all-tenant scope', () => {
    it('uses global aggregates and explicit all_tenants reads only for SYS_ADMIN', async () => {
      hoisted.authUserInfo.authority = 'SYS_ADMIN'
      hoisted.authUserInfo.roles = ['SYS_ADMIN']
      hoisted.sumData.mockResolvedValue({ data: { device_total: 200, device_on: 150, device_offline: 50 } })
      hoisted.deviceList.mockResolvedValue({
        data: {
          total: 1,
          list: [{ id: 'cross-tenant-device', name: 'Remote System', scope_tenant_id: 'tenant-b', is_online: 1 }]
        }
      })

      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(hoisted.sumData).toHaveBeenCalledWith({ all_tenants: true })
      expect(hoisted.getAlarmCount).toHaveBeenCalledWith({ all_tenants: true })
      expect(hoisted.alarmHistory.mock.calls.length).toBeGreaterThan(0)
      for (const [params] of hoisted.alarmHistory.mock.calls) {
        expect(params).toMatchObject({ all_tenants: true })
      }
      expect(hoisted.alarmHistoryMonthlyTrend).toHaveBeenCalledWith(
        dayjs().year(),
        expect.any(String),
        { all_tenants: true }
      )
      const deviceColumn = setupState.alarmColumns.find((column) => column.key === 'devices')
      const deviceLink = deviceColumn.render({
        tenant_id: 'tenant-b',
        alarm_device_list: [{ id: 'cross-tenant-device', name: 'Remote System' }]
      })
      expect(deviceLink.children.default()).toBe('Remote System · tenant-b')

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(hoisted.deviceList).toHaveBeenCalledWith({
        page: 1,
        page_size: 12,
        include_rdi_system_info_summary: true,
        all_tenants: true
      })
      expect(setupState.deviceSnapshots[0].tenantId).toBe('tenant-b')
    })
  })

  describe('fetchActiveAlarmCounts', () => {
    it('uses the current ACTIVE stream count instead of summing historical severities', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValueOnce({ data: { total: 6 } })

      await setupState.fetchActiveAlarmCounts()
      await flushPromises()

      expect(setupState.stats.activeAlarms).toBe(6)
      expect(hoisted.alarmHistory).toHaveBeenCalledWith({ page: 1, page_size: 1, alarm_status: 'ACTIVE' })
    })
  })

  describe('normalizeAlarmMonthlyTrendPoints', () => {
    it('fills all 12 months and ignores invalid duplicate month rows', () => {
      const points = normalizeAlarmMonthlyTrendPoints([
        { month: 1, count: 2 },
        { month: 3, count: 4 },
        { month: 3, count: 9 },
        { month: 13, count: 99 }
      ])

      expect(points).toHaveLength(12)
      expect(points[0]).toEqual({ month: 1, count: 2 })
      expect(points[1]).toEqual({ month: 2, count: 0 })
      expect(points[2]).toEqual({ month: 3, count: 4 })
      expect(points[11]).toEqual({ month: 12, count: 0 })
    })
  })

  describe('fetchAlarms', () => {
    it('loads active alarms by default with pagination params', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({
        data: {
          list: [{ id: 'a1', name: 'Alarm 1' }],
          total: 1
        }
      })

      await setupState.fetchAlarms()
      await flushPromises()

      expect(hoisted.alarmHistory).toHaveBeenCalledWith({
        page: 1,
        page_size: 10,
        alarm_status: 'ACTIVE'
      })
      expect(setupState.alarms).toHaveLength(1)
      expect(setupState.alarmPagination.itemCount).toBe(1)
    })

    it('omits alarm_status when All is selected', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.queryParams.alarm_status = ''
      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })

      await setupState.fetchAlarms()
      await flushPromises()

      expect(hoisted.alarmHistory).toHaveBeenCalledWith({
        page: 1,
        page_size: 10,
        alarm_status: undefined
      })
    })

    it('passes alarm_status when set', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.queryParams.alarm_status = 'H'
      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })

      await setupState.fetchAlarms()
      await flushPromises()

      expect(hoisted.alarmHistory).toHaveBeenCalledWith({
        page: 1,
        page_size: 10,
        alarm_status: 'H'
      })
    })

    it('sets alarmLoading to false in finally', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })

      await setupState.fetchAlarms()
      await flushPromises()

      expect(setupState.alarmLoading).toBe(false)
    })

    it('resets both pagination sources before searching with a changed alarm status', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.queryParams.page = 4
      setupState.alarmPagination.page = 4
      setupState.queryParams.alarm_status = 'H'
      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })

      setupState.searchAlarms()
      await flushPromises()

      expect(setupState.queryParams.page).toBe(1)
      expect(setupState.alarmPagination.page).toBe(1)
      expect(hoisted.alarmHistory).toHaveBeenCalledWith({
        page: 1,
        page_size: 10,
        alarm_status: 'H'
      })
    })
  })

  describe('fetchAlarmTrend', () => {
    it('loads the selected year monthly trend and builds matching chart options', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      setupState.alarmTrendYear = 2025
      hoisted.alarmHistoryMonthlyTrend.mockResolvedValue({
        data: {
          year: 2025,
          months: [
            { month: 1, count: 3 },
            { month: 12, count: 7 }
          ]
        }
      })

      await setupState.fetchAlarmTrend()
      await flushPromises()

      expect(hoisted.alarmHistoryMonthlyTrend).toHaveBeenCalledWith(2025, expect.any(String))
      expect(setupState.alarmTrendLoading).toBe(false)
      expect(setupState.alarmTrendPoints).toHaveLength(12)
      expect(setupState.alarmTrendChartOptions.xAxis.data).toEqual(
        setupState.alarmTrendPoints.map((point) => String(point.month).padStart(2, '0'))
      )
      expect(setupState.alarmTrendChartOptions.series[0].data).toEqual(
        setupState.alarmTrendPoints.map((point) => point.count)
      )
      expect(setupState.alarmTrendChartOptions.series[0].name).toBe('rdi.overview.alarmTrendSeries')
    })
  })

  describe('fetchDeviceSnapshots', () => {
    it('loads device snapshots with telemetry data', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: {
          total: 25,
          list: [
            { id: 'd1', name: 'Device 1', is_online: 1 },
            { id: 'd2', name: 'Device 2', is_online: 0 }
          ]
        }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({
        data: [{ key: 'temperature_1', value: 25.5 }]
      })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots).toHaveLength(2)
      expect(setupState.deviceSnapshots[0].id).toBe('d1')
      expect(setupState.deviceSnapshots[0].name).toBe('Device 1')
      expect(setupState.deviceSnapshots[0].online).toBe(true)
      expect(setupState.deviceSnapshots[0].alarm).toBe(false)
      expect(setupState.deviceSnapshots[0].serialNumber).toBe('--')
      expect(setupState.deviceSnapshots[0].telemetry.temperature_1).toBe(25.5)
      expect(setupState.snapshotTotal).toBe(25)
      expect(hoisted.deviceList).toHaveBeenCalledWith({
        page: 1,
        page_size: 12,
        include_rdi_system_info_summary: true
      })
      expect(hoisted.deviceAlarmStatus).toHaveBeenCalledWith({ device_id: 'd1' })
      expect(hoisted.deviceAlarmStatus).toHaveBeenCalledWith({ device_id: 'd2' })
    })

    it('marks a system card as alarmed when the device alarm endpoint reports an active alarm', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { total: 1, list: [{ id: 'alarm-device', name: 'Alarm Device', is_online: 1 }] }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })
      hoisted.deviceAlarmStatus.mockResolvedValue({ data: { alarm: true } })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots[0].alarm).toBe(true)
      expect(setupState.snapshotStatusLabel(setupState.deviceSnapshots[0])).toBe('custom.devicePage.alarmed')
      expect(setupState.snapshotStatusTagType(setupState.deviceSnapshots[0])).toBe('error')
    })

    it('does not label an online system normal when alarm status cannot be loaded', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { total: 1, list: [{ id: 'unknown-device', name: 'Unknown Device', is_online: 1 }] }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })
      hoisted.deviceAlarmStatus.mockRejectedValue(new Error('alarm unavailable'))

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots[0].alarm).toBeNull()
      expect(setupState.snapshotStatusLabel(setupState.deviceSnapshots[0])).toBe('rdi.overview.online')
      expect(setupState.snapshotStatusTagType(setupState.deviceSnapshots[0])).toBe('info')
    })

    it('loads another page so all systems remain reachable from the overview', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({ data: { total: 25, list: [{ id: 'd13', name: 'Device 13' }] } })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })

      setupState.changeSnapshotPage(2)
      await flushPromises()

      expect(setupState.snapshotPage).toBe(2)
      expect(hoisted.deviceList).toHaveBeenCalledWith({
        page: 2,
        page_size: 12,
        include_rdi_system_info_summary: true
      })
      expect(setupState.deviceSnapshots[0].id).toBe('d13')
    })

    it('maps customer installation and technician fields into snapshots', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: {
          list: [
            {
              id: 'dev-1',
              name: 'Sensor1',
              rdi_serial_number: 'RDI-001',
              installation_location: 'Plant A',
              installation_address: '101 Main St',
              installation_date: '2026-07-01',
              service_technician: 'Alex',
              technician_email: 'alex@example.com',
              admin_name: 'NEMAS'
            }
          ]
        }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots[0]).toMatchObject({
        serialNumber: 'RDI-001',
        installLocation: 'Plant A',
        installAddress: '101 Main St',
        installDate: '2026-07-01',
        installerName: 'Alex',
        installerContact: 'alex@example.com',
        adminName: 'NEMAS'
      })
      expect(setupState.hasInstallationInfo(setupState.deviceSnapshots[0])).toBe(true)
    })

    it('hydrates RDI installation details when the list has no usable system info summary', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { list: [{ id: 'dev-rdi', name: 'RDI System', pid_number: 'PID-001', system_info: {} }] }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })
      hoisted.deviceAlarmStatus.mockResolvedValue({ data: { alarm: false } })
      hoisted.rdiDeviceConfig.mockResolvedValue({
        error: null,
        data: {
          system_info: {
            installation_location: 'Plant B',
            address: '22 Industrial Road',
            installation_date: '2026-07-18',
            installer_name: 'Morgan',
            installer_phone: '+1 555 0100',
            installer_email: 'morgan@example.com',
            controller_serial_number: 'RDI-SN-018',
            customer_name: 'Acme Facilities'
          }
        }
      })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(hoisted.rdiDeviceConfig).toHaveBeenCalledWith('dev-rdi')
      expect(setupState.deviceSnapshots[0]).toMatchObject({
        serialNumber: 'RDI-SN-018',
        installLocation: 'Plant B',
        installAddress: '22 Industrial Road',
        installDate: '2026-07-18',
        installerName: 'Morgan',
        installerContact: '+1 555 0100 · morgan@example.com',
        // REQ-06b: administrator falls back to system_info.customer_name so the
        // alarm detail matches the device detail's "用户管理员" field.
        adminName: 'Acme Facilities'
      })
    })

    it('uses a list system info summary without issuing a duplicate config request', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: {
          list: [
            {
              id: 'dev-summary',
              name: 'Summary System',
              pid_number: 'PID-002',
              rdi_system_info_summary: { controller_serial_number: 'SUMMARY-SN' }
            }
          ]
        }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })
      hoisted.deviceAlarmStatus.mockResolvedValue({ data: { alarm: false } })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(hoisted.rdiDeviceConfig).not.toHaveBeenCalled()
      expect(setupState.deviceSnapshots[0].serialNumber).toBe('SUMMARY-SN')
    })

    it('treats an explicitly returned empty list summary as authoritative', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: {
          list: [
            {
              id: 'dev-empty-summary',
              name: 'Empty Summary System',
              pid_number: 'PID-003',
              rdi_system_info_summary: {}
            }
          ]
        }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })
      hoisted.deviceAlarmStatus.mockResolvedValue({ data: { alarm: false } })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(hoisted.rdiDeviceConfig).not.toHaveBeenCalled()
      expect(setupState.deviceSnapshots[0].serialNumber).toBe('--')
    })

    it('filters out items without id', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { list: [{ id: 'd1', name: 'Device 1' }, { name: 'No ID' }] }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots).toHaveLength(1)
    })

    it('handles telemetry fetch errors gracefully', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { list: [{ id: 'd1', name: 'Device 1' }] }
      })
      hoisted.telemetryDataCurrentKeys.mockRejectedValue(new Error('network'))

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots).toHaveLength(1)
      expect(setupState.deviceSnapshots[0].telemetry).toEqual({})
    })

    it('sets snapshotLoading to false in finally', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({ data: { list: [] } })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.snapshotLoading).toBe(false)
    })
  })

  describe('acknowledgeAlarm', () => {
    it('calls acknowledgeAlarmHistory and refreshes alarms', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.acknowledgeAlarmHistory.mockResolvedValue({ error: null })
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })

      await setupState.acknowledgeAlarm({ id: 'alarm-1' })
      await flushPromises()

      expect(hoisted.acknowledgeAlarmHistory).toHaveBeenCalledWith('alarm-1')
      expect(hoisted.messageSuccess).toHaveBeenCalledWith('rdi.overview.alarmAcknowledged')
      expect(hoisted.alarmHistory).toHaveBeenCalledTimes(1)
      expect(hoisted.alarmHistory).toHaveBeenCalledWith({
        page: 1,
        page_size: 10,
        alarm_status: 'ACTIVE'
      })
    })
  })

  describe('resetAlarm', () => {
    it('opens warning dialog with alarm info', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      setupState.resetAlarm({ id: 'alarm-1', name: 'Test Alarm' })

      expect(hoisted.dialogWarning).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'rdi.overview.resetAlarm',
          content: 'Test Alarm',
          positiveText: 'common.reset',
          negativeText: 'common.cancel'
        })
      )
    })

    it('onPositiveClick calls resetAlarmHistory and refreshes', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.resetAlarmHistory.mockResolvedValue({ error: null })
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })
      hoisted.getAlarmCount.mockResolvedValue({ data: { alarm_device_total: 0 } })

      setupState.resetAlarm({ id: 'alarm-1', name: 'Test' })
      const dialogCall = hoisted.dialogWarning.mock.calls[0][0]
      await dialogCall.onPositiveClick()
      await flushPromises()

      expect(hoisted.resetAlarmHistory).toHaveBeenCalledWith('alarm-1')
      expect(hoisted.messageSuccess).toHaveBeenCalledWith('rdi.overview.alarmReset')
    })
  })

  describe('resetAlarmFilter', () => {
    it('restores the Active filter and calls fetchAlarms', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.queryParams.alarm_status = 'H'
      setupState.queryParams.page = 3
      setupState.alarmPagination.page = 3

      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })

      setupState.resetAlarmFilter()
      await flushPromises()

      expect(setupState.queryParams.alarm_status).toBe('ACTIVE')
      expect(setupState.queryParams.page).toBe(1)
      expect(setupState.alarmPagination.page).toBe(1)
      expect(hoisted.alarmHistory).toHaveBeenCalledTimes(1)
      expect(hoisted.alarmHistory).toHaveBeenCalledWith({
        page: 1,
        page_size: 10,
        alarm_status: 'ACTIVE'
      })
    })
  })

  describe('refreshAll', () => {
    it('calls immediate fetch functions and schedules device snapshots', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.sumData.mockResolvedValue({ data: {} })
      hoisted.getAlarmCount.mockResolvedValue({ data: {} })
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })
      hoisted.deviceList.mockResolvedValue({ data: { list: [] } })

      await setupState.refreshAll()
      await flushPromises()

      expect(hoisted.sumData).toHaveBeenCalledTimes(1)
      expect(hoisted.getAlarmCount).toHaveBeenCalledTimes(1)
      // No active_alarm_total in the summary, so refreshAlarmSummaryCounts falls
      // back to one active-count alarmHistory query; fetchAlarms adds the second.
      expect(hoisted.alarmHistory).toHaveBeenCalledTimes(2)
      expect(hoisted.deviceList).toHaveBeenCalledTimes(0)
      expect(setupState.snapshotLoading).toBe(true)
    })

    it('uses summary active_alarm_total instead of three extra alarm count requests', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.sumData.mockResolvedValue({ data: {} })
      hoisted.getAlarmCount.mockResolvedValue({ data: { alarm_device_total: 2, active_alarm_total: 9 } })
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })
      hoisted.deviceList.mockResolvedValue({ data: { list: [] } })

      await setupState.refreshAll()
      await flushPromises()

      expect(setupState.stats.activeAlarms).toBe(9)
      expect(hoisted.getAlarmCount).toHaveBeenCalledTimes(1)
      // active_alarm_total present, so the active-count fallback query is skipped
      // and only fetchAlarms issues an alarmHistory request.
      expect(hoisted.alarmHistory).toHaveBeenCalledTimes(1)
    })

    it('still schedules device snapshots when the independent trend request fails', async () => {
      const wrapper = mountComponent({ activeSystemsOnly: true })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.sumData.mockResolvedValue({ data: {} })
      hoisted.getAlarmCount.mockResolvedValue({ data: { active_alarm_total: 0 } })
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })
      hoisted.alarmHistoryMonthlyTrend.mockRejectedValueOnce(new Error('timezone unavailable'))
      hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 } })

      await expect(setupState.refreshAll()).resolves.toBeUndefined()
      await flushPromises()

      expect(setupState.snapshotLoading).toBe(true)
    })
  })

  describe('pagination', () => {
    it('onChange updates page and calls fetchAlarms', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })

      setupState.alarmPagination.onChange(5)
      await flushPromises()

      expect(setupState.queryParams.page).toBe(5)
      expect(setupState.alarmPagination.page).toBe(5)
      expect(hoisted.alarmHistory).toHaveBeenCalledTimes(1)
      expect(hoisted.alarmHistory).toHaveBeenCalledWith({
        page: 5,
        page_size: 10,
        alarm_status: 'ACTIVE'
      })
    })

    it('onUpdatePageSize updates pageSize, resets page, and calls fetchAlarms', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.queryParams.page = 3
      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })

      setupState.alarmPagination.onUpdatePageSize(20)
      await flushPromises()

      expect(setupState.queryParams.page_size).toBe(20)
      expect(setupState.queryParams.page).toBe(1)
      expect(setupState.alarmPagination.pageSize).toBe(20)
      expect(setupState.alarmPagination.page).toBe(1)
      expect(hoisted.alarmHistory).toHaveBeenCalledTimes(1)
      expect(hoisted.alarmHistory).toHaveBeenCalledWith({
        page: 1,
        page_size: 20,
        alarm_status: 'ACTIVE'
      })
    })
  })

  describe('alarmStatusOptions', () => {
    it('contains all status options', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const options = setupState.alarmStatusOptions
      expect(options).toHaveLength(6)
      expect(options[0]).toEqual({ label: 'rdi.overview.active', value: 'ACTIVE' })
      const values = options.map((o) => o.value)
      expect(values).toEqual(['ACTIVE', '', 'H', 'M', 'L', 'N'])
    })
  })

  describe('onMounted', () => {
    it('calls immediate refresh work on mount and schedules device snapshots', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(hoisted.sumData).toHaveBeenCalledTimes(1)
      expect(hoisted.getAlarmCount).toHaveBeenCalledTimes(1)
      // The summary endpoint has no active_alarm_total in this fixture, so
      // refreshAll performs one fallback active-count query plus fetchAlarms.
      expect(hoisted.alarmHistory).toHaveBeenCalledTimes(2)
      expect(hoisted.deviceList).toHaveBeenCalledTimes(0)
      expect(setupState.snapshotLoading).toBe(true)
    })
  })

  describe('alarmColumns', () => {
    it('has 7 columns', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.alarmColumns).toHaveLength(7)
    })

    it('create_at column render formats time', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'create_at')
      expect(col.render({ create_at: '2024-01-15T10:30:00Z' })).toBe(
        dayjs('2024-01-15T10:30:00Z').format('YYYY-MM-DD HH:mm:ss')
      )
      expect(col.render({})).toBe('-')
    })

    it('name column render returns name or content or dash', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'name')
      expect(col.render({ name: 'Test' })).toBe('Test')
      expect(col.render({ content: 'Content' })).toBe('Content')
      expect(col.render({})).toBe('-')
    })

    it('description column render returns description or dash', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'description')
      expect(col.render({ description: 'desc' })).toBe('desc')
      expect(col.render({})).toBe('-')
    })
  })

  describe('alarmColumns render functions - full coverage', () => {
    it('create_at column title is callable', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'create_at')
      expect(col.title()).toBe('common.time')
    })

    it('name column title is callable', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'name')
      expect(col.title()).toBe('rdi.overview.alarm')
    })

    it('alarm_status column title and render', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'alarm_status')
      expect(col.title()).toBe('common.alarm_level')
      const high = col.render({ alarm_status: 'H' })
      const medium = col.render({ alarm_status: 'M' })
      const low = col.render({ alarm_status: 'L' })
      const normal = col.render({ alarm_status: 'N' })
      expect(high.props.type).toBe('error')
      expect(high.children.default()).toBe('rdi.overview.high')
      expect(medium.props.type).toBe('warning')
      expect(medium.children.default()).toBe('rdi.overview.medium')
      expect(low.props.type).toBe('info')
      expect(low.children.default()).toBe('rdi.overview.low')
      expect(normal.props.type).toBe('success')
      expect(normal.children.default()).toBe('rdi.overview.normal')
    })

    it('alarm_type column title and render', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'alarm_type')
      expect(col.title()).toBe('rdi.overview.alarmType')
      expect(col.render({ remark: '{"event_type":"temperature_alarm"}' })).toBe('rdi.overview.temperatureAlarm')
      expect(col.render({ remark: '{"event_type":"switch_alarm"}' })).toBe('rdi.overview.switchAlarm')
      expect(col.render({ remark: '{"event_type":"warranty_alarm"}' })).toBe('rdi.overview.warrantyAlarm')
      expect(col.render({ remark: '{"event_type":"pressure_alarm"}' })).toBe('rdi.overview.pressureAlarm')
      expect(col.render({ remark: '{"event_type":"sw2_long_press"}' })).toBe('rdi.overview.sw2LongPress')
      expect(col.render({ remark: '{"event_type":"sw3_short_press"}' })).toBe('rdi.overview.sw3ShortPress')
      expect(col.render({ remark: '{"event_type":"sw3_long_press"}' })).toBe('rdi.overview.sw3LongPress')
      expect(col.render({ remark: '{"event_type":"PT"}' })).toBe('rdi.overview.pressureAlarm')
    })

    it('devices column title and render with device list', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'devices')
      expect(col.title()).toBe('rdi.overview.device')
      const vnode = col.render({ alarm_device_list: [{ id: 'dev-1', name: 'Device 1' }] })
      expect(vnode.props.text).toBe(true)
      expect(vnode.props.type).toBe('primary')
      expect(vnode.children.default()).toBe('Device 1')
      vnode.props.onClick()
      expect(hoisted.routerPush).toHaveBeenCalledWith({
        name: 'device_details',
        query: { d_id: 'dev-1', tab: 'message' }
      })
    })

    it('devices column render returns dash without device list', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'devices')
      expect(col.render({ alarm_device_list: [] })).toBe('-')
      expect(col.render({})).toBe('-')
    })

    it('devices column render uses id when name is missing', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'devices')
      const vnode = col.render({ alarm_device_list: [{ id: 'dev-1' }] })
      expect(vnode.children.default()).toBe('dev-1')
      expect(vnode.props.type).toBe('primary')
    })

    it('actions column title and render', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'actions')
      expect(col.title()).toBe('common.actions')
      const vnode = col.render({ id: 'a1', alarm_status: 'H', remark: '{}' })
      expect(vnode.props.class).toBe('action-row')
      expect(vnode.children).toHaveLength(2)
      expect(vnode.children[0].props).toMatchObject({ size: 'small', type: 'success', disabled: false })
      expect(vnode.children[0].children.default()).toBe('rdi.overview.acknowledgeAlarm')
      expect(vnode.children[1].props).toMatchObject({ size: 'small', type: 'error', disabled: false })
      expect(vnode.children[1].children.default()).toBe('rdi.overview.resetAlarm')
    })

    it('actions column acknowledge button triggers acknowledgeAlarm', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.acknowledgeAlarmHistory.mockResolvedValue({ error: null })
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })

      const col = setupState.alarmColumns.find((c) => c.key === 'actions')
      const vnode = col.render({ id: 'a1', alarm_status: 'H', remark: '{}' })
      const ackBtn = vnode.children[0]
      await ackBtn.props.onClick()
      await flushPromises()

      expect(hoisted.acknowledgeAlarmHistory).toHaveBeenCalledWith('a1')
    })

    it('actions column reset button triggers resetAlarm', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()

      const col = setupState.alarmColumns.find((c) => c.key === 'actions')
      const vnode = col.render({ id: 'a1', alarm_status: 'H', remark: '{}' })
      const resetBtn = vnode.children[1]
      resetBtn.props.onClick()

      expect(hoisted.dialogWarning).toHaveBeenCalledTimes(1)
    })

    it('actions column acknowledge button is disabled when acknowledged', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'actions')
      const vnode = col.render({ id: 'a1', alarm_status: 'H', remark: { acknowledged: true } })
      const ackBtn = vnode.children[0]
      expect(ackBtn.props.disabled).toBe(true)
    })

    it('actions column reset button is disabled when alarm_status is N', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'actions')
      const vnode = col.render({ id: 'a1', alarm_status: 'N', remark: '{}' })
      const resetBtn = vnode.children[1]
      expect(resetBtn.props.disabled).toBe(true)
    })

    it('description column title is callable', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'description')
      expect(col.title()).toBe('rdi.overview.description')
    })
  })

  describe('fetchDevices - DeviceOffline fallback', () => {
    it('calculates offline from total - online when device_offline and DeviceOffline are absent', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.sumData.mockResolvedValue({
        data: { device_total: 100, device_on: 60 }
      })
      await setupState.fetchDevices()
      await flushPromises()

      expect(setupState.stats.totalDevices).toBe(100)
      expect(setupState.stats.onlineDevices).toBe(60)
      expect(setupState.stats.offlineDevices).toBe(40)
      expect(setupState.loading).toBe(false)
    })

    it('uses 0 as offline fallback when total equals online', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.sumData.mockResolvedValue({
        data: { DeviceTotal: 50, DeviceOn: 50 }
      })
      await setupState.fetchDevices()
      await flushPromises()

      expect(setupState.stats.offlineDevices).toBe(0)
      expect(setupState.loading).toBe(false)
    })
  })

  describe('fetchCounts - missing fields', () => {
    it('handles missing alarm_device_total and AlarmDeviceTotal', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.getAlarmCount.mockResolvedValue({ data: {} })
      await setupState.fetchCounts()
      await flushPromises()

      expect(setupState.alarmDeviceTotal).toBe(0)
    })
  })

  describe('fetchActiveAlarmCounts - edge cases', () => {
    it('handles responses without total field', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({ data: {} })

      await setupState.fetchActiveAlarmCounts()
      await flushPromises()

      expect(setupState.stats.activeAlarms).toBe(0)
    })

    it('handles null data in response', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({})

      await setupState.fetchActiveAlarmCounts()
      await flushPromises()

      expect(setupState.stats.activeAlarms).toBe(0)
    })
  })

  describe('fetchDeviceSnapshots - alternative fields', () => {
    it('uses alternative field names (device_id, device_name, device_number, firmware_version)', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: {
          list: [
            { device_id: 'd1', device_name: 'Dev1', device_number: 'P001', firmware_version: 'v1.0', is_online: 1 }
          ]
        }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({
        data: [{ key: 'temperature_1', value: 25 }]
      })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots).toHaveLength(1)
      expect(setupState.deviceSnapshots[0].id).toBe('d1')
      expect(setupState.deviceSnapshots[0].name).toBe('Dev1')
      expect(setupState.deviceSnapshots[0].pid).toBe('P001')
      expect(setupState.deviceSnapshots[0].firmware).toBe('v1.0')
      expect(setupState.deviceSnapshots[0].online).toBe(true)
      expect(setupState.snapshotLoading).toBe(false)
    })

    it('skips telemetry fetch for devices with empty id', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { list: [{ id: '', name: 'No ID' }] }
      })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots).toHaveLength(0)
      expect(hoisted.telemetryDataCurrentKeys).toHaveBeenCalledTimes(0)
    })

    it('uses CurrentVersion and DeviceNumber as fallback field names', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: {
          list: [{ id: 'd1', name: 'Dev1', DeviceNumber: 'P002', CurrentVersion: 'v2.0', online: 'online' }]
        }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots[0].pid).toBe('P002')
      expect(setupState.deviceSnapshots[0].firmware).toBe('v2.0')
      expect(setupState.deviceSnapshots[0].online).toBe(true)
    })

    it('handles telemetry response as object instead of list', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { list: [{ id: 'd1', name: 'Dev1' }] }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({
        data: { temperature_1: 30, switch_1: true }
      })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots[0].telemetry).toEqual({ temperature_1: 30, switch_1: true })
    })
  })

  describe('normalizeTelemetry - additional cases', () => {
    it('handles string_v and bool_v value fields', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const payload = {
        data: [
          { key: 'switch', string_v: 'on' },
          { key: 'contact', bool_v: true }
        ]
      }
      expect(setupState.normalizeTelemetry(payload)).toEqual({ switch: 'on', contact: true })
    })

    it('handles item without key', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const payload = {
        data: [{ key: 'temp', value: 25 }, { value: 99 }]
      }
      expect(setupState.normalizeTelemetry(payload)).toEqual({ temp: 25 })
    })
  })

  describe('resetAlarm - onPositiveClick full chain', () => {
    it('calls fetchAlarms, fetchCounts, and fetchActiveAlarmCounts after reset', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.resetAlarmHistory.mockResolvedValue({ error: null })
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })
      hoisted.getAlarmCount.mockResolvedValue({ data: { alarm_device_total: 0 } })

      setupState.resetAlarm({ id: 'alarm-1', name: 'Test' })
      const dialogCall = hoisted.dialogWarning.mock.calls[0][0]
      await dialogCall.onPositiveClick()
      await flushPromises()

      expect(hoisted.resetAlarmHistory).toHaveBeenCalledWith('alarm-1')
      expect(hoisted.messageSuccess).toHaveBeenCalledWith('rdi.overview.alarmReset')
      expect(hoisted.alarmHistory).toHaveBeenCalledTimes(2)
      expect(hoisted.getAlarmCount).toHaveBeenCalledTimes(1)
    })

    it('uses content as dialog content when name is missing', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      setupState.resetAlarm({ id: 'alarm-1', content: 'Alarm Content' })

      expect(hoisted.dialogWarning).toHaveBeenCalledWith(
        expect.objectContaining({
          content: 'Alarm Content'
        })
      )
    })

    it('uses id as dialog content when name and content are missing', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      setupState.resetAlarm({ id: 'alarm-1' })

      expect(hoisted.dialogWarning).toHaveBeenCalledWith(
        expect.objectContaining({
          content: 'alarm-1'
        })
      )
    })
  })

  describe('resetAlarm - onPositiveClick calls fetchActiveAlarmCounts', () => {
    it('calls fetchActiveAlarmCounts after resetAlarm positive click', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.resetAlarmHistory.mockResolvedValue({ error: null })
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })
      hoisted.getAlarmCount.mockResolvedValue({ data: { alarm_device_total: 0 } })
      hoisted.sumData.mockResolvedValue({ data: {} })

      setupState.resetAlarm({ id: 'alarm-1', name: 'Test' })
      const dialogCall = hoisted.dialogWarning.mock.calls[0][0]
      await dialogCall.onPositiveClick()
      await flushPromises()

      // One current ACTIVE count request plus the refreshed alarm list.
      expect(hoisted.alarmHistory).toHaveBeenCalledTimes(2)
      expect(hoisted.getAlarmCount).toHaveBeenCalledTimes(1)
    })

    it('resets and schedules the Active Systems page after a successful reset', async () => {
      const wrapper = mountComponent({ activeSystemsOnly: true })
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.changeSnapshotPage(2)
      await flushPromises()
      vi.clearAllMocks()
      hoisted.resetAlarmHistory.mockResolvedValue({ error: null })
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })
      hoisted.getAlarmCount.mockResolvedValue({ data: { alarm_device_total: 0 } })

      setupState.resetAlarm({ id: 'alarm-1', name: 'Test' })
      const dialogCall = hoisted.dialogWarning.mock.calls[0][0]
      await dialogCall.onPositiveClick()
      await flushPromises()

      expect(setupState.snapshotPage).toBe(1)
      expect(setupState.snapshotLoading).toBe(true)
    })
  })

  describe('fetchDevices - full coverage', () => {
    it('covers stats assignment and finally block with empty data', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.sumData.mockResolvedValue({ data: null })
      await setupState.fetchDevices()
      await flushPromises()

      expect(setupState.stats.totalDevices).toBe(0)
      expect(setupState.stats.onlineDevices).toBe(0)
      expect(setupState.stats.offlineDevices).toBe(0)
      expect(setupState.loading).toBe(false)
    })

    it('covers DeviceTotal/DeviceOn/DeviceOffline field names', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.sumData.mockResolvedValue({
        data: { DeviceTotal: 200, DeviceOn: 150, DeviceOffline: 50 }
      })
      await setupState.fetchDevices()
      await flushPromises()

      expect(setupState.stats.totalDevices).toBe(200)
      expect(setupState.stats.onlineDevices).toBe(150)
      expect(setupState.stats.offlineDevices).toBe(50)
      expect(setupState.loading).toBe(false)
    })
  })

  describe('fetchCounts - full coverage', () => {
    it('covers alarmDeviceTotal assignment with null data', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.getAlarmCount.mockResolvedValue({ data: null })
      await setupState.fetchCounts()
      await flushPromises()

      expect(setupState.alarmDeviceTotal).toBe(0)
    })
  })

  describe('fetchActiveAlarmCounts - full coverage', () => {
    it('uses the ACTIVE alias total', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValueOnce({ data: { total: 10 } })

      await setupState.fetchActiveAlarmCounts()
      await flushPromises()

      expect(setupState.stats.activeAlarms).toBe(10)
      expect(hoisted.alarmHistory).toHaveBeenCalledTimes(1)
      expect(hoisted.alarmHistory).toHaveBeenCalledWith({ page: 1, page_size: 1, alarm_status: 'ACTIVE' })
    })
  })

  describe('fetchDeviceSnapshots - full coverage', () => {
    it('covers device with id fetching telemetry successfully', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: {
          list: [{ id: 'dev-1', name: 'Sensor1', pid_number: 'P100', firmware_version: 'v3.1', is_online: 1 }]
        }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({
        data: [
          { key: 'temperature_1', value: 22.5 },
          { key: 'switch_1', value: true }
        ]
      })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots).toHaveLength(1)
      expect(setupState.deviceSnapshots[0].id).toBe('dev-1')
      expect(setupState.deviceSnapshots[0].name).toBe('Sensor1')
      expect(setupState.deviceSnapshots[0].pid).toBe('P100')
      expect(setupState.deviceSnapshots[0].firmware).toBe('v3.1')
      expect(setupState.deviceSnapshots[0].online).toBe(true)
      expect(setupState.deviceSnapshots[0].telemetry.temperature_1).toBe(22.5)
      expect(setupState.snapshotLoading).toBe(false)
    })

    it('covers device with empty id skipping telemetry', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { list: [{ id: '', name: 'Empty' }] }
      })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots).toHaveLength(0)
      expect(hoisted.telemetryDataCurrentKeys).toHaveBeenCalledTimes(0)
      expect(setupState.snapshotLoading).toBe(false)
    })

    it('covers telemetry fetch error within device loop', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { list: [{ id: 'd1', name: 'Dev1' }] }
      })
      hoisted.telemetryDataCurrentKeys.mockRejectedValue(new Error('timeout'))

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots).toHaveLength(1)
      expect(setupState.deviceSnapshots[0].telemetry).toEqual({})
      expect(setupState.snapshotLoading).toBe(false)
    })

    it('covers normalizeDeviceRows with direct array payload', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue([{ id: 'd1', name: 'Dev1', is_online: 1 }])
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots).toHaveLength(1)
      expect(setupState.snapshotLoading).toBe(false)
    })

    it('covers device name fallback to id when name is empty', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { list: [{ id: 'd1', is_online: 0 }] }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots).toHaveLength(1)
      expect(setupState.deviceSnapshots[0].name).toBe('d1')
      expect(setupState.deviceSnapshots[0].online).toBe(false)
    })

    it('covers device with DeviceName and device_id field names', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: {
          list: [
            {
              device_id: 'd2',
              DeviceName: 'MyDevice',
              device_number: 'PN002',
              current_version: 'v5.0',
              online: 'online'
            }
          ]
        }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({ data: [] })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots).toHaveLength(1)
      expect(setupState.deviceSnapshots[0].id).toBe('d2')
      expect(setupState.deviceSnapshots[0].name).toBe('MyDevice')
      expect(setupState.deviceSnapshots[0].pid).toBe('PN002')
      expect(setupState.deviceSnapshots[0].firmware).toBe('v5.0')
      expect(setupState.deviceSnapshots[0].online).toBe(true)
    })

    it('covers normalizeTelemetry with data.list format', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { list: [{ id: 'd1', name: 'Dev1' }] }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({
        data: { list: [{ key: 'temperature_1', value: 30 }] }
      })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots[0].telemetry).toEqual({ temperature_1: 30 })
    })

    it('covers normalizeTelemetry with object data response', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.deviceList.mockResolvedValue({
        data: { list: [{ id: 'd1', name: 'Dev1' }] }
      })
      hoisted.telemetryDataCurrentKeys.mockResolvedValue({
        data: { temperature_1: 30, switch_1: 1 }
      })

      await setupState.fetchDeviceSnapshots()
      await flushPromises()

      expect(setupState.deviceSnapshots[0].telemetry).toEqual({ temperature_1: 30, switch_1: 1 })
    })
  })

  describe('alarmColumns - name column render', () => {
    it('returns content when name is missing', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'name')
      expect(col.render({ content: 'AlarmContent' })).toBe('AlarmContent')
    })
  })

  describe('alarmColumns - actions column reset button render', () => {
    it('renders reset button with correct props for non-N status', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const col = setupState.alarmColumns.find((c) => c.key === 'actions')
      const vnode = col.render({ id: 'a1', alarm_status: 'H', remark: '{}' })
      const resetBtn = vnode.children[1]
      expect(resetBtn.props.disabled).toBe(false)
      expect(resetBtn.props.type).toBe('error')
    })
  })

  describe('pagination - full coverage', () => {
    it('onChange sets queryParams.page and alarmPagination.page', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })

      setupState.alarmPagination.onChange(3)
      await flushPromises()

      expect(setupState.queryParams.page).toBe(3)
      expect(setupState.alarmPagination.page).toBe(3)
    })

    it('onUpdatePageSize sets pageSize and resets page to 1', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.queryParams.page = 5
      vi.clearAllMocks()
      hoisted.alarmHistory.mockResolvedValue({ data: { list: [], total: 0 } })

      setupState.alarmPagination.onUpdatePageSize(50)
      await flushPromises()

      expect(setupState.queryParams.page_size).toBe(50)
      expect(setupState.queryParams.page).toBe(1)
      expect(setupState.alarmPagination.pageSize).toBe(50)
      expect(setupState.alarmPagination.page).toBe(1)
    })
  })
})
