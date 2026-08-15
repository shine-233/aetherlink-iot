/**
 * 文件用途: automate 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/views/automation/scene-linkage/modules/dataList.vue', () => ({
  default: defineComponent({
    name: 'sceneLinkage',
    props: ['deviceConfigId', 'backType'],
    setup(props, { slots }) {
      return () => h('div', { 'data-config-id': props.deviceConfigId }, slots.default ? slots.default() : [])
    }
  })
}))

import Component from '../automate.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { configId: 'cfg-1', ...props },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/config-detail/modules/automate.vue', () => {
  beforeEach(() => { vi.clearAllMocks() })
  afterEach(() => { while (mountedWrappers.length > 0) { mountedWrappers.pop()?.unmount() } })

  it('renders scene linkage list scoped to the device config', () => {
    const wrapper = mountComponent()
    const scene = wrapper.getComponent({ name: 'sceneLinkage' })
    expect(scene.props()).toMatchObject({
      deviceConfigId: 'cfg-1',
      backType: 'config'
    })
  })

  it('passes configId prop to sceneLinkage', () => {
    const wrapper = mountComponent({ configId: 'cfg-99' })
    expect(wrapper.getComponent({ name: 'sceneLinkage' }).props('deviceConfigId')).toBe('cfg-99')
  })
})
