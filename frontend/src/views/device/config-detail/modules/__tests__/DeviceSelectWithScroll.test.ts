/**
 * 文件用途: DeviceSelectWithScroll 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('naive-ui', () => ({
  NCheckbox: defineComponent({ props: { checked: { default: false } }, emits: ['update:checked'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NEmpty: defineComponent({ setup() { return () => h('div') } }),
  NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NInfiniteScroll: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NPopover: defineComponent({ props: { show: { default: false } }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NSelect: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
  NSpin: defineComponent({ setup() { return () => h('div') } })
}))

import Component from '../DeviceSelectWithScroll.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      modelValue: null,
      options: [
        { device_id: 'd1', device_name: 'Device 1' },
        { device_id: 'd2', device_name: 'Device 2' }
      ],
      loading: false,
      hasMore: true,
      ...props
    },
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

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config-detail/modules/DeviceSelectWithScroll.vue', () => {
  beforeEach(() => { vi.clearAllMocks() })
  afterEach(() => { while (mountedWrappers.length > 0) { mountedWrappers.pop()?.unmount() } })

  it('initializes selected ids from modelValue and exposes selected option tags', () => {
    const wrapper = mountComponent({ modelValue: ['d1'] })
    const state = getSetupState(wrapper)
    expect(state.selectedDeviceIds).toEqual(['d1'])
    expect(state.selectedOptions).toEqual([{ device_id: 'd1', device_name: 'Device 1' }])
    expect(state.filteredOptions.map((option: any) => option.device_id)).toEqual(['d1', 'd2'])
    expect(state.isSelected('d1')).toBe(true)
    expect(state.isSelected('d2')).toBe(false)
  })

  it('toggles a device selection and emits the updated id list', () => {
    const wrapper = mountComponent({ modelValue: ['d1'] })
    const state = getSetupState(wrapper)
    state.handleOptionClick('d2')
    expect(wrapper.emitted('update:modelValue')).toEqual([[['d1', 'd2']]])
    state.handleOptionClick('d1')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([['d2']])
  })

  it('filteredOptions returns all options when no search keyword', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.filteredOptions).toHaveLength(2)
  })

  it('filteredOptions filters by search keyword', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.searchKeyword = 'Device 1'
    expect(state.filteredOptions).toHaveLength(1)
    expect(state.filteredOptions[0].device_name).toBe('Device 1')
  })

  it('handleLoadMore emits loadMore when not loading and has more', () => {
    const wrapper = mountComponent({ loading: false, hasMore: true })
    const state = getSetupState(wrapper)
    state.handleLoadMore()
    expect(wrapper.emitted('loadMore')).toEqual([[]])
  })

  it('handleLoadMore does not emit when loading', () => {
    const wrapper = mountComponent({ loading: true, hasMore: true })
    const state = getSetupState(wrapper)
    state.handleLoadMore()
    expect(wrapper.emitted('loadMore')).toBeUndefined()
  })
})
