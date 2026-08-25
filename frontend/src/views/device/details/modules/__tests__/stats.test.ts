/**
 * 文件用途: stats 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/service/api', () => ({
  attributeDataPub: vi.fn(),
  deleteAttributeDataSet: vi.fn(),
  expectMessageAdd: vi.fn(),
  getAttributeDataSet: vi.fn(),
  getAttributeDataSetLogs: vi.fn()
}))

vi.mock('dayjs', () => ({
  default: vi.fn(() => ({ format: vi.fn(() => '2024-01-01 00:00:00') }))
}))

import Component from '../stats.vue'

// setupState 的受控视图：列结构只需要 key/title，其余成员走 unknown 兜底。
interface StatsTableColumn {
  key: string
  title: unknown
}

interface StatsSetupState {
  columns: StatsTableColumn[]
  columns0: StatsTableColumn[]
  formatOperationType: (...args: unknown[]) => unknown
  formatStatus: (...args: unknown[]) => unknown
  [key: string]: unknown
}

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { id: 'device-1', ...props },
    global: {
      stubs: {
        DistributionAndTable: defineComponent({ name: 'DistributionAndTable', props: ['id', 'noRefresh', 'tableColumns', 'fetchDataApi', 'buttonName', 'submitApi', 'expect', 'expectApi'], emits: ['refresh'], setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/details/modules/stats.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds attribute current data and log tables to the device id and APIs', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const tables = wrapper.findAllComponents({ name: 'DistributionAndTable' })
    const state = wrapper.vm.$.setupState as StatsSetupState

    expect(tables).toHaveLength(2)
    expect(tables[0].props()).toMatchObject({
      id: 'device-1',
      noRefresh: true,
      tableColumns: state.columns0,
      fetchDataApi: expect.any(Function)
    })
    expect(tables[1].props()).toMatchObject({
      id: 'device-1',
      buttonName: 'generate.issue-attribute',
      tableColumns: state.columns,
      fetchDataApi: expect.any(Function),
      submitApi: expect.any(Function),
      expect: true,
      expectApi: expect.any(Function)
    })
  })

  it('accepts id prop', async () => {
    const wrapper = mountComponent({ id: 'test-id' })
    await flushPromises()
    expect(wrapper.props('id')).toBe('test-id')
  })

  it('has two sets of columns for DistributionAndTable', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    // shallowMount stubs child components, so verify the setup state has the expected columns
    const state = wrapper.vm.$.setupState as StatsSetupState
    expect(state.columns0.map(column => column.key)).toEqual(['key', 'data_name', 'value', 'ts', 'created_at'])
    expect(state.columns.map(column => column.key)).toEqual([
      'created_at',
      'message_id',
      'data',
      'operation_type',
      'status',
      'error_message'
    ])
  })

  it('has correct columns0 for attribute data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as StatsSetupState
    expect(state.columns0.map(column => column.title)).toEqual([
      'device_template.table_header.attributeIdentifier',
      'device_template.table_header.attributeName',
      'device_template.table_header.attributeValue',
      'device_template.table_header.updateTime',
      'common.actions'
    ])
    expect(state.columns0[2].render({ value: 23, unit: 'C' })).toBe('23C')
    expect(state.columns0[2].render({ value: 23, unit: null })).toBe('23')
  })

  it('has correct columns for attribute logs', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as StatsSetupState
    expect(state.columns.map(column => column.title)).toEqual([
      'custom.device_details.attributeDistributionTime',
      'custom.device_details.messageId',
      'custom.device_details.sendContent',
      'custom.device_details.operationType',
      'generate.status',
      'generate.errorMessage'
    ])
    expect(state.columns[3].render({ status: '1' })).toBe('custom.device_details.manualOperation')
    expect(state.columns[4].render({ status: '3' })).toBe('generate.returnSuccess')
  })

  it('formatOperationType returns correct labels', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as StatsSetupState
    expect(state.formatOperationType('1')).toBe('custom.device_details.manualOperation')
    expect(state.formatOperationType('2')).toBe('custom.device_details.automaticTriggering')
    expect(state.formatOperationType('99')).toBe('')
  })

  it('formatStatus returns correct labels', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as StatsSetupState
    expect(state.formatStatus('1')).toBe('generate.sendingSuccess')
    expect(state.formatStatus('2')).toBe('generate.sendingFail')
    expect(state.formatStatus('3')).toBe('generate.returnSuccess')
    expect(state.formatStatus('4')).toBe('generate.returnFail')
    expect(state.formatStatus('99')).toBe('')
  })
})
