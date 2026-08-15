/**
 * 文件用途：验证 ThingsVis 仪表盘列表页的数据加载、筛选与关键操作流程。
 * 核心逻辑：挂载页面并通过局部组件及接口 mock，断言父页面传递的业务数据、提交载荷和状态流转。
 * 关键注意事项：卡片 stub 只承载父子组件契约，不把 stub 自行拼装的链接或内容当作真实卡片行为证据。
 * 重构建议：为 ThingsVisDashboardCard 补独立组件测试，覆盖真实预览链接、懒加载资源和卡片操作。
 */
import { defineComponent, h, onMounted } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getThingsVisDashboards: vi.fn(),
  getThingsVisHomeDashboard: vi.fn(),
  getThingsVisProject: vi.fn(),
  createThingsVisDashboard: vi.fn(),
  deleteThingsVisDashboard: vi.fn(),
  setHomeThingsVisDashboard: vi.fn(),
  getThingsVisDashboardThumbnail: vi.fn(),
  fetchDashboardMenuConfig: vi.fn(),
  fetchDashboardMenuConfigs: vi.fn(),
  saveDashboardMenuConfig: vi.fn(),
  deleteDashboardMenuConfig: vi.fn(),
  refreshAuthRoutes: vi.fn(),
  clearThingsVisHomeCache: vi.fn(),
  messageError: vi.fn(),
  messageSuccess: vi.fn(),
  providerSelectionError: null as { code: string; message: string } | null,
  authUserInfo: {
    authority: 'TENANT_ADMIN',
    roles: ['TENANT_ADMIN']
  }
}))

let currentRouteQuery: Record<string, any> = { projectId: 'proj-1' }

vi.mock('@/service/api/thingsvis', () => ({
  getThingsVisDashboards: hoisted.getThingsVisDashboards,
  getThingsVisHomeDashboard: hoisted.getThingsVisHomeDashboard,
  getThingsVisProject: hoisted.getThingsVisProject,
  createThingsVisDashboard: hoisted.createThingsVisDashboard,
  deleteThingsVisDashboard: hoisted.deleteThingsVisDashboard,
  setHomeThingsVisDashboard: hoisted.setHomeThingsVisDashboard,
  getThingsVisDashboardThumbnail: hoisted.getThingsVisDashboardThumbnail
}))

vi.mock('@/service/visualization-provider/index', async importOriginal => {
  const actual = await importOriginal<typeof import('@/service/visualization-provider/index')>()
  return {
    ...actual,
    getDefaultVisualizationProviderFacade: () => {
      const selectionError = hoisted.providerSelectionError
      return {
        selectionError,
        execute: (operation: (provider: typeof actual.legacyThingsVisProvider) => unknown) =>
          selectionError
            ? Promise.resolve({ ok: false, error: selectionError })
            : operation(actual.legacyThingsVisProvider)
      }
    }
  }
})

vi.mock('@/service/api/dashboard-menu', () => ({
  fetchDashboardMenuConfig: hoisted.fetchDashboardMenuConfig,
  fetchDashboardMenuConfigs: hoisted.fetchDashboardMenuConfigs,
  saveDashboardMenuConfig: hoisted.saveDashboardMenuConfig,
  deleteDashboardMenuConfig: hoisted.deleteDashboardMenuConfig
}))

vi.mock('@/utils/router/refresh-auth-routes', () => ({
  refreshAuthRoutes: hoisted.refreshAuthRoutes
}))

vi.mock('@/utils/thingsvis/home-cache', () => ({
  clearThingsVisHomeCache: hoisted.clearThingsVisHomeCache
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({ userInfo: hoisted.authUserInfo })
}))

vi.mock('@/locales', () => ({
  $t: (key: string, params?: Record<string, any>) => (params?.name ? `${key}:${params.name}` : key)
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: vi.fn() })
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: currentRouteQuery, fullPath: '/visualization/thingsvis-dashboards' }),
  useRouter: () => ({ push: vi.fn(), back: vi.fn() })
}))

vi.mock('naive-ui', async importOriginal => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      error: hoisted.messageError,
      success: hoisted.messageSuccess,
      warning: vi.fn(),
      info: vi.fn()
    })
  }
})

