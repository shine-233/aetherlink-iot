import type {
  CreateVisualizationDashboardPayload,
  CreateVisualizationProjectPayload,
  LocalVisualizationProvider,
  UpdateVisualizationDashboardPayload,
  UpdateVisualizationProjectPayload,
  VisualizationDashboardSchema,
  VisualizationDashboardSummary,
  VisualizationPage,
  VisualizationProject,
  VisualizationResult
} from './contracts'

interface InMemoryVisualizationProviderOptions {
  id?: string
  now?: () => string
}

const success = <T>(data: T): VisualizationResult<T> => ({ ok: true, data })
const failure = <T>(message: string): VisualizationResult<T> => ({
  ok: false,
  error: { code: 'provider-failure', message }
})

function clone<T>(value: T): T {
  return structuredClone(value)
}

function paginate<T>(items: T[], page = 1, limit = 20): VisualizationPage<T> {
  const safePage = Math.max(1, Math.floor(page))
  const safeLimit = Math.max(1, Math.floor(limit))
  const total = items.length
  return {
    items: items.slice((safePage - 1) * safeLimit, safePage * safeLimit),
    page: safePage,
    limit: safeLimit,
    total,
    totalPages: total === 0 ? 0 : Math.ceil(total / safeLimit)
  }
}

function toSummary(dashboard: VisualizationDashboardSchema, home: boolean): VisualizationDashboardSummary {
  return {
    id: dashboard.id,
    name: dashboard.name,
    description: dashboard.description,
    thumbnail: dashboard.thumbnail,
    version: dashboard.version,
    published: dashboard.published,
    publishedAt: dashboard.publishedAt,
    shareToken: dashboard.shareToken,
    home,
    projectId: dashboard.projectId,
    createdAt: dashboard.createdAt,
    updatedAt: dashboard.updatedAt
  }
}

