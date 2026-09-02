import { computed, defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchCompatHomeConfig: vi.fn(),
  loadVisualizationHomeDashboard: vi.fn(),
  probeVisualizationHomeDashboard: vi.fn(),
  readThingsVisHomeCache: vi.fn(),
  writeThingsVisHomeCache: vi.fn(),
  clearThingsVisHomeCache: vi.fn(),
  isSysAdminUser: vi.fn(),
  authUserInfo: { userName: 'admin' }
}))

vi.mock('@/service/api', () => ({
  fetchCompatHomeConfig: hoisted.fetchCompatHomeConfig
}))

vi.mock('@/service/visualization-provider/home-dashboard', () => ({
  loadVisualizationHomeDashboard: hoisted.loadVisualizationHomeDashboard,
  probeVisualizationHomeDashboard: hoisted.probeVisualizationHomeDashboard
}))

vi.mock('@/service/api/automation', () => ({
  sceneAutomationsGet: vi.fn(() => Promise.resolve({ data: { list: [] } }))
}))

vi.mock('@/service/api/auth', () => ({
  fetchTenantSetupState: vi.fn(() => Promise.resolve({ data: null }))
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({
    userInfo: hoisted.authUserInfo
  })
}))

vi.mock('@/utils/thingsvis/home-cache', () => ({
  readThingsVisHomeCache: hoisted.readThingsVisHomeCache,
  writeThingsVisHomeCache: hoisted.writeThingsVisHomeCache,
  clearThingsVisHomeCache: hoisted.clearThingsVisHomeCache
}))

vi.mock('@/utils/thingsvis/space', () => ({
  isSysAdminUser: hoisted.isSysAdminUser
}))

vi.mock('@/router', () => ({
  router: { go: vi.fn(), push: vi.fn() }
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: {},
    hash: ''
  })
}))

vi.mock('../homeDeploymentHealth', () => ({
  fetchDeploymentHealthReport: vi.fn(() => Promise.resolve(null)),
  normalizeDeploymentHealth: vi.fn(() => []),
  type: {}
}))

vi.mock('../homeFirstRunWizard', () => ({
  createHomeFirstRunFirstDevice: vi.fn(),
  getHomeFirstRunTenantId: vi.fn((userInfo: { tenant_id?: string; tenantId?: string }) =>
    String(userInfo?.tenant_id || userInfo?.tenantId || '').trim()
  )
}))

vi.mock('../homeFirstRunStorage', () => ({
  loadHomeFirstRunGuideState: vi.fn(() => null),
  saveHomeFirstRunGuideState: vi.fn((_storage, state) => state)
}))

vi.mock('../useHomeFirstDeviceWorkbench', () => ({
  useHomeFirstDeviceWorkbench: () => ({
    loading: ref(false),
    device: ref(null),
    telemetry: ref([]),
    simulation: ref(null),
    accessGuide: ref(null),
    actionLoading: ref(false),
    testResult: ref(''),
    browserTest: ref(null),
    publishCommand: computed(() => ''),
    onboardingGuard: computed(() => null),
    readyProof: computed(() => ({ ready: false })),
    firstChart: computed(() => ({ ready: false })),
    buildSupportSummary: vi.fn(() => ''),
    openReadyCheck: vi.fn(),
    openFullGuide: vi.fn(),
    copyPublishCommand: vi.fn(),
    simulateTelemetry: vi.fn(),
    runQuickstartAction: vi.fn(() => Promise.resolve()),
    refresh: vi.fn(() => Promise.resolve())
  })
}))

vi.mock('../useViewportDeferredMount', () => ({
  useViewportDeferredMount: () => ({
    shouldMount: { value: true },
    mountNow: vi.fn(),
    observe: vi.fn(() => Promise.resolve()),
    reset: vi.fn()
  })
}))

vi.mock('@/components/thingsvis/ThingsVisAppFrame.vue', () => ({
  default: defineComponent({
    props: ['id', 'schema', 'mode'],
    setup(props) {
      return () =>
        h('div', {
          'data-testid': 'thingsvis-frame',
          'data-id': props.id as string,
          'data-mode': props.mode as string
        })
    }
  })
}))

import Component from '../index.vue'
import { router } from '@/router'

const NButtonStub = defineComponent({
  inheritAttrs: false,
  emits: ['click'],
  setup(_props, { attrs, emit, slots }) {
    return () =>
      h(
        'button',
        {
          ...attrs,
          onClick: () => emit('click')
        },
        slots.default?.()
      )
  }
})

