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
  routeQuery: { id: 'dash-1', projectId: 'proj-1' }
}))

vi.mock('@/service/visualization-provider/index', () => ({
  getDefaultVisualizationProviderFacade: () => ({
    selectionError: null,
    execute: (operation: (provider: { getDashboard: typeof hoisted.getDashboard }) => Promise<unknown>) =>
      operation({ getDashboard: hoisted.getDashboard })
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: vi.fn() })
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: hoisted.routeQuery }),
  useRouter: () => ({ push: vi.fn(), back: vi.fn() })
}))

vi.mock('@/components/visualization-provider/VisualizationProviderFrame.vue', () => ({
  default: defineComponent({ name: 'VisualizationProviderFrame', props: ['id', 'schema', 'mode'], setup() { return () => h('div') } })
}))

import ThingsVisEditor from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []
let consoleWarnSpy: ReturnType<typeof vi.spyOn> | null = null

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(ThingsVisEditor, {
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

describe('ThingsVisEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    hoisted.routeQuery.id = 'dash-1'
    hoisted.routeQuery.projectId = 'proj-1'
    hoisted.getDashboard.mockResolvedValue({ ok: true, data: { name: 'Test Dashboard', projectId: 'proj-1' } })
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
    consoleWarnSpy?.mockRestore()
    consoleWarnSpy = null
  })

  it('should mount and load dashboard info', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.getDashboard).toHaveBeenCalledWith('dash-1')
    const state = getState(wrapper)
    expect(state.projectTitle).toBe('Test Dashboard')
  })

  it('should compute dashboardId from route query', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.dashboardId).toBe('dash-1')
  })

  it('should compute currentProjectId from route query', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.currentProjectId).toBe('proj-1')
  })

  it('should clear state when dashboardId is empty', async () => {
    hoisted.getDashboard.mockResolvedValue({ ok: false, error: { code: 'invalid-response', message: 'invalid' } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    hoisted.routeQuery.id = ''
    await state.loadDashboardInfo()
    expect(state.projectTitle).toBe('')
    expect(state.dashboardSchema).toBeNull()
  })

  it('should make only one request on 401 error', async () => {
    hoisted.getDashboard.mockResolvedValue({
      ok: false,
      error: { code: 'provider-unauthenticated', message: 'unauthenticated', status: 401 }
    })
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.getDashboard).toHaveBeenCalledTimes(1)
    expect(getState(wrapper).dashboardSchema).toBeNull()
  })

  it('ignores an older response that resolves after a newer request', async () => {
    let resolveOlder!: (value: unknown) => void
    hoisted.getDashboard
      .mockImplementationOnce(() => new Promise(resolve => { resolveOlder = resolve }))
      .mockResolvedValueOnce({ ok: true, data: { name: 'New Dashboard', projectId: 'proj-1' } })
    const wrapper = mountComponent()
    const state = getState(wrapper)

    await state.loadDashboardInfo()
    resolveOlder({ ok: true, data: { name: 'Old Dashboard', projectId: 'proj-1' } })
    await flushPromises()

    expect(state.projectTitle).toBe('New Dashboard')
    expect(state.dashboardSchema).toMatchObject({ name: 'New Dashboard' })
  })

  it('should handle load error gracefully', async () => {
    hoisted.getDashboard.mockRejectedValue(new Error('Network error'))
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    hoisted.routeQuery.id = 'test-id'
    await state.loadDashboardInfo()
    expect(state.dashboardSchema).toBeNull()
  })

  it('keeps the editor host container scrollable for dynamic iframe height growth', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    expect(wrapper.html()).toContain('overflow-auto')
  })
})
