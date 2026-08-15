/**
 * 文件用途: automate 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/views/automation/scene-linkage/modules/dataList.vue', () => ({
  default: defineComponent({ name: 'sceneLinkage', props: ['device_id', 'backType'], setup() { return () => h('div') } })
}))

import Component from '../automate.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const mountComponent = (props = {}) => {
  const wrapper = mount(Component, {
    props: { id: 'device-1', ...props },
    global: {
      stubs: {
        sceneLinkage: defineComponent({ name: 'sceneLinkage', props: ['device_id', 'backType'], setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/details/modules/automate.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds scene linkage to the current device context', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const sceneLinkage = wrapper.getComponent({ name: 'sceneLinkage' })

    expect(sceneLinkage.props('device_id')).toBe('device-1')
    expect(sceneLinkage.props('backType')).toBe('device')
  })

  it('accepts id prop', async () => {
    const wrapper = mountComponent({ id: 'test-id' })
    await flushPromises()
    expect(wrapper.props('id')).toBe('test-id')
  })

  it('renders sceneLinkage component with correct props', async () => {
    const wrapper = mountComponent({ id: 'device-123' })
    await flushPromises()
    const sceneLinkage = wrapper.getComponent({ name: 'sceneLinkage' })
    expect(sceneLinkage.props('device_id')).toBe('device-123')
    expect(sceneLinkage.props('backType')).toBe('device')
  })
})