export function createInMemoryLocalVisualizationProvider(
  options: InMemoryVisualizationProviderOptions = {}
): LocalVisualizationProvider {
  const projects = new Map<string, VisualizationProject>()
  const dashboards = new Map<string, VisualizationDashboardSchema>()
  const now = options.now ?? (() => new Date().toISOString())
  let sequence = 0
  let homeDashboardId: string | null = null
  const nextId = (prefix: string) => `${prefix}-${++sequence}`

  return {
    id: options.id ?? 'in-memory-local',
    kind: 'local',
    deploymentMode: 'local-default',

    async listProjects(params) {
      const items = [...projects.values()].map(project => ({
        ...project,
        dashboardCount: [...dashboards.values()].filter(item => item.projectId === project.id).length
      }))
      return success(clone(paginate(items, params?.page, params?.limit)))
    },

    async getProject(id) {
      const project = projects.get(id)
      return project ? success(clone(project)) : failure(`Visualization project not found: ${id}`)
    },

    async createProject(payload: CreateVisualizationProjectPayload) {
      const timestamp = now()
      const project: VisualizationProject = {
        id: nextId('project'),
        name: payload.name,
        description: payload.description ?? null,
        thumbnail: null,
        createdAt: timestamp,
        updatedAt: timestamp,
        dashboardCount: 0
      }
      projects.set(project.id, project)
      return success(clone(project))
    },

    async updateProject(id, payload: UpdateVisualizationProjectPayload) {
      const project = projects.get(id)
      if (!project) return failure(`Visualization project not found: ${id}`)
      const updated: VisualizationProject = {
        ...project,
        ...payload,
        description: payload.description ?? project.description,
        thumbnail: payload.thumbnail ?? project.thumbnail,
        updatedAt: now()
      }
      projects.set(id, updated)
      return success(clone(updated))
    },

    async deleteProject(id) {
      if (!projects.has(id)) return failure(`Visualization project not found: ${id}`)
      if ([...dashboards.values()].some(item => item.projectId === id)) {
        return failure(`Visualization project still contains dashboards: ${id}`)
      }
      projects.delete(id)
      return success(undefined)
    },

    async listDashboards(params) {
      const name = params.name?.trim().toLocaleLowerCase()
      const items = [...dashboards.values()]
        .filter(item => item.projectId === params.projectId)
        .filter(item => !name || item.name.toLocaleLowerCase().includes(name))
        .map(item => toSummary(item, item.id === homeDashboardId))
      return success(clone(paginate(items, params.page, params.limit)))
    },

    async getDashboard(id) {
      const dashboard = dashboards.get(id)
      return dashboard ? success(clone(dashboard)) : failure(`Visualization dashboard not found: ${id}`)
    },

    async getDashboardThumbnail(id) {
      const dashboard = dashboards.get(id)
      return dashboard ? success(dashboard.thumbnail) : failure(`Visualization dashboard not found: ${id}`)
    },

    async createDashboard(payload: CreateVisualizationDashboardPayload) {
      if (!projects.has(payload.projectId)) return failure(`Visualization project not found: ${payload.projectId}`)
      const timestamp = now()
      const dashboard: VisualizationDashboardSchema = {
        id: nextId('dashboard'),
        name: payload.name,
        description: payload.description ?? null,
        thumbnail: null,
        version: 1,
        canvasConfig: {
          mode: payload.canvasConfig?.mode ?? 'fixed',
          width: payload.canvasConfig?.width ?? 1920,
          height: payload.canvasConfig?.height ?? 1080,
          background: payload.canvasConfig?.background ?? null
        },
        nodes: clone(payload.nodes ?? []),
        dataSources: clone(payload.dataSources ?? []),
        variables: clone(payload.variables ?? []),
        rendererData: clone(payload.rendererData),
        published: false,
        publishedAt: null,
        shareToken: null,
        projectId: payload.projectId,
        createdAt: timestamp,
        updatedAt: timestamp
      }
      dashboards.set(dashboard.id, dashboard)
      return success(clone(dashboard))
    },

    async updateDashboard(id, payload: UpdateVisualizationDashboardPayload) {
      const dashboard = dashboards.get(id)
      if (!dashboard) return failure(`Visualization dashboard not found: ${id}`)
      const updated: VisualizationDashboardSchema = {
        ...dashboard,
        ...payload,
        canvasConfig: payload.canvasConfig
          ? clone(payload.canvasConfig as VisualizationDashboardSchema['canvasConfig'])
          : dashboard.canvasConfig,
        nodes: payload.nodes ? clone(payload.nodes) : dashboard.nodes,
        dataSources: payload.dataSources ? clone(payload.dataSources) : dashboard.dataSources,
        variables: payload.variables ? clone(payload.variables) : dashboard.variables,
        rendererData: payload.rendererData === undefined ? dashboard.rendererData : clone(payload.rendererData),
        version: dashboard.version + 1,
        updatedAt: now()
      }
      dashboards.set(id, updated)
      return success(clone(updated))
    },

    async deleteDashboard(id) {
      if (!dashboards.has(id)) return failure(`Visualization dashboard not found: ${id}`)
      dashboards.delete(id)
      if (homeDashboardId === id) homeDashboardId = null
      return success(undefined)
    },

    async publishDashboard(id) {
      const dashboard = dashboards.get(id)
      if (!dashboard) return failure(`Visualization dashboard not found: ${id}`)
      const timestamp = now()
      const published = {
        ...dashboard,
        published: true,
        publishedAt: timestamp,
        shareToken: dashboard.shareToken ?? nextId('share'),
        version: dashboard.version + 1,
        updatedAt: timestamp
      }
      dashboards.set(id, published)
      return success(clone(published))
    },

    async duplicateDashboard(id) {
      const dashboard = dashboards.get(id)
      if (!dashboard) return failure(`Visualization dashboard not found: ${id}`)
      const timestamp = now()
      const duplicate: VisualizationDashboardSchema = {
        ...clone(dashboard),
        id: nextId('dashboard'),
        name: `${dashboard.name} Copy`,
        version: 1,
        published: false,
        publishedAt: null,
        shareToken: null,
        createdAt: timestamp,
        updatedAt: timestamp
      }
      dashboards.set(duplicate.id, duplicate)
      return success(clone(duplicate))
    },

    async setHomeDashboard(id) {
      if (!dashboards.has(id)) return failure(`Visualization dashboard not found: ${id}`)
      homeDashboardId = id
      return success(undefined)
    },

    async unsetHomeDashboard(id) {
      if (!dashboards.has(id)) return failure(`Visualization dashboard not found: ${id}`)
      if (homeDashboardId === id) homeDashboardId = null
      return success(undefined)
    },

    async getHomeDashboard() {
      if (!homeDashboardId) return success(null)
      const dashboard = dashboards.get(homeDashboardId)
      return dashboard ? success(clone(dashboard)) : success(null)
    }
  }
}
