import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { VisualizationDashboardSchema, VisualizationDashboardSummary, VisualizationProvider } from './contracts'
import { VisualizationProviderRegistry } from './registry'
import { createVisualizationProviderFacade } from './facade'
import { createInMemoryLocalVisualizationProvider } from './in-memory-local-provider'

const legacy = vi.hoisted(() => ({
  getThingsVisProjects: vi.fn(),
  getThingsVisProject: vi.fn(),
  createThingsVisProject: vi.fn(),
  updateThingsVisProject: vi.fn(),
  deleteThingsVisProject: vi.fn(),
  getThingsVisDashboards: vi.fn(),
  getThingsVisDashboard: vi.fn(),
  getThingsVisDashboardThumbnail: vi.fn(),
  createThingsVisDashboard: vi.fn(),
  updateThingsVisDashboard: vi.fn(),
  deleteThingsVisDashboard: vi.fn(),
  publishThingsVisDashboard: vi.fn(),
  duplicateThingsVisDashboard: vi.fn(),
  setHomeThingsVisDashboard: vi.fn(),
  unsetHomeThingsVisDashboard: vi.fn(),
  getThingsVisHomeDashboard: vi.fn()
}))

vi.mock('@/service/api/thingsvis', () => legacy)

import { legacyThingsVisProvider } from './legacy-thingsvis-adapter'
import {
  getDefaultVisualizationProviderFacade,
  getDefaultVisualizationProviderRegistry,
  registerDefaultVisualizationProviders,
  resolveVisualizationProviderId
} from './composition'

const timestamp = '2026-08-01T00:00:00.000Z'
const project = (overrides: Record<string, unknown> = {}) => ({
  id: 'project-1', name: 'Project', description: null, thumbnail: null,
  createdAt: timestamp, updatedAt: timestamp, _count: { dashboards: 2 }, ...overrides
})
const dashboard = (overrides: Record<string, unknown> = {}) => ({
  id: 'dashboard-1', name: 'Dashboard', thumbnail: null, version: 2,
  canvasConfig: { mode: 'fixed', width: 1920, height: 1080, background: null },
  nodes: [], dataSources: [], variables: [], isPublished: true, publishedAt: timestamp,
  shareToken: 'share', homeFlag: false, projectId: 'project-1', createdAt: timestamp,
  updatedAt: timestamp, ...overrides
})
const page = (data: unknown[]) => ({ data, meta: { page: 1, limit: 20, total: data.length, totalPages: data.length ? 1 : 0 } })

function providerStub(id = 'stub'): VisualizationProvider {
  const ok = async () => ({ ok: true as const, data: undefined })
  return {
    id, kind: 'local', deploymentMode: 'local-default',
    listProjects: vi.fn(), getProject: vi.fn(), createProject: vi.fn(), updateProject: vi.fn(), deleteProject: ok,
    listDashboards: vi.fn(), getDashboard: vi.fn(), getDashboardThumbnail: vi.fn(), createDashboard: vi.fn(),
    updateDashboard: vi.fn(), deleteDashboard: ok, publishDashboard: vi.fn(), duplicateDashboard: vi.fn(),
    setHomeDashboard: ok, unsetHomeDashboard: ok, getHomeDashboard: vi.fn()
  } as VisualizationProvider
}

describe('visualization provider contracts and registry', () => {
  it('keeps neutral summary fields primary while accepting readonly aliases', () => {
    const value: VisualizationDashboardSummary = {
      id: 'd', name: 'D', description: null, thumbnail: null, version: 1, published: true, isPublished: true,
      home: false, homeFlag: false, projectId: 'p', createdAt: timestamp, updatedAt: timestamp
    }
    expect(value).toMatchObject({ description: null, published: true, home: false, isPublished: true, homeFlag: false })
  })

  it('rejects duplicate provider registrations without replacing the original', () => {
    const registry = new VisualizationProviderRegistry()
    const first = providerStub('same')
    expect(registry.register(first)).toBe(true)
    expect(registry.register(providerStub('same'))).toBe(false)
    expect(registry.get('same')).toBe(first)
    expect(registry.ids()).toEqual(['same'])
  })
})

