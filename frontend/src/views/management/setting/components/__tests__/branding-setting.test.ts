/**
 * 文件用途：覆盖 branding-setting 在 系统与账号设置 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchThemeSetting: vi.fn(),
  editThemeSetting: vi.fn(),
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  initSysSetting: vi.fn()
}))

vi.mock('@/service/api/setting', () => ({
  fetchThemeSetting: hoisted.fetchThemeSetting,
  editThemeSetting: hoisted.editThemeSetting
}))

vi.mock('@/store/modules/sys-setting', () => ({
  useSysSettingStore: () => ({
    initSysSetting: hoisted.initSysSetting
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/discrete', () => ({
  message: {
    success: hoisted.messageSuccess,
    error: hoisted.messageError
  }
}))

import BrandingSetting from '../branding-setting.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(BrandingSetting, {
    global: {
      stubs: {
        NSpin: defineComponent({ props: { show: Boolean }, setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], props: { loading: Boolean }, setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('management/setting/components/branding-setting.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchThemeSetting.mockResolvedValue({
      error: null,
      data: {
        list: [
          {
            id: 't-1',
            system_name: 'AetherLink IoT',
            logo_cache: 'https://example.com/favicon.ico',
            logo_background: 'https://example.com/logo.png',
            logo_loading: 'https://example.com/loading.png',
            home_background: 'https://example.com/bg.png'
          }
        ]
      }
    })
    hoisted.editThemeSetting.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads branding settings into the editable form on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.fetchThemeSetting).toHaveBeenCalledTimes(1)
    expect(state.form).toEqual({
      id: 't-1',
      system_name: 'AetherLink IoT',
      logo_cache: 'https://example.com/favicon.ico',
      logo_background: 'https://example.com/logo.png',
      logo_loading: 'https://example.com/loading.png',
      home_background: 'https://example.com/bg.png'
    })
    expect(state.loading).toBe(false)
  })

  it('calls loadBrandingSetting on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.fetchThemeSetting).toHaveBeenCalledTimes(1)
  })

  it('assignForm populates form with record data', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.assignForm({
      id: 't-1',
      system_name: 'Test',
      logo_cache: 'cache',
      logo_background: 'bg',
      logo_loading: 'loading',
      home_background: 'home'
    })
    expect(state.form.id).toBe('t-1')
    expect(state.form.system_name).toBe('Test')
    expect(state.form.logo_cache).toBe('cache')
    expect(state.form.logo_background).toBe('bg')
    expect(state.form.logo_loading).toBe('loading')
    expect(state.form.home_background).toBe('home')
  })

  it('assignForm handles undefined record', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.assignForm(undefined)
    expect(state.form.id).toBe('')
    expect(state.form.system_name).toBe('')
    expect(state.form.logo_cache).toBe('')
  })

  it('loadBrandingSetting fetches and assigns form data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.loading).toBe(false)
    expect(state.form.id).toBe('t-1')
    expect(state.form.system_name).toBe('AetherLink IoT')
  })

  it('saveBrandingSetting shows error when form id is empty', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.form.id = ''
    await state.saveBrandingSetting()
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.management.branding.missingRecord')
    expect(hoisted.editThemeSetting).toHaveBeenCalledTimes(0)
  })

  it('saveBrandingSetting calls editThemeSetting and initSysSetting on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editThemeSetting.mockResolvedValue({ error: null })
    const state = getSetupState(wrapper)
    await state.saveBrandingSetting()
    await flushPromises()
    expect(hoisted.editThemeSetting).toHaveBeenCalledTimes(1)
    expect(hoisted.editThemeSetting).toHaveBeenCalledWith({
      id: 't-1',
      system_name: 'AetherLink IoT',
      logo_cache: 'https://example.com/favicon.ico',
      logo_background: 'https://example.com/logo.png',
      logo_loading: 'https://example.com/loading.png',
      home_background: 'https://example.com/bg.png'
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('custom.management.branding.saved')
    expect(hoisted.initSysSetting).toHaveBeenCalledTimes(1)
  })

  it('saveBrandingSetting does not call initSysSetting when API returns error', async () => {
    hoisted.editThemeSetting.mockResolvedValue({ error: 'fail' })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editThemeSetting.mockResolvedValue({ error: 'fail' })
    const state = getSetupState(wrapper)
    await state.saveBrandingSetting()
    await flushPromises()
    expect(hoisted.initSysSetting).toHaveBeenCalledTimes(0)
  })

  it('saveBrandingSetting trims string values before submit', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editThemeSetting.mockResolvedValue({ error: null })
    const state = getSetupState(wrapper)
    state.form.system_name = '  AetherLink IoT  '
    state.form.logo_cache = '  cache  '
    await state.saveBrandingSetting()
    await flushPromises()
    const callArgs = hoisted.editThemeSetting.mock.calls[0][0]
    expect(callArgs.system_name).toBe('AetherLink IoT')
    expect(callArgs.logo_cache).toBe('cache')
  })

  it('saveBrandingSetting sets saving to false after completion', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.editThemeSetting.mockResolvedValue({ error: null })
    const state = getSetupState(wrapper)
    await state.saveBrandingSetting()
    await flushPromises()
    expect(state.saving).toBe(false)
  })

  it('loading is set to false after loadBrandingSetting completes', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.loading).toBe(false)
  })
})
