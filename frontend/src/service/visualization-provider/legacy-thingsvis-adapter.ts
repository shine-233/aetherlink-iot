import {
  createThingsVisDashboard,
  createThingsVisProject,
  deleteThingsVisDashboard,
  deleteThingsVisProject,
  duplicateThingsVisDashboard,
  getThingsVisDashboard,
  getThingsVisDashboards,
  getThingsVisDashboardThumbnail,
  getThingsVisHomeDashboard,
  getThingsVisProject,
  getThingsVisProjects,
  publishThingsVisDashboard,
  setHomeThingsVisDashboard,
  unsetHomeThingsVisDashboard,
  updateThingsVisDashboard,
  updateThingsVisProject
} from '@/service/api/thingsvis'
import { LEGACY_THINGSVIS_PROVIDER_ID } from './provider-ids'
import type {
  ThirdPartyVisualizationProvider,
  VisualizationDashboardSchema,
  VisualizationDashboardSummary,
  VisualizationError,
  VisualizationPage,
  VisualizationProject,
  VisualizationResult
} from './contracts'

type LegacyResult<T> = { data: T | null; error: { message?: string; status?: number } | string | null }

const invalid = (message: string): VisualizationResult<never> => ({
  ok: false,
  error: { code: 'invalid-response', message }
})

function legacyError(error: LegacyResult<unknown>['error']): VisualizationError {
  const status = typeof error === 'object' && error ? error.status : undefined
  return {
    code: status === 401 ? 'provider-unauthenticated' : 'provider-failure',
    message: (typeof error === 'object' && error ? error.message : error) || 'Legacy ThingsVis provider failed',
    status
  }
}

function isRecord(value: unknown): value is Record<string, any> {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}

function requiredString(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0
}

function mapProject(value: unknown): VisualizationProject | null {
  if (!isRecord(value) || !requiredString(value.id) || typeof value.name !== 'string') return null
  if (typeof value.createdAt !== 'string' || typeof value.updatedAt !== 'string') return null
  return {
    id: value.id,
    name: value.name,
    description: typeof value.description === 'string' ? value.description : null,
    thumbnail: typeof value.thumbnail === 'string' ? value.thumbnail : null,
    tenantId: typeof value.tenantId === 'string' ? value.tenantId : undefined,
    ownerId: typeof value.createdById === 'string' ? value.createdById : undefined,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
    dashboardCount: typeof value._count?.dashboards === 'number' ? value._count.dashboards : undefined
  }
}

function mapDashboardSummary(value: unknown): VisualizationDashboardSummary | null {
  if (!isRecord(value) || !requiredString(value.id) || typeof value.name !== 'string') return null
  if (!requiredString(value.projectId) || typeof value.createdAt !== 'string' || typeof value.updatedAt !== 'string') return null
  if (typeof value.version !== 'number' || typeof value.isPublished !== 'boolean' || typeof value.homeFlag !== 'boolean') return null
  return {
    id: value.id,
    name: value.name,
    description: null,
    thumbnail: typeof value.thumbnail === 'string' ? value.thumbnail : null,
    version: value.version,
    published: value.isPublished,
    isPublished: value.isPublished,
    publishedAt: typeof value.publishedAt === 'string' ? value.publishedAt : null,
    shareToken: typeof value.shareToken === 'string' ? value.shareToken : null,
    home: value.homeFlag,
    homeFlag: value.homeFlag,
    projectId: value.projectId,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt
  }
}

function mapDashboard(value: unknown): VisualizationDashboardSchema | null {
  if (!isRecord(value) || !requiredString(value.id) || typeof value.name !== 'string') return null
  if (!isRecord(value.canvasConfig) || !Array.isArray(value.nodes) || !Array.isArray(value.dataSources)) return null
  if (!requiredString(value.projectId) || typeof value.createdAt !== 'string' || typeof value.updatedAt !== 'string') return null
  if (typeof value.version !== 'number' || typeof value.isPublished !== 'boolean') return null
  const canvas = value.canvasConfig
  if (typeof canvas.mode !== 'string' || typeof canvas.width !== 'number' || typeof canvas.height !== 'number') return null
  return {
    id: value.id,
    name: value.name,
    description: null,
    thumbnail: typeof value.thumbnail === 'string' ? value.thumbnail : null,
    version: value.version,
    canvasConfig: {
      mode: canvas.mode,
      width: canvas.width,
      height: canvas.height,
      background: typeof canvas.background === 'string' || isRecord(canvas.background) ? canvas.background : null
    },
    nodes: value.nodes,
    dataSources: value.dataSources,
    variables: Array.isArray(value.variables) ? value.variables : undefined,
    published: value.isPublished,
    publishedAt: typeof value.publishedAt === 'string' ? value.publishedAt : null,
    shareToken: typeof value.shareToken === 'string' ? value.shareToken : null,
    projectId: value.projectId,
    ownerId: typeof value.createdById === 'string' ? value.createdById : undefined,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt
  }
}