describe('visualization provider facade and composition', () => {
  it('resolves explicit local compatibility routes without probing the external provider', () => {
    expect(resolveVisualizationProviderId({ provider: 'native' })).toBe('native-board')
    expect(resolveVisualizationProviderId({ provider: 'local' })).toBe('native-board')
    expect(resolveVisualizationProviderId({ provider: 'native-board' })).toBe('native-board')
    expect(resolveVisualizationProviderId({ projectId: 'native-boards' })).toBe('native-board')
    expect(resolveVisualizationProviderId({ provider: 'legacy-thingsvis', projectId: 'project-1' })).toBe('legacy-thingsvis')
  })

  it('uses Native for an unqualified compatibility route unless the optional profile is enabled', () => {
    vi.stubEnv('VITE_ENABLE_THINGSVIS_COMPAT', '')
    expect(resolveVisualizationProviderId({})).toBe('native-board')

    vi.stubEnv('VITE_ENABLE_THINGSVIS_COMPAT', 'Y')
    expect(resolveVisualizationProviderId({})).toBe('legacy-thingsvis')
    vi.unstubAllEnvs()
  })

  it('selects the default native provider and rejects invalid selections', async () => {
    const registry = new VisualizationProviderRegistry()
    const selected = providerStub('native-board')
    registry.register(selected)
    expect(createVisualizationProviderFacade(registry).id).toBe('native-board')
    expect(createVisualizationProviderFacade(registry, { providerId: 'missing' }).selectionError?.code).toBe('unknown-provider')
    expect(createVisualizationProviderFacade(registry, { providerId: null }).selectionError?.code).toBe('unknown-provider')
    expect(createVisualizationProviderFacade(registry, { context: { available: false } }).selectionError?.code).toBe('provider-unavailable')
    expect(createVisualizationProviderFacade(registry, { context: { authenticated: false } }).selectionError).toMatchObject({ code: 'provider-unauthenticated', status: 401 })
    expect(createVisualizationProviderFacade(registry, { expectedOwnerId: 'a', context: { ownerId: 'b' } }).selectionError?.code).toBe('ownership-mismatch')
  })

  it('normalizes thrown provider operations and does not invoke operations after selection failure', async () => {
    const registry = new VisualizationProviderRegistry()
    registry.register(providerStub())
    const thrown = await createVisualizationProviderFacade(registry, { providerId: 'stub' }).execute(async () => {
      throw new Error('boom')
    })
    expect(thrown).toMatchObject({ ok: false, error: { code: 'provider-failure' } })
    const operation = vi.fn()
    const rejected = await createVisualizationProviderFacade(registry, { providerId: 'missing' }).execute(operation)
    expect(rejected).toMatchObject({ ok: false, error: { code: 'unknown-provider' } })
    expect(operation).not.toHaveBeenCalled()
  })

  it('composes one stable default registry, provider and facade', () => {
    const first = registerDefaultVisualizationProviders()
    expect(first).toBe(getDefaultVisualizationProviderRegistry())
    expect(first.ids()).toEqual(['native-board', 'legacy-thingsvis'])
    expect(first.get('native-board')?.deploymentMode).toBe('local-default')
    expect(first.get('legacy-thingsvis')?.deploymentMode).toBe('optional-external')
    expect(getDefaultVisualizationProviderFacade().id).toBe('native-board')
    expect(getDefaultVisualizationProviderFacade({ providerId: 'legacy-thingsvis' }).selectionError?.code)
      .toBe('external-blocked')

    vi.stubEnv('VITE_ENABLE_THINGSVIS_COMPAT', 'Y')
    expect(getDefaultVisualizationProviderFacade({ providerId: 'legacy-thingsvis' }).id).toBe('legacy-thingsvis')
    vi.unstubAllEnvs()
  })
})

describe('legacy ThingsVis adapter', () => {
  beforeEach(() => vi.clearAllMocks())

  it('maps canonical project pages and dashboard summaries including compatibility aliases', async () => {
    legacy.getThingsVisProjects.mockResolvedValue({ data: page([project()]), error: null })
    legacy.getThingsVisDashboards.mockResolvedValue({ data: page([dashboard()]), error: null })
    const projects = await legacyThingsVisProvider.listProjects()
    const dashboards = await legacyThingsVisProvider.listDashboards({ projectId: 'project-1' })
    expect(projects).toMatchObject({ ok: true, data: { items: [{ id: 'project-1', dashboardCount: 2 }], page: 1, total: 1 } })
    expect(dashboards).toMatchObject({
      ok: true,
      data: { items: [{ description: null, version: 2, published: true, isPublished: true, home: false, homeFlag: false }] }
    })
    expect(legacy.getThingsVisDashboards).toHaveBeenCalledWith({ projectId: 'project-1' })
  })

  it('maps dashboard schema and null thumbnails', async () => {
    legacy.getThingsVisDashboard.mockResolvedValue({ data: dashboard(), error: null })
    legacy.getThingsVisDashboardThumbnail.mockResolvedValue({ data: { thumbnail: null }, error: null })
    expect(await legacyThingsVisProvider.getDashboard('dashboard-1')).toMatchObject({
      ok: true, data: { description: null, published: true, canvasConfig: { width: 1920 }, nodes: [] }
    })
    expect(await legacyThingsVisProvider.getDashboardThumbnail('dashboard-1')).toEqual({ ok: true, data: null })
  })

  it('does not send the unsupported neutral description field to ThingsVis', async () => {
    legacy.createThingsVisDashboard.mockResolvedValue({ data: dashboard(), error: null })
    legacy.updateThingsVisDashboard.mockResolvedValue({ data: dashboard(), error: null })

    await legacyThingsVisProvider.createDashboard({ name: 'Dashboard', description: 'Local only', projectId: 'project-1' })
    await legacyThingsVisProvider.updateDashboard('dashboard-1', { name: 'Changed', description: 'Local only' })

    expect(legacy.createThingsVisDashboard).toHaveBeenCalledWith({ name: 'Dashboard', projectId: 'project-1' })
    expect(legacy.updateThingsVisDashboard).toHaveBeenCalledWith('dashboard-1', { name: 'Changed' })
  })

  it('rejects invalid project lists and maps 401 errors', async () => {
    legacy.getThingsVisProjects.mockResolvedValueOnce({ data: { data: [project({ createdAt: undefined })], meta: {} }, error: null })
    expect(await legacyThingsVisProvider.listProjects()).toMatchObject({ ok: false, error: { code: 'invalid-response' } })
    legacy.getThingsVisProject.mockResolvedValue({ data: null, error: { status: 401, message: 'login' } })
    expect(await legacyThingsVisProvider.getProject('project-1')).toMatchObject({
      ok: false, error: { code: 'provider-unauthenticated', status: 401 }
    })
  })

  it('accepts nested null home and rejects malformed home responses', async () => {
    legacy.getThingsVisHomeDashboard.mockResolvedValueOnce({ data: { data: null }, error: null })
    expect(await legacyThingsVisProvider.getHomeDashboard()).toEqual({ ok: true, data: null })
    legacy.getThingsVisHomeDashboard.mockResolvedValueOnce({ data: dashboard(), error: null })
    expect(await legacyThingsVisProvider.getHomeDashboard()).toMatchObject({ ok: false, error: { code: 'invalid-response' } })
  })
})

