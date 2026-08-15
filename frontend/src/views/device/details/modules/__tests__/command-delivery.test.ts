/**
 * 文件用途: command-delivery 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/service/api', () => ({
  commandDataPub: vi.fn(),
  expectMessageAdd: vi.fn(),
  getCommandDataSetLogs: vi.fn(),
  getCommandDeliveryDiagnostics: vi.fn(),
  invokeDirectMethod: vi.fn()
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('dayjs', () => ({
  default: vi.fn(() => ({ format: vi.fn(() => '2024-01-01 00:00:00') }))
}))

import Component from '../command-delivery.vue'
import {
  commandDataPub,
  expectMessageAdd,
  getCommandDataSetLogs,
  getCommandDeliveryDiagnostics,
  invokeDirectMethod
} from '@/service/api'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []
const distributionStub = defineComponent({
  name: 'DistributionAndTable',
  props: [
    'id',
    'buttonName',
    'isCommand',
    'tableColumns',
    'fetchDataApi',
    'submitApi',
    'directMethodApi',
    'directMethodOnline',
    'expect',
    'expectApi',
    'onDirectMethodResult'
  ],
  emits: ['refresh'],
  setup() {
    return () => h('div', { class: 'distribution-and-table-stub' })
  }
})

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { id: 'device-1', ...props },
    global: {
      stubs: {
        DistributionAndTable: distributionStub
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/details/modules/command-delivery.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(getCommandDeliveryDiagnostics).mockResolvedValue({
      data: {
        is_online: true,
        latest_log: { message_id: 'mid-1', status: '3', status_label: 'device_ack_success' },
        conclusion: { level: 'ok', summary: 'Device acknowledged', next_actions: ['Open detail'] }
      }
    } as any)
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('wires DistributionAndTable with the command submission contract', async () => {
    const wrapper = mountComponent({ id: 'test-id' })
    await flushPromises()
    const table = wrapper.getComponent(distributionStub)

    expect(table.props()).toMatchObject({
      id: 'test-id',
      buttonName: 'generate.issueCommand',
      isCommand: true,
      fetchDataApi: getCommandDataSetLogs,
      submitApi: commandDataPub,
      directMethodApi: invokeDirectMethod,
      directMethodOnline: true,
      expect: true,
      expectApi: expectMessageAdd
    })
  })

  it('loads command delivery diagnostics for the current device', async () => {
    mountComponent({ id: 'diagnostic-device' })
    await flushPromises()

    expect(getCommandDeliveryDiagnostics).toHaveBeenCalledWith('diagnostic-device', { limit: 5 })
  })

  it('defines command log columns with status and fallback renderers', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const table = wrapper.getComponent(distributionStub)
    const columns = table.props('tableColumns') as Array<Record<string, any>>

    expect(columns.map((column) => column.key)).toEqual([
      'identify',
      'identify_name',
      'message_id',
      'created_at',
      'status',
      'data',
      'rsp_data',
      undefined,
      'actions'
    ])
    expect(columns[1].render({ identify_name: '' })).toBe('--')
    expect(columns[1].render({ identify_name: 'Reboot' })).toBe('Reboot')
    expect(columns[2].render({ message_id: '' })).toBe('--')
    expect(columns[2].render({ message_id: 'mid-1' })).toBe('mid-1')
    expect(columns[3].render({ created_at: '2026-06-21T10:20:30Z' })).toBe('2024-01-01 00:00:00')
    expect(columns[4].render({ status: '0' })).toBe('custom.device_details.commandStatusPending')
    expect(columns[4].render({ status: '1' })).toBe('custom.device_details.commandStatusSent')
    expect(columns[4].render({ status: '2' })).toBe('custom.device_details.commandStatusSendFailed')
    expect(columns[4].render({ status: '3' })).toBe('custom.device_details.commandStatusDeviceSuccess')
    expect(columns[4].render({ status: '4' })).toBe('custom.device_details.commandStatusDeviceFailed')
    expect(columns[4].render({ status: '99' })).toBe('custom.device_details.commandStatusUnknown: 99')
    expect(columns[6].render({ rsp_data: '' })).toBe('--')
    expect(columns[6].render({ rsp_data: '{"ok":true}' })).toBe('{"ok":true}')
    expect(columns[7].render({ error_message: '' })).toBe('--')
    expect(columns[7].render({ error_message: 'timeout' })).toBe('timeout')
    const actionCell = columns[8].render({ identify: 'reboot' }) as any
    expect(actionCell.props.size).toBe('small')
    expect(actionCell.props.secondary).toBe(true)
    expect(actionCell.children.default()).toBe('custom.device_details.commandDetailAction')
  })
})
