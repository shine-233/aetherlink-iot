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
  getThingsVisProjects: vi.fn(),
  getThingsVisDashboards: vi.fn(),
  createThingsVisProject: vi.fn(),
  updateThingsVisProject: vi.fn(),
  deleteThingsVisProject: vi.fn(),
  deleteDashboardMenuConfig: vi.fn(),
  refreshAuthRoutes: vi.fn(),
  clearThingsVisHomeCache: vi.fn(),
  routerPushByKey: vi.fn(),
  messageError: vi.fn(),
}))

let currentRouteQuery: Record<string, any> = {}
let facadeSelectionError: { code: string; message: string } | null = null

vi.mock('@/service/api/thingsvis', () => ({
  getThingsVisProjects: hoisted.getThingsVisProjects,
  getThingsVisDashboards: hoisted.getThingsVisDashboards,
  createThingsVisProject: hoisted.createThingsVisProject,
  updateThingsVisProject: hoisted.updateThingsVisProject,
  deleteThingsVisProject: hoisted.deleteThingsVisProject,
}))

vi.mock('@/service/visualization-provider/index', async importOriginal => {
  const actual = await importOriginal<typeof import('@/service/visualization-provider/index')>()
  return {
    ...actual,
    getDefaultVisualizationProviderFacade: () => ({
      selectionError: facadeSelectionError,
      execute: (operation: (provider: typeof actual.legacyThingsVisProvider) => unknown) =>
        facadeSelectionError
          ? Promise.resolve({ ok: false, error: facadeSelectionError })
          : operation(actual.legacyThingsVisProvider)
    })
  }
})

vi.mock('@/service/api/dashboard-menu', () => ({
  deleteDashboardMenuConfig: hoisted.deleteDashboardMenuConfig,
}))

vi.mock('@/utils/router/refresh-auth-routes', () => ({
  refreshAuthRoutes: hoisted.refreshAuthRoutes,
}))

vi.mock('@/utils/thingsvis/home-cache', () => ({
  clearThingsVisHomeCache: hoisted.clearThingsVisHomeCache,
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: hoisted.routerPushByKey })
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: currentRouteQuery, fullPath: '/visualization/thingsvis' }),
  useRouter: () => ({ push: vi.fn(), back: vi.fn() })
}))

vi.mock('naive-ui', () => ({
  createDiscreteApi: () => ({ message: { success: vi.fn(), error: vi.fn(), warning: vi.fn() }, notification: {}, dialog: {}, loadingBar: {} }),
  NAlert: defineComponent({ setup(_, { slots }) { return () => h('div', [slots.header?.(), slots.default?.()]) } }),
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
  NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
  NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NForm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NGrid: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NGridItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  NEmpty: defineComponent({
    props: { description: { type: String, default: '' } },
    setup(props, { slots }) {
      return () => h('div', [props.description, slots.default?.(), slots.extra?.()])
    }
  }),
  NSpin: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
  useMessage: () => ({ success: vi.fn(), error: hoisted.messageError, warning: vi.fn() }),
}))

