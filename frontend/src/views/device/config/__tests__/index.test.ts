/**
 * 文件用途: 覆盖测试在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceConfig: vi.fn(),
  routerPushByKey: vi.fn(),
  routerPush: vi.fn(),
  getPlatformApiBaseUrl: vi.fn(),
  marketLoginOpen: vi.fn(),
  publishConfirmOpen: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceConfig: hoisted.deviceConfig
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: hoisted.routerPushByKey })
}))

vi.mock('@/utils/common/tool', () => ({
  getPlatformApiBaseUrl: hoisted.getPlatformApiBaseUrl
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: hoisted.routerPush }),
  useRoute: () => ({ query: {}, path: '/device/config' })
}))

vi.mock('naive-ui', () => ({
  NTabs: defineComponent({ props: ['value'], emits: ['update:value'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NTabPane: defineComponent({ props: ['name', 'tab'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NInput: defineComponent({ props: ['value', 'placeholder'], emits: ['update:value', 'clear', 'keydown'], setup(_, { slots, emit }) { return () => h('input', { value: _.value, onInput: (e: any) => emit('update:value', e.target.value), onKeydown: (e: any) => emit('keydown', e) }) } }),
  NIcon: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
  NPagination: defineComponent({ props: ['page', 'pageSize', 'itemCount'], emits: ['update:page', 'update:page-size'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NDataTable: defineComponent({ props: ['columns', 'data', 'loading'], setup() { return () => h('table') } }),
  NTag: defineComponent({ props: ['type'], setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
  NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NEmpty: defineComponent({ props: ['description', 'size'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NDropdown: defineComponent({ props: ['options', 'trigger', 'placement'], emits: ['select'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NTooltip: defineComponent({ props: ['disabled', 'trigger'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NGrid: defineComponent({ props: ['cols'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NGi: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NSpin: defineComponent({ props: ['show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } })
}))

vi.mock('@/components/list-page/index.vue', () => ({
  default: defineComponent({ emits: ['add-new', 'query', 'reset', 'refresh'], setup(_, { slots }) { return () => h('div', slots.default?.()) } })
}))

vi.mock('@/components/dev-card-item/index.vue', () => ({
  default: defineComponent({ props: ['title', 'footerText', 'subtitle', 'deviceConfigId', 'isStatus'], emits: ['click-card'], setup(_, { slots }) { return () => h('div', slots.default?.()) } })
}))

vi.mock('@/components/custom/svg-icon.vue', () => ({
  default: defineComponent({ props: ['localIcon'], setup() { return () => h('span') } })
}))

vi.mock('../modules/market-login-modal.vue', () => ({
  default: defineComponent({
    emits: ['login-success'],
    setup(_, { slots, expose }) {
      expose({
        open: hoisted.marketLoginOpen
      })
      return () => h('div', slots.default?.())
    }
  })
}))

vi.mock('../modules/publish-confirm-modal.vue', () => ({
  default: defineComponent({
    emits: ['publish-success'],
    setup(_, { slots, expose }) {
      expose({
        open: hoisted.publishConfirmOpen
      })
      return () => h('div', slots.default?.())
    }
  })
}))

vi.mock('../modules/market-template-list.vue', () => ({
  default: defineComponent({ emits: ['installed'], setup(_, { slots }) { return () => h('div', slots.default?.()) } })
}))

vi.mock('@vicons/ionicons5', () => ({
  SearchOutline: defineComponent({ setup() { return () => h('span') } }),
  ListOutline: defineComponent({ setup() { return () => h('span') } }),
  GridOutline: defineComponent({ setup() { return () => h('span') } }),
  EllipsisHorizontal: defineComponent({ setup() { return () => h('span') } })
}))

import Component from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        MarketLoginModal: defineComponent({
          setup(_, { expose }) {
            expose({
              open: hoisted.marketLoginOpen
            })
            return () => h('div')
          }
        }),
        PublishConfirmModal: defineComponent({
          setup(_, { expose }) {
            expose({
              open: hoisted.publishConfirmOpen
            })
            return () => h('div')
          }
        }),
        MarketTemplateList: defineComponent({ setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceConfig.mockResolvedValue({ error: null, data: { list: [], total: 0 } })
    hoisted.getPlatformApiBaseUrl.mockReturnValue('http://localhost/api/v1')
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes the local config list tab with first-page query contract', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.activeTab).toBe('local')
    expect(state.queryData).toEqual({
      page: 1,
      page_size: 10,
      name: ''
    })
    expect(hoisted.deviceConfig).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      name: ''
    })
    expect(state.deviceConfigList).toEqual([])
    expect(state.dataTotal).toBe(0)
    expect(state.availableViews.map((view: any) => view.key)).toEqual(['card', 'list'])
  })

  it('calls deviceConfig on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceConfig).toHaveBeenCalledTimes(1)
  })

  it('populates deviceConfigList and dataTotal on successful getData', async () => {
    hoisted.deviceConfig.mockResolvedValue({
      error: null,
      data: { list: [{ id: '1', name: 'Config1', device_type: '1', device_count: 5 }], total: 1 }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.deviceConfigList).toHaveLength(1)
    expect(state.dataTotal).toBe(1)
  })

  it('handleQuery resets page and calls getData', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.queryData.page = 3
    vi.clearAllMocks()
    await state.handleQuery()
    expect(state.queryData.page).toBe(1)
    expect(hoisted.deviceConfig).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfig).toHaveBeenCalledWith({ page: 1, page_size: 10, name: '' })
  })

  it('handleReset resets page and name, then calls getData', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.queryData.name = 'test'
    state.queryData.page = 5
    vi.clearAllMocks()
    await state.handleReset()
    expect(state.queryData.page).toBe(1)
    expect(state.queryData.name).toBe('')
    expect(hoisted.deviceConfig).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfig).toHaveBeenCalledWith({ page: 1, page_size: 10, name: '' })
  })

  it('handleAddNew calls routerPushByKey with device_config-edit', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleAddNew()
    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('device_config-edit')
  })

  it('goToDetail pushes to config-detail with id', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.goToDetail('cfg-123')
    expect(hoisted.routerPush).toHaveBeenCalledWith({ path: '/device/config-detail', query: { id: 'cfg-123' } })
  })

  it('handleEdit calls routerPushByKey with device_config-edit and id query', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleEdit('cfg-456')
    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('device_config-edit', { query: { id: 'cfg-456' } })
  })

  it('handlePageChange updates page and calls getData', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    state.handlePageChange(3)
    expect(state.queryData.page).toBe(3)
    expect(hoisted.deviceConfig).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfig).toHaveBeenCalledWith({ page: 3, page_size: 10, name: '' })
  })

  it('handlePageSizeChange updates page_size and resets page', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    state.handlePageSizeChange(20)
    expect(state.queryData.page_size).toBe(20)
    expect(state.queryData.page).toBe(1)
    expect(hoisted.deviceConfig).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfig).toHaveBeenCalledWith({ page: 1, page_size: 20, name: '' })
  })

  it('handleRefresh calls getData', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getSetupState(wrapper)
    state.handleRefresh()
    expect(hoisted.deviceConfig).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfig).toHaveBeenCalledWith({ page: 1, page_size: 10, name: '' })
  })

  it('handlePublishToMarket shows warning when deviceConfigId is empty', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handlePublishToMarket('')
    expect(window.$message?.warning).toHaveBeenCalledTimes(1)
  })

  it('handlePublishToMarket opens login modal when no market_token', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    sessionStorage.removeItem('market_token')
    await state.handlePublishToMarket('cfg-1', 'TestConfig')
    expect(state.pendingPublishId).toBe('cfg-1')
    expect(state.pendingPublishName).toBe('TestConfig')
    expect(state.marketLoginVisited).toBe(true)
  })

  it('handlePublishToMarket opens publish confirm when market_token exists', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    sessionStorage.setItem('market_token', 'test-token')
    await state.handlePublishToMarket('cfg-1', 'TestConfig')
    expect(state.publishConfirmVisited).toBe(true)
    expect(state.pendingPublishId).toBe('')
    sessionStorage.removeItem('market_token')
  })

  it('onMarketLoginSuccess opens publish confirm with pending data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.pendingPublishId = 'cfg-pending'
    state.pendingPublishName = 'PendingConfig'
    await state.onMarketLoginSuccess()
    expect(state.publishConfirmVisited).toBe(true)
    expect(state.pendingPublishId).toBe('')
    expect(state.pendingPublishName).toBe('')
  })

  it('handleInstalled switches to local tab and refreshes data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.activeTab = 'market'
    vi.clearAllMocks()
    state.handleInstalled()
    expect(state.activeTab).toBe('local')
    expect(hoisted.deviceConfig).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfig).toHaveBeenCalledWith({ page: 1, page_size: 10, name: '' })
  })

  it('getDeviceIconName returns correct icon for device types', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.getDeviceIconName('1')).toBe('direct')
    expect(state.getDeviceIconName('2')).toBe('gateway')
    expect(state.getDeviceIconName('3')).toBe('subdevice')
    expect(state.getDeviceIconName('99')).toBe('defaultdevice')
  })

  it('getConfigImageUrl returns empty string for no imageUrl', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.getConfigImageUrl()).toBe('')
    expect(state.getConfigImageUrl('')).toBe('')
  })

  it('getConfigImageUrl returns original URL for https URLs', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.getConfigImageUrl('https://example.com/img.png')).toBe('https://example.com/img.png')
  })

  it('getConfigImageUrl constructs URL for relative paths', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const result = state.getConfigImageUrl('/uploads/img.png')
    expect(result).toContain('/uploads/img.png')
  })

  it('loading state is managed correctly during getData', async () => {
    let resolvePromise: (value: any) => void
    hoisted.deviceConfig.mockImplementation(() => new Promise(resolve => { resolvePromise = resolve }))
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.loading).toBe(true)
    resolvePromise!({ error: null, data: { list: [], total: 0 } })
    await flushPromises()
    expect(state.loading).toBe(false)
  })
})