describe('in-memory local visualization provider', () => {
  it('supports project/dashboard CRUD, home, publish, duplicate and guarded deletion', async () => {
    let tick = 0
    const provider = createInMemoryLocalVisualizationProvider({ now: () => `2026-08-01T00:00:0${tick++}.000Z` })
    const createdProject = await provider.createProject({ name: 'Project', description: 'Description' })
    expect(createdProject.ok).toBe(true)
    if (!createdProject.ok) return
    const projectId = createdProject.data.id
    expect(await provider.updateProject(projectId, { name: 'Updated', thumbnail: 'thumb' })).toMatchObject({ ok: true, data: { name: 'Updated', thumbnail: 'thumb' } })

    const rendererData = { version: 1, columns: 24, widgets: [] }
    const created = await provider.createDashboard({
      name: 'Dashboard',
      description: 'Local description',
      projectId,
      nodes: [{ id: 'node-1' }],
      rendererData
    })
    expect(created).toMatchObject({ ok: true, data: { description: 'Local description' } })
    if (!created.ok) return
    const dashboardId = created.data.id
    expect(await provider.deleteProject(projectId)).toMatchObject({ ok: false, error: { code: 'provider-failure' } })
    expect(await provider.updateDashboard(dashboardId, { name: 'Changed', description: 'Updated description' })).toMatchObject({
      ok: true,
      data: { name: 'Changed', description: 'Updated description', version: 2 }
    })
    expect(await provider.setHomeDashboard(dashboardId)).toEqual({ ok: true, data: undefined })
    expect(await provider.getHomeDashboard()).toMatchObject({ ok: true, data: { id: dashboardId } })
    expect(await provider.listDashboards({ projectId })).toMatchObject({
      ok: true,
      data: { items: [{ description: 'Updated description', home: true }] }
    })
    expect(await provider.listDashboards({ projectId, name: ' changed ' })).toMatchObject({
      ok: true,
      data: { total: 1, items: [{ id: dashboardId }] }
    })
    expect(await provider.listDashboards({ projectId, name: 'missing' })).toMatchObject({
      ok: true,
      data: { total: 0, items: [] }
    })
    expect(await provider.publishDashboard(dashboardId)).toMatchObject({ ok: true, data: { published: true, version: 3 } })
    const duplicate = await provider.duplicateDashboard(dashboardId)
    expect(duplicate).toMatchObject({ ok: true, data: { name: 'Changed Copy', published: false, version: 1 } })
    if (!duplicate.ok) return
    expect(await provider.deleteDashboard(duplicate.data.id)).toEqual({ ok: true, data: undefined })
    expect(await provider.unsetHomeDashboard(dashboardId)).toEqual({ ok: true, data: undefined })
    expect(await provider.deleteDashboard(dashboardId)).toEqual({ ok: true, data: undefined })
    expect(await provider.deleteProject(projectId)).toEqual({ ok: true, data: undefined })
  })

  it('returns isolated clones and rejects missing parent resources', async () => {
    const provider = createInMemoryLocalVisualizationProvider()
    expect(await provider.createDashboard({ name: 'Orphan', projectId: 'missing' })).toMatchObject({ ok: false })
    const projectResult = await provider.createProject({ name: 'Clone' })
    if (!projectResult.ok) return
    projectResult.data.name = 'Mutated outside'
    expect(await provider.getProject(projectResult.data.id)).toMatchObject({ ok: true, data: { name: 'Clone' } })
  })
})
