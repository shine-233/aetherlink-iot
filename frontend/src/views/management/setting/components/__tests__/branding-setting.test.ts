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
  fetchTenantTranslations: vi.fn(),
  upsertTenantTranslations: vi.fn(),
  deleteTenantTranslations: vi.fn(),
  fetchTenantCustomCSS: vi.fn(),
  upsertTenantCustomCSS: vi.fn(),
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  initSysSetting: vi.fn(),
  initWhitelabelOverrides: vi.fn(),
  updateThemeColors: vi.fn()
}))

vi.mock('@/service/api/setting', () => ({
  fetchThemeSetting: hoisted.fetchThemeSetting,
  editThemeSetting: hoisted.editThemeSetting
}))

vi.mock('@/service/api/whitelabel', () => ({
  fetchTenantTranslations: hoisted.fetchTenantTranslations,
  upsertTenantTranslations: hoisted.upsertTenantTranslations,
  deleteTenantTranslations: hoisted.deleteTenantTranslations,
  fetchTenantCustomCSS: hoisted.fetchTenantCustomCSS,
  upsertTenantCustomCSS: hoisted.upsertTenantCustomCSS
}))

vi.mock('@/store/modules/sys-setting', () => ({
  useSysSettingStore: () => ({
    initSysSetting: hoisted.initSysSetting,
    initWhitelabelOverrides: hoisted.initWhitelabelOverrides
  })
}))

