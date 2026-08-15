/**
 * 文件用途: device-status 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceStatusHistory: vi.fn(),
  startLoading: vi.fn(),
  endLoading: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceStatusHistory: hoisted.deviceStatusHistory
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@aetherlink/hooks', () => ({
  useLoading: () => ({
    loading: ref(false),
    startLoading: hoisted.startLoading,
    endLoading: hoisted.endLoading
  })
}))

import DeviceStatus from '../device-status.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountDeviceStatus = (props: Record<string, any> = {}) => {
  const wrapper = shallowMount(DeviceStatus, {
    props: {
      deviceId: 'device-1',
      visible: true,
      ...props
    },
    global: {
      stubs: {
        NModal: true,
        NCard: true,
        NForm: true,
        NFormItem: true,
        NFlex: true,
        NDatePicker: true,
        NSelect: true,
        NButton: true,
        NDataTable: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device-status.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceStatusHistory.mockResolvedValue({
      data: { list: [{ status: 1, change_time: 1719000000 }], total: 1 },
      error: null
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('triggers immediate first load via visible watcher on mount', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceStatusHistory).toHaveBeenCalledWith(
      expect.objectContaining({
        device_id: 'device-1',
        page: 1,
        page_size: 20
      })
    )
  })

  it('does not fetch when visible is false on mount', async () => {
    const wrapper = mountDeviceStatus({ visible: false, deviceId: 'device-1' })
    await flushPromises()

    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(0)
  })

  it('re-fetches when deviceId changes while visible', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    await wrapper.setProps({ deviceId: 'device-2' })
    await flushPromises()

    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceStatusHistory).toHaveBeenCalledWith(
      expect.objectContaining({ device_id: 'device-2' })
    )
  })

  it('does not re-fetch when deviceId changes but visible is false', async () => {
    const wrapper = mountDeviceStatus({ visible: false, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    await wrapper.setProps({ deviceId: 'device-2' })
    await flushPromises()

    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(0)
  })

  it('constructs query params with start_time, end_time, and status', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = 1000
    setupState.queryParams.end_time = 2000
    setupState.queryParams.status = 1

    await setupState.fetchData()
    await flushPromises()

    expect(hoisted.deviceStatusHistory).toHaveBeenCalledWith(
      expect.objectContaining({
        device_id: 'device-1',
        page: 1,
        page_size: 20,
        start_time: 1000,
        end_time: 2000,
        status: 1
      })
    )
  })

  it('omits start_time, end_time, and status when not set', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = undefined
    setupState.queryParams.end_time = undefined
    setupState.queryParams.status = null

    await setupState.fetchData()
    await flushPromises()

    const callArgs = hoisted.deviceStatusHistory.mock.calls[0][0]
    expect(callArgs).not.toHaveProperty('start_time')
    expect(callArgs).not.toHaveProperty('end_time')
    expect(callArgs).not.toHaveProperty('status')
  })

  it('search button triggers query with page reset to 1', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.page = 5
    setupState.pagination.page = 5

    setupState.handleSearch()
    await flushPromises()

    expect(setupState.queryParams.page).toBe(1)
    expect(setupState.pagination.page).toBe(1)
    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(1)
  })

  it('reset button clears query conditions and re-fetches', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = 1000
    setupState.queryParams.end_time = 2000
    setupState.queryParams.status = 1
    setupState.dateRangeValue = [1000000, 2000000]
    setupState.queryParams.page = 5

    setupState.handleReset()
    await flushPromises()

    expect(setupState.queryParams.start_time).toBeUndefined()
    expect(setupState.queryParams.end_time).toBeUndefined()
    expect(setupState.queryParams.status).toBeNull()
    expect(setupState.dateRangeValue).toBeNull()
    expect(setupState.queryParams.page).toBe(1)
    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(1)
  })

  it('date range converts to timestamp in seconds', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.handleDateRangeChange([1000000, 2000000])

    expect(setupState.queryParams.start_time).toBe(1000)
    expect(setupState.queryParams.end_time).toBe(2000)
    expect(setupState.dateRangeValue).toEqual([1000000, 2000000])
  })

  it('clears timestamps when date range is null', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = 1000
    setupState.queryParams.end_time = 2000

    setupState.handleDateRangeChange(null)

    expect(setupState.queryParams.start_time).toBeUndefined()
    expect(setupState.queryParams.end_time).toBeUndefined()
    expect(setupState.dateRangeValue).toBeNull()
  })

  it('renders online status text for status=1', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const statusColumn = setupState.columns.find((c: any) => c.key === 'status')
    const vnode = statusColumn.render({ status: 1 })

    expect(vnode.children).toBe('custom.device_details.online')
  })

  it('renders offline status text for status=0', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const statusColumn = setupState.columns.find((c: any) => c.key === 'status')
    const vnode = statusColumn.render({ status: 0 })

    expect(vnode.children).toBe('custom.device_details.offline')
  })

  it('ends loading even when API rejects with error', async () => {
    hoisted.deviceStatusHistory.mockRejectedValue(new Error('network error'))

    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    expect(hoisted.startLoading).toHaveBeenCalledTimes(1)
    expect(hoisted.endLoading).toHaveBeenCalledTimes(1)
  })

  it('ends loading even when API returns error field', async () => {
    hoisted.deviceStatusHistory.mockResolvedValue({
      data: null,
      error: { message: 'server error' }
    })

    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    expect(hoisted.startLoading).toHaveBeenCalledTimes(1)
    expect(hoisted.endLoading).toHaveBeenCalledTimes(1)
  })

  it('populates table data on successful fetch', async () => {
    hoisted.deviceStatusHistory.mockResolvedValue({
      data: {
        list: [
          { status: 1, change_time: 1719000000 },
          { status: 0, change_time: 1719000100 }
        ],
        total: 2
      },
      error: null
    })

    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.tableData).toHaveLength(2)
    expect(setupState.total).toBe(2)
    expect(setupState.pagination.itemCount).toBe(2)
  })

  it('pagination.onChange updates page and triggers fetch', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.pagination.onChange(3)
    await flushPromises()

    expect(setupState.queryParams.page).toBe(3)
    expect(setupState.pagination.page).toBe(3)
    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceStatusHistory).toHaveBeenCalledWith(
      expect.objectContaining({ page: 3 })
    )
  })

  it('pagination.onUpdatePageSize updates page size and resets page to 1', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.page = 5
    setupState.pagination.page = 5

    setupState.pagination.onUpdatePageSize(50)
    await flushPromises()

    expect(setupState.queryParams.page_size).toBe(50)
    expect(setupState.queryParams.page).toBe(1)
    expect(setupState.pagination.pageSize).toBe(50)
    expect(setupState.pagination.page).toBe(1)
    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceStatusHistory).toHaveBeenCalledWith(
      expect.objectContaining({ page: 1, page_size: 50 })
    )
  })

  it('index column render returns 1-based index', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const indexColumn = setupState.columns.find((c: any) => c.key === 'index')

    expect(indexColumn.render({}, 0)).toBe(1)
    expect(indexColumn.render({}, 4)).toBe(5)
  })

  it('change_time column renders formatted date when change_time exists', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const timeColumn = setupState.columns.find((c: any) => c.key === 'change_time')

    const result = timeColumn.render({ change_time: 1719000000000 })
    expect(typeof result).toBe('string')
    expect(result).not.toBe('--')
    expect(result).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/)
  })

  it('change_time column renders placeholder when change_time is missing', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const timeColumn = setupState.columns.find((c: any) => c.key === 'change_time')

    expect(timeColumn.render({})).toBe('--')
    expect(timeColumn.render({ change_time: undefined })).toBe('--')
    expect(timeColumn.render({ change_time: 0 })).toBe('--')
  })

  it('falls back to empty list and zero total when data fields are missing', async () => {
    hoisted.deviceStatusHistory.mockResolvedValue({
      data: {},
      error: null
    })

    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.tableData).toEqual([])
    expect(setupState.total).toBe(0)
    expect(setupState.pagination.itemCount).toBe(0)
  })

  it('fetchData includes start_time when queryParams.start_time is truthy', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = 5000
    setupState.queryParams.end_time = undefined
    setupState.queryParams.status = null

    await setupState.fetchData()
    await flushPromises()

    const callArgs = hoisted.deviceStatusHistory.mock.calls[0][0]
    expect(callArgs.start_time).toBe(5000)
    expect(callArgs).not.toHaveProperty('end_time')
    expect(callArgs).not.toHaveProperty('status')
  })

  it('fetchData includes end_time when queryParams.end_time is truthy', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = undefined
    setupState.queryParams.end_time = 8000
    setupState.queryParams.status = null

    await setupState.fetchData()
    await flushPromises()

    const callArgs = hoisted.deviceStatusHistory.mock.calls[0][0]
    expect(callArgs).not.toHaveProperty('start_time')
    expect(callArgs.end_time).toBe(8000)
    expect(callArgs).not.toHaveProperty('status')
  })

  it('fetchData includes status when queryParams.status is 0', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = undefined
    setupState.queryParams.end_time = undefined
    setupState.queryParams.status = 0

    await setupState.fetchData()
    await flushPromises()

    const callArgs = hoisted.deviceStatusHistory.mock.calls[0][0]
    expect(callArgs).not.toHaveProperty('start_time')
    expect(callArgs).not.toHaveProperty('end_time')
    expect(callArgs.status).toBe(0)
  })

  it('handleDateRangeChange clears timestamps when value is empty array', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = 1000
    setupState.queryParams.end_time = 2000

    setupState.handleDateRangeChange([] as any)

    expect(setupState.queryParams.start_time).toBeUndefined()
    expect(setupState.queryParams.end_time).toBeUndefined()
  })

  it('handleReset sets pagination.page to 1', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.pagination.page = 5

    setupState.handleReset()
    await flushPromises()

    expect(setupState.pagination.page).toBe(1)
  })

  it('does not re-fetch when visible becomes false', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    await wrapper.setProps({ visible: false })
    await flushPromises()

    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(0)
  })

  it('does not fetch when visible becomes true but deviceId is empty', async () => {
    const wrapper = mountDeviceStatus({ visible: false, deviceId: '' })
    await flushPromises()
    vi.clearAllMocks()

    await wrapper.setProps({ visible: true })
    await flushPromises()

    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(0)
  })

  it('does not re-fetch when deviceId changes to empty while visible', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    await wrapper.setProps({ deviceId: '' })
    await flushPromises()

    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(0)
  })

  it('fetchData handles API response with null data gracefully', async () => {
    hoisted.deviceStatusHistory.mockResolvedValue({
      data: null,
      error: null
    })

    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.tableData).toEqual([])
    expect(setupState.total).toBe(0)
  })

  it('fetchData includes status when queryParams.status is undefined (not null)', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = undefined
    setupState.queryParams.end_time = undefined
    setupState.queryParams.status = undefined

    await setupState.fetchData()
    await flushPromises()

    const callArgs = hoisted.deviceStatusHistory.mock.calls[0][0]
    expect(callArgs).not.toHaveProperty('start_time')
    expect(callArgs).not.toHaveProperty('end_time')
    expect(callArgs).not.toHaveProperty('status')
  })

  it('fetchData includes all params when start_time, end_time and status=0 are set together', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = 3000
    setupState.queryParams.end_time = 6000
    setupState.queryParams.status = 0

    await setupState.fetchData()
    await flushPromises()

    const callArgs = hoisted.deviceStatusHistory.mock.calls[0][0]
    expect(callArgs.start_time).toBe(3000)
    expect(callArgs.end_time).toBe(6000)
    expect(callArgs.status).toBe(0)
  })

  it('fetchData updates tableData, total and pagination on successful response with data', async () => {
    hoisted.deviceStatusHistory.mockResolvedValue({
      data: {
        list: [{ status: 1, change_time: 1719000000 }],
        total: 42
      },
      error: null
    })

    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = 1000
    setupState.queryParams.end_time = 2000
    setupState.queryParams.status = 1

    await setupState.fetchData()
    await flushPromises()

    expect(setupState.tableData).toHaveLength(1)
    expect(setupState.total).toBe(42)
    expect(setupState.pagination.itemCount).toBe(42)
  })

  it('handleReset clears all filters and triggers fetchData via nextTick', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.start_time = 5000
    setupState.queryParams.end_time = 9000
    setupState.queryParams.status = 1
    setupState.dateRangeValue = [5000000, 9000000]
    setupState.queryParams.page = 3
    setupState.pagination.page = 3

    setupState.handleReset()
    await flushPromises()

    expect(setupState.dateRangeValue).toBeNull()
    expect(setupState.queryParams.start_time).toBeUndefined()
    expect(setupState.queryParams.end_time).toBeUndefined()
    expect(setupState.queryParams.status).toBeNull()
    expect(setupState.queryParams.page).toBe(1)
    expect(setupState.pagination.page).toBe(1)
    expect(hoisted.deviceStatusHistory).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceStatusHistory).toHaveBeenCalledWith(
      expect.objectContaining({
        device_id: 'device-1',
        page: 1,
        page_size: 20
      })
    )
    const callArgs = hoisted.deviceStatusHistory.mock.calls[0][0]
    expect(callArgs).not.toHaveProperty('start_time')
    expect(callArgs).not.toHaveProperty('end_time')
    expect(callArgs).not.toHaveProperty('status')
  })

  it('handleDateRangeChange sets timestamps in seconds when value has length 2', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.handleDateRangeChange([1700000000000, 1700086400000])

    expect(setupState.dateRangeValue).toEqual([1700000000000, 1700086400000])
    expect(setupState.queryParams.start_time).toBe(Math.floor(1700000000000 / 1000))
    expect(setupState.queryParams.end_time).toBe(Math.floor(1700086400000 / 1000))
  })

  it('renders modal with correct title when visible', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const html = wrapper.html()
    expect(html).toContain('common.deviceActiveTime')
  })

  it('emits update:visible with false when showModal setter is called', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.showModal = false
    await flushPromises()

    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('emits update:visible when NModal update:show event fires', async () => {
    const wrapper = mountDeviceStatus({ visible: true, deviceId: 'device-1' })
    await flushPromises()

    const modal = wrapper.findComponent({ name: 'NModal' })
    modal.vm.$emit('update:show', false)
    await flushPromises()

    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })
})
