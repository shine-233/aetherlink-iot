/**
 * 文件用途：覆盖 column-setting 在 后台用户管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-draggable-plus', () => ({
  VueDraggable: defineComponent({
    name: 'VueDraggableStub',
    props: { modelValue: { type: Array, default: () => [] } },
    setup(_, { slots }) {
      return () => h('div', { class: 'vue-draggable-plus-stub' }, slots.default ? slots.default() : [])
    }
  })
}))

import ColumnSetting from '../column-setting.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const sampleColumns = [
  { key: 'name', title: 'Name' },
  { key: 'email', title: 'Email' },
  { key: 'status', title: 'Status' }
] as any

const mountComponent = (props: Record<string, any> = {}) => {
  const wrapper = shallowMount(ColumnSetting, {
    props: {
      columns: sampleColumns,
      ...props
    },
    global: {
      stubs: {
        NPopover: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NButton: defineComponent({
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
          }
        }),
        NCheckbox: defineComponent({
          props: { checked: { default: false } },
          emits: ['update:checked'],
          setup(_, { slots }) {
            return () => h('label', slots.default ? slots.default() : [])
          }
        }),
        IconAntDesignSettingOutlined: true,
        IconMdiDrag: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('management/user/components/column-setting.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds the draggable column list with stable item keys', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    const draggable = wrapper.getComponent({ name: 'VueDraggableStub' })

    expect(draggable.props('modelValue')).toBe(state.list)
    expect(state.list.map((item: any) => ({ key: item.key, title: item.title, checked: item.checked }))).toEqual([
      { key: 'name', title: 'Name', checked: true },
      { key: 'email', title: 'Email', checked: true },
      { key: 'status', title: 'Status', checked: true }
    ])
  })

  it('initList maps columns to list with checked true', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.list).toHaveLength(3)
    expect(state.list[0].checked).toBe(true)
    expect(state.list[0].key).toBe('name')
    expect(state.list[1].checked).toBe(true)
    expect(state.list[2].checked).toBe(true)
  })

  it('initList returns empty array when columns is empty', () => {
    const wrapper = mountComponent({ columns: [] })
    const state = getSetupState(wrapper)
    expect(state.list).toHaveLength(0)
  })

  it('emits update:columns with filtered columns on list change', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.list[0].checked = false
    await flushPromises()
    const emitted = wrapper.emitted('update:columns')
    expect(emitted).toHaveLength(1)
    expect(emitted![0][0]).toEqual([
      { key: 'email', title: 'Email' },
      { key: 'status', title: 'Status' }
    ])
  })

  it('emitted columns do not contain checked property', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.list[0].checked = false
    await flushPromises()
    const emitted = wrapper.emitted('update:columns')
    const lastEmitted = emitted![emitted!.length - 1][0] as any[]
    expect(lastEmitted[0].checked).toBeUndefined()
  })

  it('list reflects prop columns initially', () => {
    const wrapper = mountComponent({ columns: [{ key: 'id', title: 'ID' }] as any })
    const state = getSetupState(wrapper)
    expect(state.list).toHaveLength(1)
    expect(state.list[0].key).toBe('id')
  })
})