type MappedValue<T> = { valid: true; value: T } | { valid: false }

const mapped = <T>(value: T): MappedValue<T> => ({ valid: true, value })
const unmapped = (): MappedValue<never> => ({ valid: false })

async function unwrap<T, U>(request: Promise<LegacyResult<T>>, map: (value: T) => MappedValue<U>, label: string): Promise<VisualizationResult<U>> {
  try {
    const result = await request
    if (result.error) return { ok: false, error: legacyError(result.error) }
    if (result.data === null) return invalid(`Invalid ${label} response`)
    const data = map(result.data)
    return data.valid ? { ok: true, data: data.value } : invalid(`Invalid ${label} response`)
  } catch (cause) {
    return { ok: false, error: { code: 'provider-failure', message: `Legacy ${label} request failed`, cause } }
  }
}

const mapRequired = <T>(map: (value: unknown) => T | null) => (value: unknown): MappedValue<T> => {
  const result = map(value)
  return result === null ? unmapped() : mapped(result)
}

function mapPage<T>(value: unknown, map: (item: unknown) => T | null): VisualizationPage<T> | null {
  if (!isRecord(value) || !Array.isArray(value.data) || !isRecord(value.meta)) return null
  const items = value.data.map(map)
  if (items.some(item => item === null)) return null
  const { page, limit, total, totalPages } = value.meta
  if (![page, limit, total, totalPages].every(item => typeof item === 'number')) return null
  return { items: items as T[], page, limit, total, totalPages }
}

const voidResult = async (request: Promise<LegacyResult<unknown>>): Promise<VisualizationResult<void>> => {
  try {
    const result = await request
    return result.error ? { ok: false, error: legacyError(result.error) } : { ok: true, data: undefined }
  } catch (cause) {
    return { ok: false, error: { code: 'provider-failure', message: 'Legacy ThingsVis request failed', cause } }
  }
}

function withoutUnsupportedDashboardFields<T extends { description?: string }>(payload: T): Omit<T, 'description'> {
  const { description: _description, ...supported } = payload
  return supported
}

export const legacyThingsVisProvider: ThirdPartyVisualizationProvider = {
  id: LEGACY_THINGSVIS_PROVIDER_ID,
  kind: 'third-party',
  deploymentMode: 'optional-external',
  listProjects: params => unwrap(getThingsVisProjects(params), mapRequired(value => mapPage(value, mapProject)), 'project list'),
  getProject: id => unwrap(getThingsVisProject(id), mapRequired(mapProject), 'project'),
  createProject: payload => unwrap(createThingsVisProject(payload), mapRequired(mapProject), 'project'),
  updateProject: (id, payload) => unwrap(updateThingsVisProject(id, payload), mapRequired(mapProject), 'project'),
  deleteProject: id => voidResult(deleteThingsVisProject(id)),
  listDashboards: params => unwrap(getThingsVisDashboards(params), mapRequired(value => mapPage(value, mapDashboardSummary)), 'dashboard list'),
  getDashboard: id => unwrap(getThingsVisDashboard(id), mapRequired(mapDashboard), 'dashboard'),
  getDashboardThumbnail: id => unwrap(getThingsVisDashboardThumbnail(id), value =>
    isRecord(value) && (typeof value.thumbnail === 'string' || value.thumbnail === null)
      ? mapped(value.thumbnail)
      : unmapped(), 'dashboard thumbnail'),
  createDashboard: payload => unwrap(
    createThingsVisDashboard(withoutUnsupportedDashboardFields(payload)),
    mapRequired(mapDashboard),
    'dashboard'
  ),
  updateDashboard: (id, payload) => unwrap(
    updateThingsVisDashboard(id, withoutUnsupportedDashboardFields(payload)),
    mapRequired(mapDashboard),
    'dashboard'
  ),
  deleteDashboard: id => voidResult(deleteThingsVisDashboard(id)),
  publishDashboard: id => unwrap(publishThingsVisDashboard(id), mapRequired(mapDashboard), 'dashboard'),
  duplicateDashboard: id => unwrap(duplicateThingsVisDashboard(id), mapRequired(mapDashboard), 'dashboard'),
  setHomeDashboard: id => voidResult(setHomeThingsVisDashboard(id)),
  unsetHomeDashboard: id => voidResult(unsetHomeThingsVisDashboard(id)),
  getHomeDashboard: async () => {
    try {
      const result = await getThingsVisHomeDashboard()
      if (result.error) return { ok: false, error: legacyError(result.error) }
      if (!isRecord(result.data) || !('data' in result.data)) return invalid('Invalid home dashboard response')
      const nested = result.data.data
      if (nested === null) return { ok: true, data: null }
      const dashboard = mapDashboard(nested)
      return dashboard ? { ok: true, data: dashboard } : invalid('Invalid home dashboard response')
    } catch (cause) {
      return { ok: false, error: { code: 'provider-failure', message: 'Legacy home dashboard request failed', cause } }
    }
  }
}
