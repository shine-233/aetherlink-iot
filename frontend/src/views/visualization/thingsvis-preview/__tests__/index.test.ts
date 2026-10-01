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
  getDashboardsByShareTokens: vi.fn(),
  providerAvailable: true,
  route: { query: { id: 'dash-1' }, params: {} }
}))

vi.mock('@/service/visualization-provider/index', () => ({
  getDefaultVisualizationProviderFacade: () => ({
    selectionError: hoisted.providerAvailable ? null : { code: 'provider-unavailable' },
    execute: (operation: (provider: Record<string, unknown>) => Promise<unknown>) =>
      hoisted.providerAvailable
        ? operation({ getDashboard: hoisted.getDashboard, getDashboardsByShareTokens: hoisted.getDashboardsByShareTokens })
        : Promise.resolve({ ok: false, error: { code: 'provider-unavailable', message: 'unavailable' } })
  })
}))

vi.mock('@/components/visualization-provider/VisualizationProviderFrame.vue', () => ({
  default: defineComponent({
    name: 'VisualizationProviderFrame',
    props: ['id', 'schema', 'mode'],
    setup() {
      return () => h('div')
    }
  })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => hoisted.route,
  useRouter: () => ({ push: vi.fn(), back: vi.fn() })
}))

// VisualizationProviderFrame is the only visualization component boundary mocked here.