const NResultStub = defineComponent({
  name: 'NResult',
  inheritAttrs: false,
  props: {
    status: { type: String, default: '' },
    title: { type: String, default: '' },
    description: { type: String, default: '' }
  },
  setup(props, { slots }) {
    return () =>
      h('section', { 'data-testid': 'result', 'data-status': props.status }, [
        h('h1', props.title),
        h('p', props.description),
        slots.footer?.()
      ])
  }
})

const HomeSecondaryPanelStub = defineComponent({
  name: 'HomeSecondaryPanel',
  props: [
    'isHomeResolving',
    'showHomeResolvingGate',
    'homeResolvingDescription',
    'isError',
    'useThingsVis',
    'thingsVisHome',
    'thingsVisSectionRef',
    'shouldMountHomeThingsVisFrame',
    'showCompatHomeNotice',
    'compatHomeConfigCount'
  ],
  emits: [
    'reload',
    'openThingsVis',
    'mountHomeThingsVisFrame',
    'continueFirstDevice',
    'openDeviceManagement',
    'openRdiDashboard',
    'openRdiAlarmOverview',
    'openAlarmCenter',
    'openSystemSettings'
  ],
  setup() {
    return () => h('div', { 'data-testid': 'home-secondary-panel' })
  }
})

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        HomeFirstDeviceWorkbenchView: defineComponent({
          setup() {
            return () => h('div', { 'data-testid': 'first-device-workbench' })
          }
        }),
        HomeSecondaryPanel: HomeSecondaryPanelStub,
        NButton: NButtonStub,
        NCard: true,
        NResult: NResultStub,
        NSpin: true,
        NCountdown: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof mountComponent>) => wrapper.vm.$.setupState as Record<string, any>

