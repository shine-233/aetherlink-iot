/**
 * 文件用途：覆盖 function-setting 在 系统与账号设置 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getFunction: vi.fn(),
  editFunction: vi.fn()
}))

vi.mock('@/service/api/setting', () => ({
  getFunction: hoisted.getFunction,
  editFunction: hoisted.editFunction
}))

import FunctionSetting from '../function-setting.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(FunctionSetting, {
    global: {
      stubs: {
        NFlex: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NForm: defineComponent({
          props: { labelPlacement: String, labelWidth: [String, Number] },
          setup(_, { slots }) {
            return () => h('form', slots.default ? slots.default() : [])
          }
        }),
        NGrid: defineComponent({
          props: { cols: Number, xGap: [String, Number] },
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NFormItemGridItem: defineComponent({
          props: { span: Number, label: String },
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NSwitch: defineComponent({
          props: { value: Boolean },
          emits: ['update:value', 'change'],
          setup(_, { emit, attrs }) {
            return () =>
              h('input', {
                type: 'checkbox',
                checked: attrs.value,
                onChange: (e: Event) => {
                  const val = (e.target as HTMLInputElement).checked
                  emit('update:value', val)
                  emit('change', val)
                }
              })
          }
        }),
        NSpace: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) =>
  wrapper.vm.$.setupState as Record<string, any>

const mockFunctionData = (overrides: Record<string, any>[] = []) => {
  const defaults = [
    { id: 1, description: '功能A', enable_flag: 'enable' },
    { id: 2, description: '功能B', enable_flag: 'disable' }
  ]
  if (overrides.length > 0) return overrides
  return defaults
}

describe('management/setting/components/function-setting.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getFunction.mockResolvedValue({
      data: mockFunctionData()
    })
    hoisted.editFunction.mockResolvedValue({ error: null })
    localStorage.clear()
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads function switches into form options on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.getFunction).toHaveBeenCalledTimes(1)
    expect(state.funcOptions).toEqual([
      { id: '1', description: '功能A', enable_flag: 'enable', value: true },
      { id: '2', description: '功能B', enable_flag: 'disable', value: false }
    ])
    expect(localStorage.getItem('enableZcAndYzm')).toBe(JSON.stringify(mockFunctionData()))
  })

  it('calls getFunction on init', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getFunction).toHaveBeenCalledTimes(1)
  })

  it('populates funcOptions with mapped data on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.funcOptions).toHaveLength(2)
    expect(state.funcOptions[0].id).toBe('1')
    expect(state.funcOptions[0].description).toBe('功能A')
    expect(state.funcOptions[0].value).toBe(true)
    expect(state.funcOptions[1].id).toBe('2')
    expect(state.funcOptions[1].description).toBe('功能B')
    expect(state.funcOptions[1].value).toBe(false)
  })

  it('maps enable_flag "enable" to true and others to false', async () => {
    hoisted.getFunction.mockResolvedValue({
      data: [
        { id: 10, description: 'Enabled', enable_flag: 'enable' },
        { id: 20, description: 'Disabled', enable_flag: 'disable' },
        { id: 30, description: 'Other', enable_flag: 'unknown' }
      ]
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.funcOptions[0].value).toBe(true)
    expect(state.funcOptions[1].value).toBe(false)
    expect(state.funcOptions[2].value).toBe(false)
  })

  it('stores function data in localStorage', async () => {
    mountComponent()
    await flushPromises()
    const stored = localStorage.getItem('enableZcAndYzm')
    expect(stored).toBe(JSON.stringify(mockFunctionData()))
    const parsed = JSON.parse(stored!)
    expect(parsed).toEqual(mockFunctionData())
  })

  it('changeFunc calls editFunction with correct function_id', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const item = state.funcOptions[0]
    await state.changeFunc(item)
    await flushPromises()
    expect(hoisted.editFunction).toHaveBeenCalledWith({ function_id: '1' })
  })

  it('changeFunc refreshes funcOptions on editFunction success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.getFunction.mockResolvedValue({
      data: [{ id: 1, description: '功能A', enable_flag: 'disable' }]
    })
    hoisted.editFunction.mockResolvedValue({ error: null })
    const state = getSetupState(wrapper)
    await state.changeFunc(state.funcOptions[0])
    await flushPromises()
    expect(hoisted.getFunction).toHaveBeenCalledTimes(1)
    expect(state.funcOptions[0].value).toBe(false)
  })

  it('changeFunc does not refresh when editFunction returns error', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editFunction.mockResolvedValue({ error: 'fail' })
    const state = getSetupState(wrapper)
    await state.changeFunc(state.funcOptions[0])
    await flushPromises()
    expect(hoisted.getFunction).toHaveBeenCalledTimes(0)
  })

  it('handles getFunction returning no data gracefully', async () => {
    hoisted.getFunction.mockResolvedValue({ data: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.funcOptions).toHaveLength(0)
  })

  it('handles null/undefined id and description in function data', async () => {
    hoisted.getFunction.mockResolvedValue({
      data: [{ id: null, description: null, enable_flag: 'enable' }]
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.funcOptions[0].id).toBe('')
    expect(state.funcOptions[0].description).toBe('')
  })
})