vi.mock('../rdi-preset', () => ({
  buildRdiDashboardPreset: vi.fn(() => ({ canvasConfig: {}, nodes: [], dataSources: [], variables: [] })),
  getDashboardTemplateOptions: vi.fn(() => [
    { label: 'Blank', value: 'blank' },
    { label: 'RDI', value: 'rdi' }
  ])
}))

import ThingsVisDashboards from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const passthroughStub = (name: string, tag = 'div') =>
  defineComponent({
    name,
    inheritAttrs: false,
    setup(_, { attrs, slots }) {
      return () => h(tag, attrs, slots.default?.())
    }
  })

const iconStub = (name: string, label: string) =>
  defineComponent({
    name,
    setup() {
      return () => h('span', { 'data-icon': label }, `[${label}]`)
    }
  })

const ButtonStub = defineComponent({
  name: 'NButton',
  inheritAttrs: false,
  props: {
    type: { type: String, default: 'default' },
    disabled: Boolean,
    loading: Boolean,
    secondary: Boolean,
    size: { type: String, default: 'medium' }
  },
  emits: ['click'],
  setup(props, { attrs, slots, emit }) {
    return () =>
      h(
        'button',
        {
          ...attrs,
          type: 'button',
          disabled: props.disabled,
          'data-type': props.type,
          'data-loading': String(props.loading),
          'data-secondary': String(props.secondary),
          'data-size': props.size,
          onClick: () => emit('click')
        },
        [slots.icon?.(), slots.default?.()]
      )
  }
})

const InputStub = defineComponent({
  name: 'NInput',
  props: {
    value: { type: String, default: '' },
    placeholder: { type: String, default: '' },
    disabled: Boolean
  },
  emits: ['update:value'],
  setup(props, { slots, emit }) {
    return () =>
      h('label', [
        slots.prefix?.(),
        h('input', {
          value: props.value,
          placeholder: props.placeholder,
          disabled: props.disabled,
          onInput: (event: Event) => emit('update:value', (event.target as HTMLInputElement).value)
        })
      ])
  }
})

const InputNumberStub = defineComponent({
  name: 'NInputNumber',
  props: {
    value: { type: Number, default: null },
    placeholder: { type: String, default: '' },
    min: { type: Number, default: undefined },
    disabled: Boolean
  },
  emits: ['update:value'],
  setup(props, { emit }) {
    return () =>
      h('input', {
        type: 'number',
        value: props.value ?? '',
        placeholder: props.placeholder,
        min: props.min,
        disabled: props.disabled,
        onInput: (event: Event) => {
          const raw = (event.target as HTMLInputElement).value
          emit('update:value', raw === '' ? null : Number(raw))
        }
      })
  }
})

const SelectStub = defineComponent({
  name: 'NSelect',
  props: {
    value: { type: [String, Number], default: null },
    options: { type: Array, default: () => [] }
  },
  emits: ['update:value'],
  setup(props, { emit }) {
    return () =>
      h(
        'select',
        {
          value: props.value ?? '',
          onChange: (event: Event) => emit('update:value', (event.target as HTMLSelectElement).value)
        },
        (props.options as Array<any>).map(option =>
          h('option', { value: option.value }, option.label ?? String(option.value))
        )
      )
  }
})

const SwitchStub = defineComponent({
  name: 'NSwitch',
  props: {
    value: { type: Boolean, default: false }
  },
  emits: ['update:value'],
  setup(props, { emit }) {
    return () =>
      h('input', {
        type: 'checkbox',
        checked: props.value,
        onChange: (event: Event) => emit('update:value', (event.target as HTMLInputElement).checked)
      })
  }
})

const ModalStub = defineComponent({
  name: 'NModal',
  props: {
    show: { type: Boolean, default: false }
  },
  setup(props, { slots }) {
    return () =>
      props.show
        ? h('div', { class: 'modal-stub' }, [slots.default?.(), slots.footer?.(), slots.action?.()])
        : null
  }
})

