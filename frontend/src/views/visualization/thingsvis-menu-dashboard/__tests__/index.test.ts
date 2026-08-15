/**
 * 文件用途: 覆盖测试在可视化场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getDashboard: vi.fn(),
  providerAvailable: true,
  route: { query: { id: 'dash-1' }, params: { dashboardId: 'param-1' }, path: '/menu-dashboard/param-1' }
}))

vi.mock('@/service/visualization-provider/index', () => ({
  getDefaultVisualizationProviderFacade: () => ({
    selectionError: hoisted.providerAvailable ? null : { code: 'provider-unavailable' },
    execute: (operation: (provider: { getDashboard: typeof hoisted.getDashboard }) => Promise<unknown>) =>
      hoisted.providerAvailable
        ? operation({ getDashboard: hoisted.getDashboard })
        : Promise.resolve({ ok: false, error: { code: 'provider-unavailable', message: 'unavailable' } })
  })
}))

vi.mock('@/components/visualization-provider/VisualizationProviderFrame.vue', () => ({
  default: defineComponent({ name: 'VisualizationProviderFrame', props: ['id', 'schema', 'mode'], setup() { return () => h('div') } })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: vi.fn() })
}))

vi.mock('vue-router', () => ({
  useRoute: () => hoisted.route,
  useRouter: () => ({ push: vi.fn(), back: vi.fn() })
}))

// VisualizationProviderFrame is the only visualization component boundary mocked here.

import ThingsVisMenuDashboard from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(ThingsVisMenuDashboard, {
    props,
    global: {
      stubs: {
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NBreadcrumb: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NBreadcrumbItem: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('ThingsVisMenuDashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.providerAvailable = true
    hoisted.route.query = { id: 'dash-1' }
    hoisted.route.params = { dashboardId: 'param-1' }
    hoisted.route.path = '/menu-dashboard/param-1'
    hoisted.getDashboard.mockResolvedValue({ ok: true, data: { name: 'Menu Dashboard' } })
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('loads the dashboard through the external provider and preserves frame props', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.getDashboard).toHaveBeenCalledTimes(1)
    expect(hoisted.getDashboard).toHaveBeenCalledWith('param-1')
    const state = getState(wrapper)
    expect(state.dashboardTitle).toBe('Menu Dashboard')
    expect(state.dashboardSchema).toEqual({ name: 'Menu Dashboard' })
    expect(wrapper.html()).toContain('id="param-1"')
    expect(wrapper.html()).toContain('mode="viewer"')
  })

  it('fails closed without requests or frame mounts when the provider is unavailable', async () => {
    hoisted.providerAvailable = false
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.getDashboard).not.toHaveBeenCalled()
    expect(wrapper.findComponent({ name: 'VisualizationProviderFrame' }).exists()).toBe(false)
  })

  it('ignores an older response that resolves after a newer request', async () => {
    let resolveOlder!: (value: unknown) => void
    hoisted.getDashboard
      .mockImplementationOnce(() => new Promise(resolve => { resolveOlder = resolve }))
      .mockResolvedValueOnce({ ok: true, data: { name: 'New Dashboard' } })
    const wrapper = mountComponent()
    const state = getState(wrapper)

    await state.loadDashboard()
    resolveOlder({ ok: true, data: { name: 'Old Dashboard' } })
    await flushPromises()

    expect(state.dashboardSchema).toEqual({ name: 'New Dashboard' })
    expect(state.dashboardTitle).toBe('New Dashboard')
  })

  it('allows the embedded dashboard area to scroll when auto height exceeds the layout viewport', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.html()).toContain('overflow-auto')
  })

  it('should compute dashboardId from route params first', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.dashboardId).toBe('param-1')
  })

  it('should clear state when dashboardId is empty', async () => {
    hoisted.route.query = {}
    hoisted.route.params = {}
    hoisted.route.path = ''
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.dashboardSchema).toBeNull()
    expect(state.dashboardTitle).toBe('')
  })

  it('should handle load error gracefully', async () => {
    hoisted.getDashboard.mockRejectedValue(new Error('fail'))
    hoisted.route.query = { id: 'test-id' }
    hoisted.route.params = {}
    hoisted.route.path = '/menu-dashboard/test-id'
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    await state.loadDashboard()
    expect(state.dashboardSchema).toBeNull()
    expect(state.dashboardTitle).toBe('')
  })

  it('should extract dashboardId from route path segments', async () => {
    hoisted.route.query = {}
    hoisted.route.params = {}
    hoisted.route.path = '/menu-dashboard/path-fallback-id'
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(hoisted.getDashboard).toHaveBeenCalledWith('path-fallback-id')
    expect(state.dashboardId).toBe('path-fallback-id')
  })
})