describe('home/index.vue', () => {
  afterEach(() => {
    mountedWrappers.forEach((wrapper) => wrapper.unmount())
    mountedWrappers.length = 0
  })

  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchCompatHomeConfig.mockResolvedValue({ data: { config: '[]' }, error: null })
    hoisted.loadVisualizationHomeDashboard.mockResolvedValue({ ok: true, data: null })
    hoisted.probeVisualizationHomeDashboard.mockResolvedValue({ reachable: false, status: 0, dashboard: null })
    hoisted.readThingsVisHomeCache.mockReturnValue(null)
    hoisted.isSysAdminUser.mockReturnValue(false)
    hoisted.authUserInfo = { userName: 'admin' }
  })

  it('loads compatible home config first and defers the ThingsVis probe for non-sysadmin fallback', async () => {
    const wrapper = mountComponent()

    await flushPromises()

    expect(hoisted.fetchCompatHomeConfig).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchCompatHomeConfig).toHaveBeenCalledWith({})
    // The production path deliberately schedules this network probe as an
    // idle/background task so the fallback page can render without waiting.
    expect(hoisted.probeVisualizationHomeDashboard).not.toHaveBeenCalled()

    await getSetupState(wrapper).refreshThingsVisHomeDashboardInBackground()
    expect(hoisted.probeVisualizationHomeDashboard).toHaveBeenCalledTimes(1)
    const panel = wrapper.getComponent(HomeSecondaryPanelStub)
    expect(panel.props()).toMatchObject({
      isHomeResolving: false,
      isError: false,
      useThingsVis: false,
      showCompatHomeNotice: false,
      compatHomeConfigCount: 0
    })
  })

  it('shows compatible home notice when compatible homepage config exists', async () => {
    hoisted.fetchCompatHomeConfig.mockResolvedValue({
      data: { config: JSON.stringify([{ i: 'compat-home-1', x: 0, y: 0, w: 1, h: 1 }]) },
      error: null
    })

    const wrapper = mountComponent()

    await flushPromises()

    const panel = wrapper.findComponent(HomeSecondaryPanelStub)
    expect(panel.props('showCompatHomeNotice')).toBe(true)
    expect(panel.props('compatHomeConfigCount')).toBe(1)
  })

  it('navigates to RDI overview from the compatible home notice', async () => {
    hoisted.fetchCompatHomeConfig.mockResolvedValue({
      data: { config: JSON.stringify([{ i: 'compat-home-1', x: 0, y: 0, w: 1, h: 1 }]) },
      error: null
    })

    const wrapper = mountComponent()

    await flushPromises()

    const panel = wrapper.findComponent(HomeSecondaryPanelStub)
    panel.vm.$emit('openRdiDashboard')
    await flushPromises()
    expect(router.push).toHaveBeenCalledWith('/dashboard/rdi-overview')
  })

  it('navigates to RDI alarm overview from the compatible home notice', async () => {
    hoisted.fetchCompatHomeConfig.mockResolvedValue({
      data: { config: JSON.stringify([{ i: 'compat-home-1', x: 0, y: 0, w: 1, h: 1 }]) },
      error: null
    })

    const wrapper = mountComponent()

    await flushPromises()

    const panel = wrapper.findComponent(HomeSecondaryPanelStub)
    panel.vm.$emit('openRdiAlarmOverview')
    await flushPromises()
    expect(router.push).toHaveBeenCalledWith('/alarm/rdi-overview')
  })

  it('does not show compatible home notice when compatible homepage config is empty', async () => {
    hoisted.fetchCompatHomeConfig.mockResolvedValue({
      data: { config: '[]' },
      error: null
    })

    const wrapper = mountComponent()

    await flushPromises()

    const state = getSetupState(wrapper)
    expect(state.hasCompatHomeConfig).toBe(false)
    const panel = wrapper.getComponent(HomeSecondaryPanelStub)
    expect(panel.props('showCompatHomeNotice')).toBe(false)
    expect(panel.props('compatHomeConfigCount')).toBe(0)
  })

  it('marks error when compatible homepage config request fails for non-sysadmin', async () => {
    hoisted.fetchCompatHomeConfig.mockResolvedValue({ data: null, error: true })

    const wrapper = mountComponent()

    await flushPromises()

    const state = getSetupState(wrapper)
    expect(state.isError).toBe(true)
    const panel = wrapper.getComponent(HomeSecondaryPanelStub)
    expect(panel.props()).toMatchObject({
      isError: true,
      useThingsVis: false,
      showCompatHomeNotice: false,
      compatHomeConfigCount: 0
    })
  })

  it('does not reuse a stale ThingsVis cache in the Native default profile', async () => {
    const dashboard = {
      id: 'dash-1',
      canvasConfig: {},
      nodes: [{ id: 'node-1' }],
      dataSources: [{ id: 'source-1' }]
    }
    hoisted.readThingsVisHomeCache.mockReturnValue({ state: 'thingsvis', dashboard })

    const wrapper = mountComponent()

    await flushPromises()

    const state = getSetupState(wrapper)
    expect(state.useThingsVis).toBe(false)
    expect(state.thingsVisHome).toBe(null)
    expect(hoisted.readThingsVisHomeCache).toHaveBeenCalledTimes(1)
    expect(hoisted.clearThingsVisHomeCache).toHaveBeenCalledTimes(1)
    expect(hoisted.probeVisualizationHomeDashboard).not.toHaveBeenCalled()
    expect(hoisted.loadVisualizationHomeDashboard).not.toHaveBeenCalled()
    expect(hoisted.fetchCompatHomeConfig).toHaveBeenCalledTimes(1)
    const panel = wrapper.getComponent(HomeSecondaryPanelStub)
    expect(panel.props('useThingsVis')).toBe(false)
    expect(panel.props('thingsVisHome')).toBe(null)
  })

  it('shows the Native tenant-context requirement without probing or guessing a tenant', async () => {
    hoisted.isSysAdminUser.mockReturnValue(true)
    hoisted.authUserInfo = { userName: 'admin', authority: 'SYS_ADMIN', roles: ['SYS_ADMIN'] }

    const wrapper = mountComponent()

    await flushPromises()

    const state = getSetupState(wrapper)
    expect(state.showSysAdminSetup).toBe(true)
    expect(state.nativeTenantContextRequired).toBe(true)
    expect(state.homeDashboardUnavailable).toBe(false)
    expect(state.isError).toBe(false)
    expect(hoisted.fetchCompatHomeConfig).not.toHaveBeenCalled()
    expect(hoisted.probeVisualizationHomeDashboard).not.toHaveBeenCalled()

    const result = wrapper.getComponent(NResultStub)
    expect(result.props()).toMatchObject({
      status: 'info',
      title: 'custom.home.sysAdminSetup.nativeTenantContextTitle',
      description: 'custom.home.sysAdminSetup.nativeTenantContextDescription'
    })
    const openNativeBoardsButton = wrapper
      .findAll('button')
      .find((button) => button.text() === 'custom.home.actions.openNativeBoards')
    if (!openNativeBoardsButton) throw new Error('Missing Native boards tenant-context action')
    await openNativeBoardsButton.trigger('click')
    expect(router.push).toHaveBeenCalledWith('/visualization/native-boards')
  })
})
