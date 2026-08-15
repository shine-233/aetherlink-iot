/**
 * 文件用途: 覆盖Market Template List在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getMarketTemplates: vi.fn(),
  installFromMarket: vi.fn(),
  isLoggedIn: vi.fn(),
  getToken: vi.fn(),
  clearToken: vi.fn(),
  openMarketLogin: vi.fn()
}))

vi.mock('@/service/api/market', () => ({
  getMarketTemplates: hoisted.getMarketTemplates,
  installFromMarket: hoisted.installFromMarket
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('../../composables/use-market-auth', () => ({
  useMarketAuth: () => ({
    isLoggedIn: hoisted.isLoggedIn,
    getToken: hoisted.getToken,
    clearToken: hoisted.clearToken
  })
}))

vi.mock('@vueuse/core', () => ({
  useDebounceFn: vi.fn((fn: any) => fn)
}))

vi.mock('./market-template-card.vue', () => ({
  default: defineComponent({ props: ['template'], emits: ['install', 'view-detail'], setup(_, { slots }) { return () => h('div', slots.default?.()) } })
}))

vi.mock('./market-template-drawer.vue', () => ({
  default: defineComponent({ props: ['visible', 'templateId'], emits: ['update:visible', 'install'], setup(_, { slots }) { return () => h('div', slots.default?.()) } })
}))

vi.mock('./market-login-modal.vue', () => ({
  default: defineComponent({
    emits: ['login-success'],
    setup(_, { slots, expose }) {
      expose({ open: hoisted.openMarketLogin })
      return () => h('div', slots.default?.())
    }
  })
}))

vi.mock('@vicons/ionicons5', () => ({
  SearchOutline: defineComponent({ setup: () => () => h('span') })
}))

import Component from '../market-template-list.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        MarketLoginModal: false,
        NInput: defineComponent({ props: ['value', 'placeholder', 'clearable'], emits: ['update:value', 'keyup'], setup() { return () => h('input') } }),
        NSelect: defineComponent({ props: ['value', 'options', 'clearable', 'placeholder'], emits: ['update:value'], setup() { return () => h('div') } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NSpin: defineComponent({ props: ['show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NGrid: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NGi: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NEmpty: defineComponent({ setup() { return () => h('div') } }),
        NPagination: defineComponent({ props: ['page', 'pageSize', 'itemCount'], emits: ['update:page'], setup() { return () => h('div') } }),
        NIcon: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default?.()) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config/modules/market-template-list.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getMarketTemplates.mockResolvedValue({ error: null, data: { list: [], total: 0 } })
    hoisted.installFromMarket.mockResolvedValue({ error: null, data: {} })
    hoisted.isLoggedIn.mockReturnValue(false)
    hoisted.getToken.mockReturnValue('')
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes market template filters and fetches first page sorted by latest', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.searchParams).toMatchObject({
      keyword: '',
      category: null,
      sort_by: 'latest',
      page: 1,
      page_size: 12
    })
    expect(state.categoryOptions.map((option: any) => option.value)).toEqual(['IoT', '工业', '农业', '智慧城市', '其他'])
    expect(state.sortOptions.map((option: any) => option.value)).toEqual(['latest', 'hottest'])
    expect(hoisted.getMarketTemplates).toHaveBeenCalledWith({
      page: 1,
      page_size: 12,
      sort_by: 'latest'
    })
    expect(state.templateList).toEqual([])
    expect(state.total).toBe(0)
  })

  it('fetches market templates on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getMarketTemplates).toHaveBeenCalledTimes(1)
  })

  it('populates templateList on successful fetch', async () => {
    hoisted.getMarketTemplates.mockResolvedValue({
      error: null,
      data: { list: [{ id: '1', name: 'Template1' }], total: 1 }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.templateList).toHaveLength(1)
    expect(state.total).toBe(1)
  })

  it('handleSearch resets page and fetches data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    state.handleSearch()
    expect(state.searchParams.page).toBe(1)
    expect(hoisted.getMarketTemplates).toHaveBeenCalledTimes(1)
  })

  it('handleViewDetail sets selectedTemplateId and opens drawer', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleViewDetail('tpl-1')
    expect(state.selectedTemplateId).toBe('tpl-1')
    expect(state.drawerVisible).toBe(true)
  })

  it('handleInstall sets pendingInstallId when not logged in', async () => {
    hoisted.isLoggedIn.mockReturnValue(false)
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleInstall('tpl-1')
    await flushPromises()
    expect(state.pendingInstallId).toBe('tpl-1')
    expect(state.marketLoginVisited).toBe(true)
  })

  it('handleInstall calls doInstall when logged in', async () => {
    hoisted.isLoggedIn.mockReturnValue(true)
    hoisted.getToken.mockReturnValue('token-123')
    hoisted.installFromMarket.mockResolvedValue({ error: null, data: {} })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleInstall('tpl-1')
    expect(hoisted.installFromMarket).toHaveBeenCalledWith({
      market_template_id: 'tpl-1',
      market_token: 'token-123'
    })
  })

  it('doInstall shows success message on success', async () => {
    hoisted.isLoggedIn.mockReturnValue(true)
    hoisted.getToken.mockReturnValue('token-123')
    hoisted.installFromMarket.mockResolvedValue({ error: null, data: {} })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.doInstall('tpl-1')
    expect(window.$message?.success).toHaveBeenCalledTimes(1)
  })

  it('doInstall shows warning for duplicate install', async () => {
    hoisted.isLoggedIn.mockReturnValue(true)
    hoisted.getToken.mockReturnValue('token-123')
    hoisted.installFromMarket.mockResolvedValue({ error: { msg: '已存在' }, data: {} })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.doInstall('tpl-1')
    expect(window.$message?.warning).toHaveBeenCalledTimes(1)
  })

  it('doInstall clears token on 401 error', async () => {
    hoisted.isLoggedIn.mockReturnValue(true)
    hoisted.getToken.mockReturnValue('token-123')
    const error401: any = new Error('Unauthorized')
    error401.response = { status: 401 }
    hoisted.installFromMarket.mockRejectedValue(error401)
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.doInstall('tpl-1')
    await flushPromises()
    expect(hoisted.clearToken).toHaveBeenCalledTimes(1)
    expect(state.marketLoginVisited).toBe(true)
  })

  it('does not throw when the login ref exists before its open contract is ready', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.marketLoginRef = {}
    await expect(state.openMarketLoginModal()).resolves.toBeUndefined()
    expect(state.marketLoginVisited).toBe(true)
  })

  it('opens the login modal when its exposed open contract is ready', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.marketLoginRef = { open: hoisted.openMarketLogin }
    await state.openMarketLoginModal()
    expect(hoisted.openMarketLogin).toHaveBeenCalledTimes(1)
  })

  it('onMarketLoginSuccess calls doInstall with pending id', async () => {
    hoisted.isLoggedIn.mockReturnValue(true)
    hoisted.getToken.mockReturnValue('token-123')
    hoisted.installFromMarket.mockResolvedValue({ error: null, data: {} })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.pendingInstallId = 'tpl-pending'
    await state.onMarketLoginSuccess()
    expect(hoisted.installFromMarket).toHaveBeenCalledTimes(1)
    expect(state.pendingInstallId).toBe('')
  })

  it('emits installed on successful install', async () => {
    hoisted.isLoggedIn.mockReturnValue(true)
    hoisted.getToken.mockReturnValue('token-123')
    hoisted.installFromMarket.mockResolvedValue({ error: null, data: {} })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.doInstall('tpl-1')
    expect(wrapper.emitted('installed')).toEqual([[]])
  })
})