import ThingsVisIndex from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(ThingsVisIndex, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NGrid: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NGridItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NEmpty: defineComponent({
          props: { description: { type: String, default: '' } },
          setup(props, { slots }) {
            return () => h('div', [props.description, slots.default?.(), slots.extra?.()])
          }
        }),
        NSpin: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

const projectFixture = (overrides: Record<string, unknown> = {}) => ({
  id: '1',
  name: 'Project1',
  description: 'desc',
  thumbnail: null,
  createdAt: '2024-01-01',
  updatedAt: '2024-01-01',
  _count: { dashboards: 0 },
  ...overrides
})

const projectPage = (data: unknown[] = []) => ({
  data,
  meta: { page: 1, limit: 100, total: data.length, totalPages: data.length ? 1 : 0 }
})

describe('ThingsVisIndex', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    currentRouteQuery = {}
    facadeSelectionError = null
    hoisted.getThingsVisProjects.mockResolvedValue({ data: projectPage(), error: null })
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount and fetch projects on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.getThingsVisProjects).toHaveBeenCalledTimes(1)
    expect(hoisted.getThingsVisProjects).toHaveBeenCalledWith({ page: 1, limit: 100 })
    const state = getState(wrapper)
    expect(state.loading).toBe(false)
    expect(state.projects).toEqual([])
  })

  it('should populate projects on successful fetch', async () => {
    const mockProjects = [projectFixture()]
    hoisted.getThingsVisProjects.mockResolvedValue({ data: projectPage(mockProjects), error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.projects).toEqual([expect.objectContaining({ id: '1', name: 'Project1', dashboardCount: 0 })])
  })

  it('should show error message on fetch failure', async () => {
    hoisted.getThingsVisProjects.mockResolvedValue({ data: null, error: 'fail' })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.projects).toEqual([])
    expect(hoisted.messageError).toHaveBeenCalledWith('rdi.thingsvis.loadProjectsFailed')
  })

  it('shows an explicit provider state and skips project loading when ThingsVis is disabled', async () => {
    facadeSelectionError = { code: 'external-blocked', message: 'optional ThingsVis provider disabled' }
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.getThingsVisProjects).not.toHaveBeenCalled()
    const blockedAlert = wrapper.find('[data-testid="thingsvis-provider-blocked"]')
    expect(blockedAlert.attributes('data-provider-error')).toBe('external-blocked')
    expect(getState(wrapper).providerBlockedMessage).toBe('rdi.thingsvis.externalProviderDisabledDescription')
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('should open create modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.openCreateModal()
    expect(state.showModal).toBe(true)
    expect(state.editingProject).toBeNull()
    expect(state.formData).toEqual({ name: '', description: '' })
  })

  it('should open edit modal with project data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const project = { ...projectFixture({ name: 'Test' }), dashboardCount: 0 }
    state.openEditModal(project)
    expect(state.showModal).toBe(true)
    expect(state.editingProject).toEqual(project)
    expect(state.formData.name).toBe('Test')
    expect(state.formData.description).toBe('desc')
  })

  it('should not save project with empty name', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.openCreateModal()
    state.formData = { name: '', description: '' }
    await state.handleSaveProject()
    expect(hoisted.createThingsVisProject).toHaveBeenCalledTimes(0)
    expect(state.showModal).toBe(true)
    expect(hoisted.messageError).toHaveBeenCalledWith('rdi.thingsvis.projectNamePlaceholder')
  })

  it('should create project successfully', async () => {
    hoisted.createThingsVisProject.mockResolvedValue({ error: null })
    hoisted.getThingsVisProjects.mockResolvedValue({ data: projectPage(), error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.formData = { name: 'New Project', description: 'desc' }
    state.editingProject = null
    await state.handleSaveProject()
    await flushPromises()
    expect(hoisted.createThingsVisProject).toHaveBeenCalledWith({ name: 'New Project', description: 'desc' })
  })

  it('should expose first-device onboarding project creation when no project exists', async () => {
    currentRouteQuery = { onboarding: 'first-device' }
    const wrapper = mountComponent()
    await flushPromises()

    expect(wrapper.text()).toContain('rdi.thingsvis.firstDeviceDashboardTitle')
    expect(getState(wrapper).projects).toHaveLength(0)
    expect(wrapper.text()).toContain('rdi.thingsvis.firstDeviceProjectCreate')
  })

  it('should route created first-device project to dashboards with onboarding context', async () => {
    currentRouteQuery = { onboarding: 'first-device' }
    hoisted.createThingsVisProject.mockResolvedValue({ data: projectFixture({ id: 'proj-new', name: 'First Project' }), error: null })
    hoisted.getThingsVisProjects.mockResolvedValue({ data: projectPage(), error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.formData = { name: 'First Project', description: 'desc' }
    state.editingProject = null

    await state.handleSaveProject()
    await flushPromises()

    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('visualization_thingsvis-dashboards', {
      query: {
        projectId: 'proj-new',
        onboarding: 'first-device',
        provider: 'native'
      }
    })
  })

  it('should update project successfully', async () => {
    hoisted.updateThingsVisProject.mockResolvedValue({ error: null })
    hoisted.getThingsVisProjects.mockResolvedValue({ data: projectPage(), error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.editingProject = { id: '1', name: 'Old', description: 'old' } as any
    state.formData = { name: 'Updated', description: 'new' }
    await state.handleSaveProject()
    await flushPromises()
    expect(hoisted.updateThingsVisProject).toHaveBeenCalledWith('1', { name: 'Updated', description: 'new' })
  })

  it('should not open delete confirm if project has dashboards', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.allProjects = [{ ...projectFixture({ name: 'P1' }), dashboardCount: 2 }] as any
    state.openDeleteConfirm('1', 'P1')
    expect(state.deleteConfirmModal).toBe(false)
  })

  it('should open delete confirm if project has no dashboards', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.allProjects = [{ ...projectFixture({ name: 'P1' }), dashboardCount: 0 }] as any
    state.openDeleteConfirm('1', 'P1')
    expect(state.deleteConfirmModal).toBe(true)
    expect(state.pendingDeleteProject).toEqual({ id: '1', name: 'P1' })
  })

  it('blocks project deletion when listing dashboards fails', async () => {
    hoisted.getThingsVisDashboards.mockResolvedValue({ data: null, error: 'list failed' })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.pendingDeleteProject = { id: '1', name: 'P1' }
    state.deleteConfirmModal = true

    await state.handleDeleteProject()
    await flushPromises()

    expect(hoisted.deleteDashboardMenuConfig).not.toHaveBeenCalled()
    expect(hoisted.deleteThingsVisProject).not.toHaveBeenCalled()
    expect(hoisted.messageError).toHaveBeenCalledWith('rdi.thingsvis.deleteFailed')
    expect(state.deleteConfirmModal).toBe(true)
  })

  it('should handle delete project successfully', async () => {
    hoisted.getThingsVisDashboards.mockResolvedValue({
      data: { data: [], meta: { page: 1, limit: 1000, total: 0, totalPages: 0 } },
      error: null
    })
    hoisted.deleteThingsVisProject.mockResolvedValue({ error: null })
    hoisted.refreshAuthRoutes.mockResolvedValue(undefined)
    hoisted.getThingsVisProjects.mockResolvedValue({ data: projectPage(), error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.pendingDeleteProject = { id: '1', name: 'P1' }
    state.deleteConfirmModal = true
    await state.handleDeleteProject()
    await flushPromises()
    expect(hoisted.deleteThingsVisProject).toHaveBeenCalledWith('1')
    expect(hoisted.refreshAuthRoutes).toHaveBeenCalledTimes(1)
    expect(hoisted.refreshAuthRoutes).toHaveBeenCalledWith('/visualization/thingsvis')
    expect(hoisted.clearThingsVisHomeCache).toHaveBeenCalledTimes(1)
  })

  it('should filter projects by search keyword', async () => {
    const mockProjects = [
      projectFixture({ id: '1', name: 'Alpha', description: '' }),
      projectFixture({ id: '2', name: 'Beta', description: '' })
    ]
    hoisted.getThingsVisProjects.mockResolvedValue({ data: projectPage(mockProjects), error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.searchKeyword = 'alpha'
    await state.fetchProjects()
    await flushPromises()
    expect(state.projects.length).toBe(1)
    expect(state.projects[0].name).toBe('Alpha')
  })
})