const ThingsVisDashboardCardStub = defineComponent({
  name: 'ThingsVisDashboardCard',
  props: {
    dashboard: { type: Object, required: true },
    menuConfig: { type: Object, default: null },
    thumbnailUrl: { type: String, default: '' },
    menuConfigLoaded: { type: Boolean, default: false }
  },
  emits: [
    'edit',
    'menu',
    'set-home',
    'publish',
    'duplicate',
    'copy-link',
    'request-thumbnail',
    'request-menu-config',
    'delete'
  ],
  setup(props, { emit }) {
    onMounted(() => {
      emit('request-thumbnail', props.dashboard)
      emit('request-menu-config', props.dashboard)
    })

    return () =>
      h('div', { class: 'thingsvis-dashboard-card-stub' }, [
        h('span', { class: 'dashboard-name' }, (props.dashboard as any).name),
        h(
          'button',
          {
            type: 'button',
            onClick: () => emit('edit', (props.dashboard as any).id)
          },
          'rdi.thingsvis.edit'
        ),
        h(
          'button',
          {
            type: 'button',
            onClick: () => emit('menu', props.dashboard)
          },
          '[icon-mdi:menu]'
        ),
        !(props.dashboard as any).home
          ? h(
              PopconfirmStub,
              {
                onPositiveClick: () => emit('set-home', props.dashboard)
              },
              {
                trigger: () => h('button', { type: 'button' }, '[icon-mdi:home-outline]')
              }
            )
          : null,
        h(
          'button',
          {
            type: 'button',
            onClick: () => emit('delete', (props.dashboard as any).id, (props.dashboard as any).name)
          },
          '[icon-mdi:delete]'
        )
      ])
  }
})

const TooltipStub = defineComponent({
  name: 'NTooltip',
  setup(_, { slots }) {
    return () => h('div', { class: 'tooltip-stub' }, [slots.trigger?.(), slots.default?.()])
  }
})

const PopconfirmStub = defineComponent({
  name: 'NPopconfirm',
  emits: ['positive-click'],
  setup(_, { slots }) {
    return () => h('div', { class: 'popconfirm-stub' }, [slots.trigger?.(), slots.default?.()])
  }
})

