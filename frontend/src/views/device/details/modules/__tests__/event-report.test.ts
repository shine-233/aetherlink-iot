/**
 * 文件用途: event-report 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/service/api', () => ({
  getEventDataSet: vi.fn()
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('dayjs', () => ({
  default: vi.fn(() => ({ format: vi.fn(() => '2024-01-01 00:00:00') }))
}))

import Component from '../event-report.vue'
import { getEventDataSet } from '@/service/api'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []
const distributionStub = defineComponent({
  name: 'DistributionAndTable',
  props: ['id', 'tableColumns', 'fetchDataApi'],
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

describe('device/details/modules/event-report.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('passes the device id and event fetch API to DistributionAndTable', async () => {
    const wrapper = mountComponent({ id: 'test-id' })
    await flushPromises()
    const table = wrapper.getComponent(distributionStub)

    expect(table.props('id')).toBe('test-id')
    expect(table.props('fetchDataApi')).toBe(getEventDataSet)
  })

  it('defines the expected event table columns and formats timestamps', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const table = wrapper.getComponent(distributionStub)
    const columns = table.props('tableColumns') as Array<Record<string, any>>

    expect(columns.map(column => column.key)).toEqual([
      'identify',
      'data_name',
      'ts',
      'data',
      'error_message'
    ])
    expect(columns[2].title).toBe('device_template.table_header.eventReportingTime')
    expect(columns[2].render({ ts: '2026-06-21T10:20:30Z' })).toBe('2024-01-01 00:00:00')
  })
})