import ThingsVisPreview from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(ThingsVisPreview, {
    props,
    global: {
      stubs: {}
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('ThingsVisPreview', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.providerAvailable = true
    hoisted.route.query = { id: 'dash-1' }
    hoisted.route.params = {}
    hoisted.getDashboard.mockResolvedValue({ ok: true, data: { name: 'Preview Dashboard' } })
    hoisted.getDashboardsByShareTokens.mockResolvedValue({
      ok: true,
      data: {
        items: [
          { id: 'board-1', name: 'Wall A' },
          { id: 'board-2', name: 'Wall B' }
        ],
        missingTokens: []
      }
    })
  })

  afterEach(() => {
    mountedWrappers.forEach((w) => w.unmount())
    mountedWrappers.length = 0
    vi.useRealTimers()
  })

  it('loads the dashboard through the external provider', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.getDashboard).toHaveBeenCalledWith('dash-1')
    const state = getState(wrapper)
    expect(state.dashboardSchema).toEqual({ name: 'Preview Dashboard' })
  })

  it('fails closed without requests or frame mounts when the provider is unavailable', async () => {
    vi.useFakeTimers()
    hoisted.providerAvailable = false
    const wrapper = mountComponent()
    await flushPromises()
    vi.runAllTimers()
    await flushPromises()

    expect(hoisted.getDashboard).not.toHaveBeenCalled()
    expect(wrapper.findComponent({ name: 'VisualizationProviderFrame' }).exists()).toBe(false)
  })

  it('ignores an older response that resolves after a newer request', async () => {
    let resolveOlder!: (value: unknown) => void
    hoisted.getDashboard
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveOlder = resolve
          })
      )
      .mockResolvedValueOnce({ ok: true, data: { name: 'New Dashboard' } })
    const wrapper = mountComponent()
    const state = getState(wrapper)

    await state.loadDashboard()
    resolveOlder({ ok: true, data: { name: 'Old Dashboard' } })
    await flushPromises()

    expect(state.dashboardSchema).toEqual({ name: 'New Dashboard' })
    expect(document.title).toContain('New Dashboard')
  })

  it('preserves idle mounting and viewer frame props', async () => {
    vi.useFakeTimers()
    const wrapper = mountComponent()
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VisualizationProviderFrame' }).exists()).toBe(false)

    vi.runAllTimers()
    await flushPromises()

    expect(wrapper.html()).toContain('overflow-auto')
    expect(wrapper.html()).toContain('id="dash-1"')
    expect(wrapper.html()).toContain('mode="viewer"')
    expect(getState(wrapper).dashboardSchema).toEqual({ name: 'Preview Dashboard' })
  })

  it('should compute dashboardId from route query', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.dashboardId).toBe('dash-1')
  })

  it('should clear dashboard schema when dashboardId is empty', async () => {
    hoisted.route.query = {}
    hoisted.route.params = {}
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    await state.loadDashboard()
    expect(state.dashboardSchema).toBeNull()
  })

  it('should handle load error gracefully', async () => {
    hoisted.getDashboard.mockRejectedValue(new Error('fail'))
    hoisted.route.query = { id: 'test-id' }
    hoisted.route.params = {}
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    await state.loadDashboard()
    expect(state.dashboardSchema).toBeNull()
  })

  it('should set document title on load', async () => {
    const originalTitle = document.title
    const wrapper = mountComponent()
    await flushPromises()
    expect(document.title).toContain('Preview Dashboard')
    document.title = originalTitle
  })

  // ---- TP-22 大屏轮播（?tokens=...&interval=...） ----

  it('loads a carousel playlist from share tokens and rotates on the interval', async () => {
    vi.useFakeTimers()
    hoisted.route.query = { tokens: 'tok-a, tok-b', interval: '5' }
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.getDashboard).not.toHaveBeenCalled()
    expect(hoisted.getDashboardsByShareTokens).toHaveBeenCalledTimes(1)
    expect(hoisted.getDashboardsByShareTokens).toHaveBeenCalledWith(['tok-a', 'tok-b'])
    expect(wrapper.find('[data-testid="tv-carousel-position"]').text()).toBe('1 / 2')

    // 到点切换第二屏；ref 更新后等一次微任务 flush 再断言 DOM。
    vi.advanceTimersByTime(5000)
    await flushPromises()
    expect(wrapper.find('[data-testid="tv-carousel-position"]').text()).toBe('2 / 2')

    vi.advanceTimersByTime(5000)
    await flushPromises()
    expect(wrapper.find('[data-testid="tv-carousel-position"]').text()).toBe('1 / 2')
    expect(getState(wrapper).carouselActiveSchema).toMatchObject({ id: 'board-1' })
  })

  it('surfaces unavailable tokens while still playing the resolvable boards', async () => {
    vi.useFakeTimers()
    hoisted.route.query = { tokens: 'tok-good,tok-gone' }
    hoisted.getDashboardsByShareTokens.mockResolvedValue({
      ok: true,
      data: { items: [{ id: 'board-1', name: 'Wall A' }], missingTokens: ['tok-gone'] }
    })
    const wrapper = mountComponent()
    await flushPromises()

    expect(wrapper.find('[data-testid="tv-carousel-missing"]').exists()).toBe(true)
    // $t 在本测试里以 key 透传：出现 key 即证明警示绑定的是缺失计数文案（带 count 参数）。
    expect(wrapper.find('[data-testid="tv-carousel-missing"]').text()).toContain('carouselMissingTokens')
    expect(wrapper.find('[data-testid="tv-carousel-position"]').exists()).toBe(true)
  })

  it('shows the playlist error state when the provider cannot resolve the carousel', async () => {
    vi.useFakeTimers()
    hoisted.route.query = { tokens: 'tok-a' }
    hoisted.getDashboardsByShareTokens.mockResolvedValue({
      ok: false,
      error: { code: 'provider-failure', message: 'batch failed' }
    })
    const wrapper = mountComponent()
    await flushPromises()

    expect(wrapper.find('[data-testid="tv-carousel-error"]').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'VisualizationProviderFrame' }).exists()).toBe(false)
  })

  it('renders the fullscreen control in carousel mode', async () => {
    vi.useFakeTimers()
    hoisted.route.query = { tokens: 'tok-a,tok-b' }
    const wrapper = mountComponent()
    await flushPromises()

    const button = wrapper.find('[data-testid="tv-carousel-fullscreen"]')
    expect(button.exists()).toBe(true)
    // $t 在本测试里以 key 透传：出现 key 即证明按钮绑定的是全屏文案键。
    expect(button.text()).toContain('carouselFullscreen')
  })
})
