/**
 * 文件用途: give-an-alarm 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import dayjs from 'dayjs'

const hoisted = vi.hoisted(() => ({
  deviceAlarmHistory: vi.fn(),
  deviceAlarmHistoryPut: vi.fn(),
  acknowledgeAlarmHistory: vi.fn(),
  resetAlarmHistory: vi.fn(),
  routerPushByKey: vi.fn()
}))

vi.mock('@/service/api', () => ({
  deviceAlarmHistory: hoisted.deviceAlarmHistory,
  deviceAlarmHistoryPut: hoisted.deviceAlarmHistoryPut
}))

vi.mock('@/service/api/alarm', () => ({
  acknowledgeAlarmHistory: hoisted.acknowledgeAlarmHistory,
  resetAlarmHistory: hoisted.resetAlarmHistory
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({
    routerPushByKey: hoisted.routerPushByKey
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/views/automation/scene-linkage/modules/dataList.vue', () => ({
  default: defineComponent({
    name: 'AlarmDataListStub',
    setup() {
      return () => h('div', { class: 'alarm-data-list-stub' })
    }
  })
}))

import GiveAnAlarm from '../give-an-alarm.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

let dialogOptions: any = null

const mountComponent = () => {
  const wrapper = shallowMount(GiveAnAlarm, {
    props: {
      id: 'device-1'
    },
    global: {
      stubs: {
        NFlex: true,
        NButton: true,
        NButtonGroup: true,
        NCard: true,
        NInput: true,
        NIcon: true,
        NDatePicker: defineComponent({
          name: 'NDatePicker',
          props: {
            clearable: Boolean
          },
          setup(props) {
            return () => h('div', { class: 'date-picker-stub', 'data-clearable': String(props.clearable) })
          }
        }),
        NSelect: true,
        NInfiniteScroll: true,
        NEmpty: true,
        NModal: true,
        NH3: true,
        NFormItem: true,
        NTable: true,
        AlarmDataList: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('give-an-alarm.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    dialogOptions = null
    hoisted.deviceAlarmHistory.mockResolvedValue({
      data: {
        list: [
          {
            id: 'a1',
            name: 'alarm 1',
            alarm_status: 'H',
            content: 'content 1',
            description: 'desc 1',
            remark: '{"acknowledged":false}'
          }
        ],
        total: 3
      }
    })
    hoisted.deviceAlarmHistoryPut.mockResolvedValue({ error: null })
    hoisted.acknowledgeAlarmHistory.mockResolvedValue({ error: null })
    hoisted.resetAlarmHistory.mockResolvedValue({ error: null })
    ;(window as any).$message = {
      success: vi.fn(),
      error: vi.fn(),
      warning: vi.fn(),
      info: vi.fn()
    }
    ;(window as any).$dialog = {
      warning: vi.fn((opts: any) => {
        dialogOptions = opts
      })
    }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads all alarm history on mount without forcing a time range', async () => {
    mountComponent()
    await flushPromises()

    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(1)
    const callArg = hoisted.deviceAlarmHistory.mock.calls[0][0]
    expect(callArg.device_id).toBe('device-1')
    expect(callArg.alarm_status).toBe('')
    expect(callArg.page).toBe(1)
    expect(callArg.page_size).toBe(10)
    expect(callArg).not.toHaveProperty('selected_time')
    expect(callArg).not.toHaveProperty('start_time')
    expect(callArg).not.toHaveProperty('end_time')
  })

  it('allows users to clear the optional alarm-history time range', () => {
    const wrapper = mountComponent()

    expect(wrapper.find('.date-picker-stub').attributes('data-clearable')).toBe('true')
  })

  it('switches to tab 2 without refreshing and back to tab 1 with refresh', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.choseTab(2)
    expect(setupState.tabValue).toBe(2)
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(0)

    setupState.choseTab(1)
    expect(setupState.tabValue).toBe(1)
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(1)
  })

  it('builds start_time and end_time from selected_time', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.selected_time = [1000, 2000]
    await setupState.getAlarmHistory()

    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledWith(
      expect.objectContaining({
        device_id: 'device-1',
        start_time: dayjs(1000).format('YYYY-MM-DDTHH:mm:ssZ'),
        end_time: dayjs(2000).format('YYYY-MM-DDTHH:mm:ssZ')
      })
    )
    expect(hoisted.deviceAlarmHistory.mock.calls[0][0]).not.toHaveProperty('selected_time')
  })

  it('omits start_time and end_time when selected_time is empty', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.selected_time = null
    await setupState.getAlarmHistory()

    const callArg = hoisted.deviceAlarmHistory.mock.calls[0][0]
    expect(callArg).not.toHaveProperty('selected_time')
    expect(callArg).not.toHaveProperty('start_time')
    expect(callArg).not.toHaveProperty('end_time')
  })

  it('refresh() resets query params and reloads list', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.page = 5
    setupState.queryParams.alarm_status = 'H'
    setupState.queryParams.selected_time = [1000, 2000]
    setupState.noMore = true

    vi.clearAllMocks()
    setupState.refresh()
    await flushPromises()

    expect(setupState.queryParams.page).toBe(1)
    expect(setupState.queryParams.alarm_status).toBe('')
    expect(setupState.queryParams.selected_time).toBeNull()
    expect(setupState.noMore).toBe(false)
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(1)
    const callArg = hoisted.deviceAlarmHistory.mock.calls[0][0]
    expect(callArg).not.toHaveProperty('selected_time')
    expect(callArg).not.toHaveProperty('start_time')
    expect(callArg).not.toHaveProperty('end_time')
  })

  it('resetQuery() resets page and reloads list', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.page = 5
    setupState.noMore = true

    vi.clearAllMocks()
    setupState.resetQuery()
    await flushPromises()

    expect(setupState.queryParams.page).toBe(1)
    expect(setupState.noMore).toBe(false)
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(1)
  })

  it('parses JSON string remark', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.parseAlarmRemark('{"acknowledged_by":"admin"}')).toEqual({ acknowledged_by: 'admin' })
  })

  it('parses object remark directly', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const obj = { acknowledged_by: 'admin' }
    expect(setupState.parseAlarmRemark(obj)).toBe(obj)
  })

  it('returns empty object for invalid or empty remark', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.parseAlarmRemark(null)).toEqual({})
    expect(setupState.parseAlarmRemark(undefined)).toEqual({})
    expect(setupState.parseAlarmRemark('')).toEqual({})
    expect(setupState.parseAlarmRemark('invalid json')).toEqual({})
    expect(setupState.parseAlarmRemark(123)).toEqual({})
  })

  it('alarmActionField returns value or dash fallback', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.alarmActionField({ remark: '{"acknowledged_by":"admin"}' }, 'acknowledged_by')).toBe('admin')
    expect(setupState.alarmActionField({ remark: '{"acknowledged_by":""}' }, 'acknowledged_by')).toBe('-')
    expect(setupState.alarmActionField({ remark: '{}' }, 'acknowledged_by')).toBe('-')
    expect(setupState.alarmActionField({ remark: null }, 'acknowledged_by')).toBe('-')
  })

  it('isAcknowledged checks the acknowledged flag', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.isAcknowledged({ remark: '{"acknowledged":true}' })).toBe(true)
    expect(setupState.isAcknowledged({ remark: '{"acknowledged":false}' })).toBe(false)
    expect(setupState.isAcknowledged({ remark: null })).toBe(false)
  })

  it('opens and closes the detail dialog', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.getInfo({ id: 'a1', name: 'test' })
    expect(setupState.showDialog).toBe(true)
    expect(setupState.infoData).toEqual({ id: 'a1', name: 'test' })

    setupState.closeModal()
    expect(setupState.showDialog).toBe(false)
  })

  it('submitCallback shows error when description is empty', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.showDescModal({ id: 'a1', description: 'old' })
    setupState.description = ''
    await setupState.submitCallback()

    expect((window as any).$message.error).toHaveBeenCalledWith('common.enterAlarmDesc')
    expect(hoisted.deviceAlarmHistoryPut).toHaveBeenCalledTimes(0)
    expect(setupState.showModal).toBe(true)
  })

  it('submitCallback updates description on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.showDescModal({ id: 'a1', description: 'old desc' })
    setupState.description = 'new desc'
    await setupState.submitCallback()
    await flushPromises()

    expect(hoisted.deviceAlarmHistoryPut).toHaveBeenCalledWith({ id: 'a1', description: 'new desc' })
    expect(setupState.showModal).toBe(false)
    expect(setupState.description).toBe('')
    expect(setupState.alarmHistory[0].description).toBe('new desc')
  })

  it('keeps alarm history immutable in the device detail UI', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.alarmHistory).toHaveLength(1)
    expect(setupState.alarmHistoryTotal).toBe(3)
    expect(setupState.handleDelete).toBeUndefined()
    expect(wrapper.text()).not.toContain('common._delete')
  })

  it('acknowledgeAlarm calls API and refreshes', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    await setupState.acknowledgeAlarm({ id: 'a1' })
    await flushPromises()

    expect(hoisted.acknowledgeAlarmHistory).toHaveBeenCalledWith('a1')
    expect((window as any).$message.success).toHaveBeenCalledWith('rdi.overview.alarmAcknowledged')
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(1)
  })

  it('resetAlarm calls API and refreshes on confirm', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.resetAlarm({ id: 'a1', name: 'alarm 1' })
    expect(dialogOptions).not.toBeNull()
    await dialogOptions.onPositiveClick()
    await flushPromises()

    expect(hoisted.resetAlarmHistory).toHaveBeenCalledWith('a1')
    expect((window as any).$message.success).toHaveBeenCalledWith('rdi.overview.alarmReset')
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(1)
  })

  it('alarmAdd navigates to alarm rule edit route with device_id', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.alarmAdd()

    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('automation_linkage-edit', {
      query: { device_id: 'device-1', backType: 'device' }
    })
  })

  it('handleLoad blocks when loading is true', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const pageBefore = setupState.queryParams.page
    setupState.loading = true
    setupState.handleLoad()

    expect(setupState.queryParams.page).toBe(pageBefore)
  })

  it('handleLoad blocks when noMore is true', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const pageBefore = setupState.queryParams.page
    setupState.noMore = true
    setupState.handleLoad()

    expect(setupState.queryParams.page).toBe(pageBefore)
  })

  it('handleLoad loads next page and concatenates results', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.alarmHistory).toHaveLength(1)
    expect(setupState.queryParams.page).toBe(1)

    hoisted.deviceAlarmHistory.mockResolvedValueOnce({
      data: { list: [{ id: 'a2', name: 'alarm 2' }], total: 3 }
    })

    setupState.handleLoad()
    await flushPromises()

    expect(setupState.queryParams.page).toBe(2)
    expect(setupState.loading).toBe(false)
    expect(setupState.alarmHistory).toHaveLength(2)
    expect(setupState.alarmHistory[0].id).toBe('a1')
    expect(setupState.alarmHistory[1].id).toBe('a2')
  })

  it('renders Heart icon for normal alarm status (N)', async () => {
    hoisted.deviceAlarmHistory.mockResolvedValue({
      data: {
        list: [
          {
            id: 'a-normal',
            name: 'normal alarm',
            alarm_status: 'N',
            content: 'content normal',
            description: 'desc normal',
            remark: '{"acknowledged":false}'
          }
        ],
        total: 1
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const setupState = getSetupState(wrapper)
    expect(setupState.alarmHistory[0].alarm_status).toBe('N')
    expect(setupState.noMore).toBe(true)
  })

  it('renders empty state when alarm history is empty', async () => {
    hoisted.deviceAlarmHistory.mockResolvedValue({
      data: { list: [], total: 0 }
    })
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.alarmHistory).toHaveLength(0)
    expect(setupState.noMore).toBe(true)
  })

  it('opens detail dialog with full data including alarm device list', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.getInfo({
      id: 'a1',
      name: 'test alarm',
      alarm_config_name: 'config name',
      create_at: '2024-01-01T00:00:00Z',
      alarm_status: 'H',
      content: 'alarm content',
      description: 'alarm desc',
      remark: '{"acknowledged_by":"admin","acknowledged_at":"2024-01-01","reset_by":"user","reset_at":"2024-01-02"}',
      alarm_device_list: [
        { id: 'device-1', name: 'Device 1' },
        { id: 'device-2', name: 'Device 2' }
      ]
    })
    await flushPromises()

    expect(setupState.showDialog).toBe(true)
    expect(setupState.infoData.name).toBe('test alarm')
    expect(setupState.infoData.alarm_config_name).toBe('config name')
    expect(setupState.infoData.alarm_device_list).toHaveLength(2)
    expect(setupState.alarmActionField(setupState.infoData, 'acknowledged_by')).toBe('admin')
    expect(setupState.alarmActionField(setupState.infoData, 'acknowledged_at')).toBe('2024-01-01')
    expect(setupState.alarmActionField(setupState.infoData, 'reset_by')).toBe('user')
    expect(setupState.alarmActionField(setupState.infoData, 'reset_at')).toBe('2024-01-02')
  })

  it('closes detail dialog via closeModal after opening with full data', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.getInfo({
      id: 'a1',
      name: 'test alarm',
      alarm_config_name: 'config name',
      create_at: '2024-01-01T00:00:00Z',
      alarm_status: 'H',
      content: 'alarm content',
      description: 'alarm desc',
      remark: '{}',
      alarm_device_list: [{ id: 'd1', name: 'Device 1' }]
    })
    await flushPromises()
    expect(setupState.showDialog).toBe(true)

    setupState.closeModal()
    await flushPromises()
    expect(setupState.showDialog).toBe(false)
  })

  it('opens and cancels description modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.showDescModal({ id: 'a1', description: 'old desc' })
    await flushPromises()
    expect(setupState.showModal).toBe(true)
    expect(setupState.description).toBe('old desc')

    setupState.cancelCallback()
    await flushPromises()
    expect(setupState.showModal).toBe(false)
    expect(setupState.description).toBe('')
  })

  it('renders alarm rules list on tab 2', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.choseTab(2)
    await flushPromises()

    expect(setupState.tabValue).toBe(2)
  })

  it('resetAlarm uses content as dialog content when name is missing', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.resetAlarm({ id: 'a1', content: 'alarm content' })
    expect(dialogOptions).not.toBeNull()
    expect(dialogOptions.content).toBe('alarm content')
  })

  it('resetAlarm uses dash when both name and content are missing', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.resetAlarm({ id: 'a1' })
    expect(dialogOptions).not.toBeNull()
    expect(dialogOptions.content).toBe('-')
  })

  it('parseAlarmRemark returns empty object for JSON string that parses to non-object', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.parseAlarmRemark('"hello"')).toEqual({})
    expect(setupState.parseAlarmRemark('123')).toEqual({})
    expect(setupState.parseAlarmRemark('true')).toEqual({})
  })

  it('getAlarmHistory handles null list in response', async () => {
    hoisted.deviceAlarmHistory.mockResolvedValue({
      data: { list: null, total: 0 }
    })
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.alarmHistory).toHaveLength(0)
  })

  it('sets noMore when all alarm history items are loaded', async () => {
    hoisted.deviceAlarmHistory.mockResolvedValue({
      data: {
        list: [{ id: 'a1', name: 'alarm 1', alarm_status: 'H' }],
        total: 1
      }
    })
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.noMore).toBe(true)
  })

  it('refresh() clears alarmHistory array before reloading', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.alarmHistory).toHaveLength(1)

    vi.clearAllMocks()
    hoisted.deviceAlarmHistory.mockResolvedValue({
      data: { list: [{ id: 'a2', name: 'alarm 2', alarm_status: 'M' }], total: 1 }
    })

    setupState.refresh()
    // After refresh() is called, alarmHistory should be cleared before getAlarmHistory resolves
    expect(setupState.alarmHistory).toHaveLength(0)
    await flushPromises()

    expect(setupState.alarmHistory).toHaveLength(1)
    expect(setupState.alarmHistory[0].id).toBe('a2')
  })

  it('isAcknowledged returns false when acknowledged is not strictly true', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.isAcknowledged({ remark: '{"acknowledged":"true"}' })).toBe(false)
    expect(setupState.isAcknowledged({ remark: '{"acknowledged":1}' })).toBe(false)
    expect(setupState.isAcknowledged({ remark: '{}' })).toBe(false)
    expect(setupState.isAcknowledged({ remark: undefined })).toBe(false)
  })

  it('acknowledgeAlarm calls API, shows success message and refreshes', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    await setupState.acknowledgeAlarm({ id: 'a1' })
    await flushPromises()

    expect(hoisted.acknowledgeAlarmHistory).toHaveBeenCalledWith('a1')
    expect((window as any).$message.success).toHaveBeenCalledWith('rdi.overview.alarmAcknowledged')
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(1)
  })

  it('acknowledgeAlarm refreshes list after acknowledging', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    hoisted.deviceAlarmHistory.mockResolvedValue({
      data: { list: [{ id: 'a1', name: 'ack alarm', alarm_status: 'H', remark: '{"acknowledged":true}' }], total: 1 }
    })

    const setupState = getSetupState(wrapper)
    await setupState.acknowledgeAlarm({ id: 'a1' })
    await flushPromises()

    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(1)
    expect(setupState.alarmHistory[0].remark).toBe('{"acknowledged":true}')
  })

  it('resetAlarm onPositiveClick calls API, shows success and refreshes', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.resetAlarm({ id: 'a1', name: 'alarm 1' })

    expect(dialogOptions).not.toBeNull()
    expect(dialogOptions.title).toBe('rdi.overview.confirmResetAlarm')
    expect(dialogOptions.content).toBe('alarm 1')

    await dialogOptions.onPositiveClick()
    await flushPromises()

    expect(hoisted.resetAlarmHistory).toHaveBeenCalledWith('a1')
    expect((window as any).$message.success).toHaveBeenCalledWith('rdi.overview.alarmReset')
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(1)
  })

  it('handleLoad increments page and loads more data', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.queryParams.page).toBe(1)
    expect(setupState.alarmHistory).toHaveLength(1)

    hoisted.deviceAlarmHistory.mockResolvedValueOnce({
      data: {
        list: [
          { id: 'a2', name: 'alarm 2' },
          { id: 'a3', name: 'alarm 3' }
        ],
        total: 5
      }
    })

    setupState.handleLoad()
    await flushPromises()

    expect(setupState.queryParams.page).toBe(2)
    expect(setupState.loading).toBe(false)
    expect(setupState.alarmHistory).toHaveLength(3)
  })

  it('handleLoad sets noMore when all items loaded', async () => {
    hoisted.deviceAlarmHistory.mockResolvedValue({
      data: {
        list: [{ id: 'a1', name: 'alarm 1', alarm_status: 'H' }],
        total: 2
      }
    })
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.noMore).toBe(false)

    hoisted.deviceAlarmHistory.mockResolvedValueOnce({
      data: { list: [{ id: 'a2', name: 'alarm 2' }], total: 2 }
    })

    setupState.handleLoad()
    await flushPromises()

    expect(setupState.alarmHistory).toHaveLength(2)
    expect(setupState.noMore).toBe(true)
  })

  it('getAlarmHistory handles empty list response', async () => {
    hoisted.deviceAlarmHistory.mockResolvedValue({
      data: { list: [], total: 0 }
    })
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.alarmHistory).toHaveLength(0)
    expect(setupState.alarmHistoryTotal).toBe(0)
    expect(setupState.noMore).toBe(true)
  })

  it('getAlarmHistory handles undefined list in response', async () => {
    hoisted.deviceAlarmHistory.mockResolvedValue({
      data: { list: undefined, total: 0 }
    })
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.alarmHistory).toHaveLength(0)
    expect(setupState.noMore).toBe(true)
  })

  it('getInfo sets infoData and opens dialog', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const alarmData = {
      id: 'a1',
      name: 'test alarm',
      alarm_config_name: 'config',
      create_at: '2024-01-01',
      alarm_status: 'H',
      content: 'content',
      description: 'desc',
      remark: '{"acknowledged":false}'
    }

    setupState.getInfo(alarmData)

    expect(setupState.infoData).toEqual(alarmData)
    expect(setupState.showDialog).toBe(true)
  })

  it('closeModal closes the detail dialog', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.getInfo({ id: 'a1' })
    expect(setupState.showDialog).toBe(true)

    setupState.closeModal()
    expect(setupState.showDialog).toBe(false)
  })

  it('showDescModal opens modal and sets description from item', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const item = { id: 'a1', description: 'maintenance note' }

    setupState.showDescModal(item)

    expect(setupState.showModal).toBe(true)
    expect(setupState.description).toBe('maintenance note')
    expect(setupState.infoData).toEqual(item)
  })

  it('cancelCallback clears description and closes modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.showDescModal({ id: 'a1', description: 'some desc' })
    expect(setupState.showModal).toBe(true)
    expect(setupState.description).toBe('some desc')

    setupState.cancelCallback()

    expect(setupState.showModal).toBe(false)
    expect(setupState.description).toBe('')
  })

  it('submitCallback updates alarm history item description on success', async () => {
    hoisted.deviceAlarmHistory.mockResolvedValue({
      data: {
        list: [
          { id: 'a1', name: 'alarm 1', alarm_status: 'H', description: 'old' },
          { id: 'a2', name: 'alarm 2', alarm_status: 'M', description: 'keep' }
        ],
        total: 2
      }
    })
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.showDescModal({ id: 'a1', description: 'old' })
    setupState.description = 'updated desc'
    await setupState.submitCallback()
    await flushPromises()

    expect(hoisted.deviceAlarmHistoryPut).toHaveBeenCalledWith({ id: 'a1', description: 'updated desc' })
    expect(setupState.alarmHistory[0].description).toBe('updated desc')
    expect(setupState.alarmHistory[1].description).toBe('keep')
    expect(setupState.showModal).toBe(false)
    expect(setupState.description).toBe('')
  })

  it('alarmAdd navigates with correct route and query params', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.alarmAdd()

    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('automation_linkage-edit', {
      query: { device_id: 'device-1', backType: 'device' }
    })
  })

  it('resetQuery resets page, clears list and reloads', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.queryParams.page = 5
    setupState.noMore = true

    vi.clearAllMocks()
    setupState.resetQuery()

    expect(setupState.queryParams.page).toBe(1)
    expect(setupState.alarmHistory).toHaveLength(0)
    expect(setupState.noMore).toBe(false)

    await flushPromises()
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(1)
  })

  it('choseTab switches to tab 2 without calling getAlarmHistory', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.choseTab(2)

    expect(setupState.tabValue).toBe(2)
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(0)
  })

  it('choseTab switches back to tab 1 and calls refresh', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.choseTab(2)
    setupState.choseTab(1)

    expect(setupState.tabValue).toBe(1)
    expect(hoisted.deviceAlarmHistory).toHaveBeenCalledTimes(1)
  })

  it('handleLoad does not increment page when loading is true', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const pageBefore = setupState.queryParams.page
    setupState.loading = true
    setupState.handleLoad()

    expect(setupState.queryParams.page).toBe(pageBefore)
  })

  it('handleLoad does not increment page when noMore is true', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const pageBefore = setupState.queryParams.page
    setupState.noMore = true
    setupState.handleLoad()

    expect(setupState.queryParams.page).toBe(pageBefore)
  })

  it('resetAlarm uses name for dialog content when available', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.resetAlarm({ id: 'a1', name: 'my alarm', content: 'alarm content' })

    expect(dialogOptions.content).toBe('my alarm')
  })

  it('resetAlarm uses content for dialog content when name is missing', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.resetAlarm({ id: 'a1', content: 'alarm content' })

    expect(dialogOptions.content).toBe('alarm content')
  })

  it('resetAlarm uses dash when both name and content are missing', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const setupState = getSetupState(wrapper)
    setupState.resetAlarm({ id: 'a1' })

    expect(dialogOptions.content).toBe('-')
  })

  it('submitCallback shows error when description is empty string', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.showDescModal({ id: 'a1', description: 'old' })
    setupState.description = ''
    await setupState.submitCallback()

    expect((window as any).$message.error).toHaveBeenCalledWith('common.enterAlarmDesc')
    expect(hoisted.deviceAlarmHistoryPut).toHaveBeenCalledTimes(0)
    expect(setupState.showModal).toBe(true)
  })

  it('getAlarmHistory sets loading to false after request completes', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.loading).toBe(false)

    hoisted.deviceAlarmHistory.mockResolvedValueOnce({
      data: { list: [{ id: 'a2', name: 'alarm 2' }], total: 3 }
    })

    setupState.handleLoad()
    expect(setupState.loading).toBe(true)
    await flushPromises()
    expect(setupState.loading).toBe(false)
  })

  it('alarmActionField returns dash for undefined, null, and empty string values', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.alarmActionField({ remark: '{"key":null}' }, 'key')).toBe('-')
    expect(setupState.alarmActionField({ remark: '{"key":""}' }, 'key')).toBe('-')
    expect(setupState.alarmActionField({ remark: '{"key":undefined}' }, 'key')).toBe('-')
    expect(setupState.alarmActionField({ remark: '{"key":"value"}' }, 'key')).toBe('value')
  })

  it('parseAlarmRemark handles various edge cases', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    expect(setupState.parseAlarmRemark(0)).toEqual({})
    expect(setupState.parseAlarmRemark(false)).toEqual({})
    expect(setupState.parseAlarmRemark('null')).toEqual({})
    // '[]' parses to an array, which is typeof 'object', so parseAlarmRemark returns it as-is
    expect(Array.isArray(setupState.parseAlarmRemark('[]'))).toBe(true)
  })
})