const mountComponent = () => {
  const wrapper = mount(ThingsVisDashboards, {
    attachTo: document.body,
    global: {
      stubs: {
        NCard: passthroughStub('NCard'),
        'n-card': passthroughStub('NCard'),
        NAlert: passthroughStub('NAlert'),
        'n-alert': passthroughStub('NAlert'),
        NForm: passthroughStub('NForm', 'form'),
        'n-form': passthroughStub('NForm', 'form'),
        NFormItem: passthroughStub('NFormItem', 'label'),
        'n-form-item': passthroughStub('NFormItem', 'label'),
        NBreadcrumb: passthroughStub('NBreadcrumb', 'nav'),
        'n-breadcrumb': passthroughStub('NBreadcrumb', 'nav'),
        NBreadcrumbItem: passthroughStub('NBreadcrumbItem', 'span'),
        'n-breadcrumb-item': passthroughStub('NBreadcrumbItem', 'span'),
        NGrid: passthroughStub('NGrid'),
        'n-grid': passthroughStub('NGrid'),
        NGridItem: passthroughStub('NGridItem'),
        'n-grid-item': passthroughStub('NGridItem'),
        NTag: passthroughStub('NTag', 'span'),
        'n-tag': passthroughStub('NTag', 'span'),
        NEmpty: passthroughStub('NEmpty'),
        'n-empty': passthroughStub('NEmpty'),
        NSpin: passthroughStub('NSpin'),
        'n-spin': passthroughStub('NSpin'),
        NTooltip: TooltipStub,
        'n-tooltip': TooltipStub,
        NPopconfirm: PopconfirmStub,
        'n-popconfirm': PopconfirmStub,
        NButton: ButtonStub,
        'n-button': ButtonStub,
        NInput: InputStub,
        'n-input': InputStub,
        NInputNumber: InputNumberStub,
        'n-input-number': InputNumberStub,
        NSelect: SelectStub,
        'n-select': SelectStub,
        NSwitch: SwitchStub,
        'n-switch': SwitchStub,
        NModal: ModalStub,
        'n-modal': ModalStub,
        ThingsVisDashboardCard: ThingsVisDashboardCardStub,
        'things-vis-dashboard-card': ThingsVisDashboardCardStub,
        icon: passthroughStub('icon', 'span'),
        'icon-mdi:chevron-left': iconStub('icon-mdi:chevron-left', 'icon-mdi:chevron-left'),
        'icon-mdi:magnify': iconStub('icon-mdi:magnify', 'icon-mdi:magnify'),
        'icon-mdi:plus': iconStub('icon-mdi:plus', 'icon-mdi:plus'),
        'icon-mdi:chart-box-outline': iconStub('icon-mdi:chart-box-outline', 'icon-mdi:chart-box-outline'),
        'icon-mdi:chart-box': iconStub('icon-mdi:chart-box', 'icon-mdi:chart-box'),
        'icon-mdi:tag-outline': iconStub('icon-mdi:tag-outline', 'icon-mdi:tag-outline'),
        'icon-mdi:clock-outline': iconStub('icon-mdi:clock-outline', 'icon-mdi:clock-outline'),
        'icon-mdi:pencil': iconStub('icon-mdi:pencil', 'icon-mdi:pencil'),
        'icon-mdi:cloud-upload-outline': iconStub('icon-mdi:cloud-upload-outline', 'icon-mdi:cloud-upload-outline'),
        'icon-mdi:content-copy': iconStub('icon-mdi:content-copy', 'icon-mdi:content-copy'),
        'icon-mdi:link-variant': iconStub('icon-mdi:link-variant', 'icon-mdi:link-variant'),
        'icon-mdi:menu': iconStub('icon-mdi:menu', 'icon-mdi:menu'),
        'icon-mdi:home-outline': iconStub('icon-mdi:home-outline', 'icon-mdi:home-outline'),
        'icon-mdi:home-plus-outline': iconStub('icon-mdi:home-plus-outline', 'icon-mdi:home-plus-outline'),
        'icon-mdi:home': iconStub('icon-mdi:home', 'icon-mdi:home'),
        'icon-mdi:delete': iconStub('icon-mdi:delete', 'icon-mdi:delete'),
        'icon-mdi:alert-circle': iconStub('icon-mdi:alert-circle', 'icon-mdi:alert-circle')
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function dashboardList() {
  return [
    {
      id: 'dash-1',
      name: 'Alpha Dashboard',
      version: 1,
      projectId: 'proj-1',
      createdAt: '2026-07-01T00:00:00Z',
      updatedAt: '2026-07-05T00:00:00Z',
      isPublished: true,
      homeFlag: false,
      thumbnail: null
    },
    {
      id: 'dash-2',
      name: 'Beta Dashboard',
      version: 2,
      projectId: 'proj-1',
      createdAt: '2026-07-01T00:00:00Z',
      updatedAt: '2026-07-04T00:00:00Z',
      isPublished: false,
      homeFlag: true,
      thumbnail: null
    }
  ]
}

function dashboardSchema(overrides: Record<string, unknown> = {}) {
  return {
    id: 'dash-new',
    name: 'New Dashboard',
    thumbnail: null,
    version: 1,
    canvasConfig: { mode: 'fixed', width: 1920, height: 1080, background: null },
    nodes: [],
    dataSources: [],
    variables: [],
    isPublished: false,
    publishedAt: null,
    shareToken: null,
    projectId: 'proj-1',
    createdAt: '2026-07-01T00:00:00Z',
    updatedAt: '2026-07-01T00:00:00Z',
    ...overrides
  }
}

function findButtonByContent(wrapper: ReturnType<typeof mountComponent>, content: string, occurrence = 0) {
  const matches = wrapper.findAll('button, button-stub, n-button-stub').filter(button => button.text().includes(content))
  expect(matches.length).toBeGreaterThan(occurrence)
  return matches[occurrence]
}

function findDocumentButtonByContent(content: string, occurrence = 0) {
  const matches = Array.from(document.body.querySelectorAll('button')).filter(button =>
    button.textContent?.includes(content)
  )
  expect(matches.length).toBeGreaterThan(occurrence)
  return matches[occurrence] as HTMLButtonElement
}

describe('ThingsVisDashboards', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.providerSelectionError = null
    currentRouteQuery = { projectId: 'proj-1' }
    Object.assign(hoisted.authUserInfo, {
      authority: 'TENANT_ADMIN',
      roles: ['TENANT_ADMIN']
    })
    hoisted.getThingsVisProject.mockResolvedValue({
      data: {
        id: 'proj-1',
        name: 'Test Project',
        description: null,
        thumbnail: null,
        createdAt: '2026-07-01T00:00:00Z',
        updatedAt: '2026-07-01T00:00:00Z'
      },
      error: null
    })
    hoisted.getThingsVisDashboards.mockResolvedValue({
      data: { data: dashboardList(), meta: { page: 1, limit: 100, total: 2, totalPages: 1 } },
      error: null
    })
    hoisted.getThingsVisDashboardThumbnail.mockImplementation(async (id: string) =>
      id === 'dash-1' ? { data: { thumbnail: 'abc123' }, error: null } : { data: { thumbnail: null }, error: null }
    )
    hoisted.fetchDashboardMenuConfig.mockResolvedValue({ data: null, error: 'not found' })
    hoisted.fetchDashboardMenuConfigs.mockResolvedValue({ data: {}, error: null })
    hoisted.createThingsVisDashboard.mockResolvedValue({ data: dashboardSchema(), error: null })
    hoisted.deleteDashboardMenuConfig.mockResolvedValue({ error: null })
    hoisted.deleteThingsVisDashboard.mockResolvedValue({ error: null })
    hoisted.setHomeThingsVisDashboard.mockResolvedValue({ error: null })
    hoisted.saveDashboardMenuConfig.mockResolvedValue({ data: { enabled: true, menu_name: 'Menu Alpha', sort: 1 }, error: null })
    hoisted.getThingsVisHomeDashboard.mockResolvedValue({ data: { data: { id: 'home-1', name: 'Home Dashboard' } }, error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    document.body.innerHTML = ''
    vi.useRealTimers()
  })

  it('loads the project and dashboard list on mount', async () => {
    vi.useFakeTimers()
    const wrapper = mountComponent()

    await flushPromises()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(200)
    await flushPromises()

    expect(hoisted.getThingsVisProject).toHaveBeenCalledWith('proj-1')
    expect(hoisted.getThingsVisDashboards).toHaveBeenCalledWith({
      projectId: 'proj-1',
      page: 1,
      limit: 100
    })
    expect(wrapper.text()).toContain('Test Project')
    expect(wrapper.text()).toContain('Alpha Dashboard')
    expect(wrapper.text()).toContain('Beta Dashboard')
    const cards = wrapper.findAllComponents(ThingsVisDashboardCardStub)
    expect(cards).toHaveLength(2)
    expect(cards.map(card => card.props('dashboard').id)).toEqual(['dash-1', 'dash-2'])
    expect(cards[0].props('thumbnailUrl')).toBe('data:image/png;base64,abc123')
    expect(cards[1].props('thumbnailUrl')).toBe('')
    expect(hoisted.getThingsVisDashboardThumbnail).toHaveBeenCalledTimes(2)
    expect(hoisted.getThingsVisDashboardThumbnail).toHaveBeenCalledWith('dash-1')
    expect(hoisted.getThingsVisDashboardThumbnail).toHaveBeenCalledWith('dash-2')
  })

  it('shows an explicit disabled state instead of offering external dashboard writes', async () => {
    hoisted.providerSelectionError = {
      code: 'external-blocked',
      message: 'Optional external visualization provider is disabled: legacy-thingsvis'
    }
    currentRouteQuery = { projectId: 'external-project' }

    const wrapper = mountComponent()
    await flushPromises()

    expect(wrapper.get('[data-testid="thingsvis-provider-blocked"]').text()).toContain(
      'rdi.thingsvis.externalProviderDisabledDescription'
    )
    expect(wrapper.findAll('button').filter(button => button.text().includes('rdi.thingsvis.newDashboard'))).toHaveLength(0)
    expect(wrapper.find('[data-testid="thingsvis-dashboard-list"]').exists()).toBe(false)
    expect(hoisted.getThingsVisProject).not.toHaveBeenCalled()
    expect(hoisted.getThingsVisDashboards).not.toHaveBeenCalled()
    expect(hoisted.messageError).not.toHaveBeenCalledWith('rdi.thingsvis.loadDashboardsFailed')
  })

  it('does not request tenant menu config for a Native SYS_ADMIN fallback', async () => {
    vi.useFakeTimers()
    currentRouteQuery = { provider: 'native' }
    Object.assign(hoisted.authUserInfo, {
      authority: 'SYS_ADMIN',
      roles: ['SYS_ADMIN']
    })

    const wrapper = mountComponent()
    await flushPromises()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(250)
    await flushPromises()

    expect(hoisted.fetchDashboardMenuConfigs).not.toHaveBeenCalled()
    expect(hoisted.fetchDashboardMenuConfig).not.toHaveBeenCalled()
    expect(hoisted.saveDashboardMenuConfig).not.toHaveBeenCalled()
    expect(hoisted.deleteDashboardMenuConfig).not.toHaveBeenCalled()

    const cards = wrapper.findAllComponents(ThingsVisDashboardCardStub)
    expect(cards).toHaveLength(2)
    expect(cards.every(card => card.props('menuConfigLoaded') === true)).toBe(true)

    await cards[0].findAll('button')[1].trigger('click')
    await flushPromises()
    expect(hoisted.fetchDashboardMenuConfig).not.toHaveBeenCalled()
    expect(hoisted.saveDashboardMenuConfig).not.toHaveBeenCalled()
  })

  it('keeps the real menu flow for a TENANT_ADMIN on an explicit Native route', async () => {
    currentRouteQuery = { provider: 'native' }
    const wrapper = mountComponent()

    await flushPromises()
    await flushPromises()
    await findButtonByContent(wrapper, '[icon-mdi:menu]').trigger('click')
    await flushPromises()

    expect(hoisted.fetchDashboardMenuConfigs).toHaveBeenCalledWith(['dash-1', 'dash-2'])
    expect(document.body.querySelectorAll('[role="switch"]')).toHaveLength(1)

    const menuSwitch = document.body.querySelector('[role="switch"]') as HTMLElement | null
    menuSwitch!.click()
    await flushPromises()
    findDocumentButtonByContent('rdi.thingsvis.saveMenu').click()
    await flushPromises()

    expect(hoisted.saveDashboardMenuConfig).toHaveBeenCalledWith(
      'dash-1',
      expect.objectContaining({
        dashboard_name: 'Alpha Dashboard',
        enabled: true
      })
    )
  })

  it('does not repeat thumbnail requests for dashboards without thumbnails', async () => {
    vi.useFakeTimers()
    const wrapper = mountComponent()

    await flushPromises()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(200)
    await flushPromises()

    const betaCard = wrapper.findAllComponents(ThingsVisDashboardCardStub)[1]
    expect(betaCard.props('dashboard')).toMatchObject({
      id: 'dash-2',
      name: 'Beta Dashboard'
    })
    betaCard.vm.$emit('request-thumbnail', betaCard.props('dashboard'))
    await vi.advanceTimersByTimeAsync(200)
    await flushPromises()

    const betaThumbnailRequests = hoisted.getThingsVisDashboardThumbnail.mock.calls.filter(([id]) => id === 'dash-2')
    expect(betaThumbnailRequests).toHaveLength(1)
  })

  it('defaults a missing projectId to the built-in Native project', async () => {
    currentRouteQuery = {}
    const wrapper = mountComponent()

    await flushPromises()

    expect(hoisted.messageError).not.toHaveBeenCalledWith('rdi.thingsvis.missingProjectId')
    expect(wrapper.text()).toContain('Alpha Dashboard')
  })

  it('creates a dashboard through the rendered create modal flow', async () => {
    const wrapper = mountComponent()

    await flushPromises()
    await flushPromises()

    await findButtonByContent(wrapper, 'rdi.thingsvis.newDashboard').trigger('click')
    await flushPromises()

    const nameInput = document.body.querySelector(
      'input[placeholder="rdi.thingsvis.dashboardNamePlaceholder"]'
    ) as HTMLInputElement | null
    expect(nameInput?.getAttribute('placeholder')).toBe('rdi.thingsvis.dashboardNamePlaceholder')
    nameInput!.value = 'New Dashboard'
    nameInput!.dispatchEvent(new Event('input'))
    findDocumentButtonByContent('rdi.thingsvis.create').click()
    await flushPromises()

    expect(hoisted.createThingsVisDashboard).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'New Dashboard',
        projectId: 'proj-1'
      })
    )
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('rdi.thingsvis.createSuccess')
  })

  it('does not create a dashboard with an empty name', async () => {
    const wrapper = mountComponent()

    await flushPromises()
    await flushPromises()

    await findButtonByContent(wrapper, 'rdi.thingsvis.newDashboard').trigger('click')
    await flushPromises()
    findDocumentButtonByContent('rdi.thingsvis.create').click()
    await flushPromises()

    expect(hoisted.createThingsVisDashboard).toHaveBeenCalledTimes(0)
    expect(hoisted.messageError).toHaveBeenCalledWith('rdi.thingsvis.dashboardNamePlaceholder')
  })

  it('filters rendered dashboards through the search input', async () => {
    const wrapper = mountComponent()

    await flushPromises()
    await flushPromises()

    const searchInput = wrapper.find('input[placeholder="rdi.thingsvis.searchDashboardPlaceholder"]')
    await searchInput.setValue('alpha')
    await flushPromises()

    expect(wrapper.text()).toContain('Alpha Dashboard')
    expect(wrapper.text()).not.toContain('Beta Dashboard')
  })

  it('deletes a dashboard through the rendered delete flow', async () => {
    const wrapper = mountComponent()

    await flushPromises()
    await flushPromises()

    await findButtonByContent(wrapper, '[icon-mdi:delete]').trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('rdi.thingsvis.deleteConfirm:Alpha Dashboard')

    findDocumentButtonByContent('rdi.thingsvis.confirmDeleteAction').click()
    await flushPromises()

    expect(hoisted.deleteDashboardMenuConfig).toHaveBeenCalledWith('dash-1')
    expect(hoisted.deleteThingsVisDashboard).toHaveBeenCalledWith('dash-1')
  })

  it('sets a dashboard as homepage through the popconfirm contract', async () => {
    const wrapper = mountComponent()

    await flushPromises()
    await flushPromises()

    const popconfirm = wrapper.findComponent(PopconfirmStub)
    expect(wrapper.findAllComponents(PopconfirmStub)).toHaveLength(1)
    await popconfirm.vm.$emit('positive-click')
    await flushPromises()

    expect(hoisted.setHomeThingsVisDashboard).toHaveBeenCalledWith('dash-1')
    expect(hoisted.clearThingsVisHomeCache).toHaveBeenCalledTimes(1)
  })

  it('offers a first-device homepage dashboard CTA when onboarding has dashboards but no home dashboard', async () => {
    currentRouteQuery = { projectId: 'proj-1', onboarding: 'first-device' }
    hoisted.getThingsVisDashboards.mockResolvedValue({
      data: {
        data: dashboardList().map(dashboard => ({ ...dashboard, homeFlag: false })),
        meta: { page: 1, limit: 100, total: 2, totalPages: 1 }
      },
      error: null
    })
    hoisted.createThingsVisDashboard.mockResolvedValue({
      data: dashboardSchema({ name: 'rdi.thingsvis.homepageStarterDashboardName' }),
      error: null
    })
    const wrapper = mountComponent()

    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('rdi.thingsvis.firstDeviceHomepageMissingTitle')
    await findButtonByContent(wrapper, 'rdi.thingsvis.createHomepageDashboard').trigger('click')
    await flushPromises()

    expect(hoisted.createThingsVisDashboard).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'rdi.thingsvis.homepageStarterDashboardName',
        projectId: 'proj-1'
      })
    )
    expect(hoisted.setHomeThingsVisDashboard).toHaveBeenCalledWith('dash-new')
    expect(hoisted.clearThingsVisHomeCache).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith(
      'rdi.thingsvis.createHomepageDashboardSuccess:rdi.thingsvis.homepageStarterDashboardName'
    )
  })

  it('opens menu config and saves it through rendered modal controls', async () => {
    const wrapper = mountComponent()

    await flushPromises()
    await flushPromises()

    await findButtonByContent(wrapper, '[icon-mdi:menu]').trigger('click')
    await flushPromises()

    const menuSwitch = document.body.querySelector('[role="switch"]') as HTMLElement | null
    expect(document.body.querySelectorAll('[role="switch"]')).toHaveLength(1)
    menuSwitch!.click()
    await flushPromises()

    const menuNameInput = document.body.querySelector(
      'input[placeholder="rdi.thingsvis.menuNamePlaceholder"]'
    ) as HTMLInputElement | null
    expect(menuNameInput?.getAttribute('placeholder')).toBe('rdi.thingsvis.menuNamePlaceholder')
    menuNameInput!.value = 'Menu Alpha'
    menuNameInput!.dispatchEvent(new Event('input'))

    findDocumentButtonByContent('rdi.thingsvis.saveMenu').click()
    await flushPromises()

    expect(hoisted.saveDashboardMenuConfig).toHaveBeenCalledWith(
      'dash-1',
      expect.objectContaining({
        menu_name: 'Menu Alpha',
        dashboard_name: 'Alpha Dashboard',
        enabled: true
      })
    )
    expect(hoisted.refreshAuthRoutes).toHaveBeenCalledWith('/visualization/thingsvis-dashboards')
  })
})