vi.mock('@/store/modules/theme', () => ({
  useThemeStore: () => ({
    updateThemeColors: hoisted.updateThemeColors
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
        NSpin: defineComponent({
          props: { show: Boolean },
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NForm: defineComponent({
          setup(_, { slots }) {
            return () => h('form', slots.default ? slots.default() : [])
          }
        }),
        NFormItem: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NInput: defineComponent({
          props: { value: { default: '' } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NSpace: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NButton: defineComponent({
          emits: ['click'],
          props: { loading: Boolean },
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
          }
        }),
        NDivider: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NDataTable: defineComponent({
          props: { loading: Boolean },
          setup() {
            return () => h('div')
          }
        }),
        NSelect: defineComponent({
          props: { value: { default: '' } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        })
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
    hoisted.fetchTenantTranslations.mockResolvedValue({
      error: null,
      data: {
        total: 1,
        list: [{ id: 'tr-1', tenant_id: 't-1', lang: 'zh-cn', key: 'page.customer.title', value: '客户中心' }]
      }
    })
    hoisted.fetchTenantCustomCSS.mockResolvedValue({
      error: null,
      data: { css: '.app { color: red; }', updated_at: null }
    })
    hoisted.upsertTenantTranslations.mockResolvedValue({ error: null, data: { count: 1 } })
    hoisted.deleteTenantTranslations.mockResolvedValue({ error: null, data: { deleted: 1 } })
    hoisted.upsertTenantCustomCSS.mockResolvedValue({ error: null, data: null })
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
      home_background: 'https://example.com/bg.png',
      theme_color: '',
      favicon: ''
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
      home_background: 'https://example.com/bg.png',
      theme_color: '',
      favicon: ''
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

describe('branding-setting whitelabel entries (TB-47)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchThemeSetting.mockResolvedValue({ error: null, data: { list: [] } })
    hoisted.fetchTenantTranslations.mockResolvedValue({
      error: null,
      data: {
        total: 1,
        list: [{ id: 'tr-1', tenant_id: 't-1', lang: 'zh-cn', key: 'page.customer.title', value: '客户中心' }]
      }
    })
    hoisted.fetchTenantCustomCSS.mockResolvedValue({
      error: null,
      data: { css: '.app { color: red; }', updated_at: null }
    })
    hoisted.upsertTenantTranslations.mockResolvedValue({ error: null, data: { count: 1 } })
    hoisted.deleteTenantTranslations.mockResolvedValue({ error: null, data: { deleted: 1 } })
    hoisted.upsertTenantCustomCSS.mockResolvedValue({ error: null, data: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads translation overrides and custom css on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.fetchTenantTranslations).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchTenantCustomCSS).toHaveBeenCalledTimes(1)
    expect(state.translationRows).toEqual([{ lang: 'zh-cn', key: 'page.customer.title', value: '客户中心' }])
    expect(state.customCSS).toBe('.app { color: red; }')
    expect(state.overridesLoading).toBe(false)
  })

  it('upsertTranslationRow rejects blank key/value without calling the API', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.newTranslation.key = '   '
    state.newTranslation.value = ' '
    await state.upsertTranslationRow()

    expect(hoisted.messageError).toHaveBeenCalledWith('custom.management.branding.translationRowInvalid')
    expect(hoisted.upsertTenantTranslations).toHaveBeenCalledTimes(0)
  })

  it('upsertTranslationRow trims key and posts single item, then reloads', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.fetchTenantTranslations.mockResolvedValue({ error: null, data: { total: 0, list: [] } })
    hoisted.upsertTenantTranslations.mockResolvedValue({ error: null, data: { count: 1 } })
    const state = getSetupState(wrapper)
    state.newTranslation.lang = 'en-us'
    state.newTranslation.key = '  page.device.title  '
    state.newTranslation.value = 'Device'
    await state.upsertTranslationRow()
    await flushPromises()

    expect(hoisted.upsertTenantTranslations).toHaveBeenCalledTimes(1)
    expect(hoisted.upsertTenantTranslations.mock.calls[0][0]).toEqual([
      { lang: 'en-us', key: 'page.device.title', value: 'Device' }
    ])
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('custom.management.branding.translationSaved')
    // 保存后重载覆盖列表，键/值输入框复位
    expect(hoisted.fetchTenantTranslations).toHaveBeenCalledTimes(1)
    expect(state.newTranslation.key).toBe('')
    expect(state.newTranslation.value).toBe('')
  })

  it('removeTranslationRow deletes by lang+key and reloads', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.fetchTenantTranslations.mockResolvedValue({ error: null, data: { total: 0, list: [] } })
    hoisted.deleteTenantTranslations.mockResolvedValue({ error: null, data: { deleted: 1 } })
    const state = getSetupState(wrapper)
    await state.removeTranslationRow({ lang: 'zh-cn', key: 'page.customer.title', value: '客户中心' })
    await flushPromises()

    expect(hoisted.deleteTenantTranslations).toHaveBeenCalledTimes(1)
    expect(hoisted.deleteTenantTranslations.mock.calls[0][0]).toEqual([
      { lang: 'zh-cn', key: 'page.customer.title' }
    ])
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('custom.management.branding.translationDeleted')
  })

  it('saveCustomCSS trims payload and refreshes whitelabel runtime for instant injection', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.upsertTenantCustomCSS.mockResolvedValue({ error: null, data: null })
    const state = getSetupState(wrapper)
    state.customCSS = '  .app { color: blue; }  '
    await state.saveCustomCSS()
    await flushPromises()

    expect(hoisted.upsertTenantCustomCSS).toHaveBeenCalledTimes(1)
    expect(hoisted.upsertTenantCustomCSS.mock.calls[0][0]).toBe('.app { color: blue; }')
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('custom.management.branding.customCssSaved')
    // 保存后重新拉取覆盖，让新 CSS 经 sys-setting store 的 textContent 注入立即生效
    expect(hoisted.initWhitelabelOverrides).toHaveBeenCalledTimes(1)
    expect(state.cssSaving).toBe(false)
  })

  it('clearing css saves empty string (backend clear semantics)', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.upsertTenantCustomCSS.mockResolvedValue({ error: null, data: null })
    const state = getSetupState(wrapper)
    state.customCSS = '   '
    await state.saveCustomCSS()

    expect(hoisted.upsertTenantCustomCSS.mock.calls[0][0]).toBe('')
  })

  it('translation load failure surfaces an error message without crashing', async () => {
    hoisted.fetchTenantTranslations.mockResolvedValue({ error: new Error('boom'), data: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.messageError).toHaveBeenCalledWith('custom.management.branding.overridesLoadFailed')
    expect(state.translationRows).toEqual([])
    expect(state.overridesLoading).toBe(false)
  })
})
