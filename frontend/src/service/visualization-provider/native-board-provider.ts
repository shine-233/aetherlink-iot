import { createBoard, deleteBoard, fetchBoardById, fetchBoards, updateBoard } from '@/service/api/board'
import {
  fetchPublishedBoardByShareToken,
  publishBoard,
  type BoardDetail,
  type UpdateBoardPayload
} from '@/service/api/board'
import {
  addBoardToProject,
  createBoardProject,
  deleteBoardProject,
  fetchBoardProjectById,
  fetchBoardProjectMembership,
  fetchBoardProjects,
  updateBoardProject,
  type BoardProject
} from '@/service/api/board'
import { normalizeLocalDashboard } from '@/components/local-visualization-viewer'
import type {
  LocalVisualizationProvider,
  VisualizationDashboardSchema,
  VisualizationDashboardSummary,
  VisualizationPage,
  VisualizationProject,
  VisualizationResult
} from './contracts'

import { NATIVE_BOARD_PROJECT_ID, NATIVE_BOARD_PROVIDER_ID } from './provider-ids'

// Preserve the established direct import path while keeping the identifiers
// owned by the provider seam instead of one concrete adapter implementation.
export { NATIVE_BOARD_PROJECT_ID, NATIVE_BOARD_PROVIDER_ID } from './provider-ids'

const NATIVE_PROJECT: VisualizationProject = {
  id: NATIVE_BOARD_PROJECT_ID,
  name: 'Native boards',
  description: 'AetherLink native visualization boards',
  thumbnail: null,
  createdAt: '1970-01-01T00:00:00.000Z',
  updatedAt: '1970-01-01T00:00:00.000Z'
}

function projectToVisualization(project: BoardProject): VisualizationProject {
  return {
    id: project.id,
    name: project.name,
    description: project.description ?? null,
    thumbnail: null,
    tenantId: project.tenant_id,
    createdAt: project.created_at,
    updatedAt: project.updated_at
  }
}

const success = <T>(data: T): VisualizationResult<T> => ({ ok: true, data })
const failure = <T>(message: string, cause?: unknown): VisualizationResult<T> => ({
  ok: false,
  error: { code: 'provider-failure', message, cause }
})

function requestError(label: string, error: unknown): VisualizationResult<never> {
  return failure(`${label} failed`, error)
}

function parseBoardConfig(board: BoardDetail): VisualizationResult<unknown> {
  if (board.vis_type !== 'native' || typeof board.config !== 'string') {
    return failure(`Board is not a native visualization: ${board.id}`)
  }
  try {
    const parsed = JSON.parse(board.config) as unknown
    const normalized = normalizeLocalDashboard(parsed)
    return normalized.ok ? success(parsed) : failure(`Native board config is invalid: ${board.id}: ${normalized.error}`)
  } catch (cause) {
    return failure(`Native board config is not valid JSON: ${board.id}`, cause)
  }
}

function boardToDashboard(
  board: BoardDetail,
  projectId: string = NATIVE_BOARD_PROJECT_ID
): VisualizationResult<VisualizationDashboardSchema> {
  const config = parseBoardConfig(board)
  if (!config.ok) return config
  return success({
    id: board.id,
    name: board.name,
    tenantId: board.tenant_id,
    description: board.description ?? null,
    thumbnail: null,
    version: 1,
    canvasConfig: { mode: 'responsive', width: 1920, height: 1080, background: null },
    nodes: [],
    dataSources: [],
    variables: [],
    rendererData: config.data,
    published: board.published === true,
    publishedAt: board.published_at ?? null,
    shareToken: board.share_token ?? null,
    projectId,
    createdAt: board.created_at,
    updatedAt: board.updated_at
  })
}

function boardToSummary(
  board: BoardDetail,
  projectId: string = NATIVE_BOARD_PROJECT_ID
): VisualizationResult<VisualizationDashboardSummary> {
  // The paged board API intentionally returns summary columns and omits config.
  // Do not route list items through the detail converter: a valid board summary
  // must remain listable even when its renderer payload was not selected.
  if (board.vis_type !== 'native') {
    return failure(`Board is not a native visualization: ${board.id}`)
  }
  return success({
    id: board.id,
    name: board.name,
    tenantId: board.tenant_id,
    description: board.description ?? null,
    thumbnail: null,
    version: 1,
    published: board.published === true,
    publishedAt: board.published_at ?? null,
    shareToken: board.share_token ?? null,
    home: board.home_flag === 'Y',
    projectId,
    createdAt: board.created_at,
    updatedAt: board.updated_at
  })
}

function serializeRendererData(value: unknown): VisualizationResult<string> {
  const normalized = normalizeLocalDashboard(value)
  if (!normalized.ok) return failure(`Native dashboard config is invalid: ${normalized.error}`)
  try {
    return success(JSON.stringify(value))
  } catch (cause) {
    return failure('Native dashboard config cannot be serialized', cause)
  }
}

function fullUpdatePayload(board: BoardDetail, overrides: Partial<UpdateBoardPayload> = {}): UpdateBoardPayload {
  return {
    id: board.id,
    name: board.name,
    config: board.config ?? undefined,
    home_flag: board.home_flag,
    menu_flag: board.menu_flag ?? undefined,
    description: board.description ?? undefined,
    remark: board.remark ?? undefined,
    vis_type: 'native',
    ...overrides
  }
}

async function loadNativeBoard(id: string): Promise<VisualizationResult<BoardDetail>> {
  try {
    const { data, error } = await fetchBoardById(id)
    if (error) return requestError(`Load native board ${id}`, error)
    if (!data || data.id !== id || data.vis_type !== 'native') return failure(`Native board not found: ${id}`)
    return success(data)
  } catch (cause) {
    return requestError(`Load native board ${id}`, cause)
  }
}

const unsupported = <T>(message: string): VisualizationResult<T> => ({
  ok: false,
  error: { code: 'unsupported-operation', message }
})
const isNonNeutral = (value: unknown): boolean =>
  value !== undefined &&
  value !== null &&
  (!Array.isArray(value) || value.length > 0) &&
  (typeof value !== 'object' || Array.isArray(value) || Object.keys(value).length > 0)

export const nativeBoardProvider: LocalVisualizationProvider = {
  id: NATIVE_BOARD_PROVIDER_ID,
  kind: 'local',
  deploymentMode: 'local-default',
  capabilities: {
    projects: { list: true, create: true, update: true, delete: true },
    dashboards: {
      thumbnail: false,
      genericLayout: false,
      dataSources: false,
      variables: false,
      publish: true
    }
  },

  async listProjects(params) {
    const page = Math.max(1, Math.floor(params?.page ?? 1))
    const limit = Math.max(1, Math.floor(params?.limit ?? 20))
    try {
      const { data, error } = await fetchBoardProjects()
      if (error) return requestError('List board projects', error)
      if (!data || !Array.isArray(data)) return failure('Invalid board project list response')
      // 内置项目恒为第一项（board_project_members 无归属记录的看板属于它）。
      const items: VisualizationProject[] = [{ ...NATIVE_PROJECT }, ...data.map(projectToVisualization)]
      const total = items.length
      const start = (page - 1) * limit
      return success({
        items: items.slice(start, start + limit),
        page,
        limit,
        total,
        totalPages: total === 0 ? 0 : Math.ceil(total / limit)
      })
    } catch (cause) {
      return requestError('List board projects', cause)
    }
  },

  async getProject(id) {
    if (id === NATIVE_BOARD_PROJECT_ID) return success({ ...NATIVE_PROJECT })
    try {
      const { data, error } = await fetchBoardProjectById(id)
      if (error) return requestError(`Load board project ${id}`, error)
      if (!data) return failure(`Visualization project not found: ${id}`)
      return success(projectToVisualization(data))
    } catch (cause) {
      return requestError(`Load board project ${id}`, cause)
    }
  },

  async createProject(payload) {
    try {
      const { data, error } = await createBoardProject({
        name: payload.name,
        ...(payload.description ? { description: payload.description } : {})
      })
      if (error) return requestError('Create board project', error)
      if (!data) return failure('Invalid create board project response')
      return success(projectToVisualization(data))
    } catch (cause) {
      return requestError('Create board project', cause)
    }
  },

  async updateProject(id, payload) {
    if (id === NATIVE_BOARD_PROJECT_ID) {
      return unsupported('The built-in native project cannot be renamed')
    }
    try {
      // 后端是 PUT 语义（name 必填）；调用方只传 description 时回读当前名称补全。
      let name = payload.name
      if (name === undefined) {
        const current = await fetchBoardProjectById(id)
        if (current.error) return requestError(`Load board project ${id}`, current.error)
        if (!current.data) return failure(`Visualization project not found: ${id}`)
        name = current.data.name
      }
      const { data, error } = await updateBoardProject(id, {
        name,
        ...(payload.description !== undefined ? { description: payload.description } : {})
      })
      if (error) return requestError(`Update board project ${id}`, error)
      if (!data) return failure('Invalid update board project response')
      return success(projectToVisualization(data))
    } catch (cause) {
      return requestError(`Update board project ${id}`, cause)
    }
  },

  async deleteProject(id) {
    if (id === NATIVE_BOARD_PROJECT_ID) {
      return unsupported('The built-in native project cannot be deleted')
    }
    try {
      const { error } = await deleteBoardProject(id)
      return error ? requestError(`Delete board project ${id}`, error) : success(undefined)
    } catch (cause) {
      return requestError(`Delete board project ${id}`, cause)
    }
  },

  async listDashboards(params) {
    // 项目过滤下推后端（project_id=none 表示内置项目）；不存在/空项目由后端返回空列表，
    // 项目本身是否存在由 getProject 校验——列表语义不应携带存在性判断。
    const page = Math.max(1, Math.floor(params.page ?? 1))
    const limit = Math.max(1, Math.floor(params.limit ?? 20))
    const name = params.name?.trim()
    try {
      const { data, error } = await fetchBoards({
        page,
        page_size: limit,
        vis_type: 'native',
        ...(name ? { name } : {}),
        ...(params.tenantId ? { tenant_id: params.tenantId } : {}),
        // 项目过滤在后端完成（成员表解析），分页计数才正确。
        project_id: params.projectId === NATIVE_BOARD_PROJECT_ID ? 'none' : params.projectId
      })
      if (error) return requestError('List native boards', error)
      if (!data || !Array.isArray(data.list) || typeof data.total !== 'number')
        return failure('Invalid native board list response')
      const items: VisualizationDashboardSummary[] = []
      for (const board of data.list) {
        const summary = boardToSummary(board, params.projectId)
        if (!summary.ok) return summary
        items.push(summary.data)
      }
      const result: VisualizationPage<VisualizationDashboardSummary> = {
        items,
        page,
        limit,
        total: data.total,
        totalPages: data.total === 0 ? 0 : Math.ceil(data.total / limit)
      }
      return success(result)
    } catch (cause) {
      return requestError('List native boards', cause)
    }
  },

  async getDashboard(id) {
    const board = await loadNativeBoard(id)
    if (!board.ok) return board
    // 详情逐块解析归属（单次调用，不构成列表 N+1）；不在任何项目即内置项目。
    const membership = await fetchBoardProjectMembership(id)
    if (membership.error) return requestError(`Resolve native board project ${id}`, membership.error)
    const projectId = membership.data ? membership.data.id : NATIVE_BOARD_PROJECT_ID
    return boardToDashboard(board.data, projectId)
  },

  async getDashboardByShareToken(token) {
    try {
      const { data, error } = await fetchPublishedBoardByShareToken(token)
      if (error) return requestError('Load published native board', error)
      if (!data || data.vis_type !== 'native' || data.published !== true) {
        return failure('Published native board not found')
      }
      return boardToDashboard(data)
    } catch (cause) {
      return requestError('Load published native board', cause)
    }
  },

  async getDashboardThumbnail(id) {
    const board = await loadNativeBoard(id)
    return board.ok ? success(null) : board
  },

  async createDashboard(payload) {
    // 项目归属在创建成功后写入（addBoardToProject 会校验项目存在性）；
    // 此处不再提前拒绝非内置项目——那会让项目分组永远无法挂上新看板。
    if ([payload.canvasConfig, payload.nodes, payload.dataSources, payload.variables].some(isNonNeutral)) {
      return unsupported('Native board layout fields are not supported')
    }
    const config = serializeRendererData(payload.rendererData)
    if (!config.ok) return config
    const tenantId = payload.tenantId?.trim()
    try {
      const { data, error } = await createBoard({
        name: payload.name,
        description: payload.description,
        config: config.data,
        home_flag: 'N',
        menu_flag: 'N',
        vis_type: 'native',
        ...(tenantId ? { tenant_id: tenantId } : {})
      })
      if (error) return requestError('Create native board', error)
      if (!data) return failure('Invalid create native board response')
      if (payload.projectId !== NATIVE_BOARD_PROJECT_ID) {
        // 归属写入失败不回滚看板：看板仍在（内置项目下可见），错误如实上报，
        // 调用方可重试归属或删除看板——静默回滚会掩盖半完成状态。
        const joined = await addBoardToProject(payload.projectId, data.id)
        if (joined.error) {
          return requestError(`Assign native board ${data.id} to project ${payload.projectId}`, joined.error)
        }
        return boardToDashboard(data, payload.projectId)
      }
      return boardToDashboard(data)
    } catch (cause) {
      return requestError('Create native board', cause)
    }
  },

  async updateDashboard(id, payload) {
    if (
      [payload.thumbnail, payload.canvasConfig, payload.nodes, payload.dataSources, payload.variables].some(
        isNonNeutral
      )
    ) {
      return unsupported('Native board layout fields are not supported')
    }
    const current = await loadNativeBoard(id)
    if (!current.ok) return current
    let config = current.data.config ?? undefined
    if (payload.rendererData !== undefined) {
      const serialized = serializeRendererData(payload.rendererData)
      if (!serialized.ok) return serialized
      config = serialized.data
    }
    try {
      const { data, error } = await updateBoard(
        fullUpdatePayload(current.data, {
          name: payload.name ?? current.data.name,
          description: payload.description ?? current.data.description ?? undefined,
          config
        })
      )
      if (error) return requestError(`Update native board ${id}`, error)
      if (!data) return failure('Invalid update native board response')
      return boardToDashboard(data)
    } catch (cause) {
      return requestError(`Update native board ${id}`, cause)
    }
  },

  async deleteDashboard(id) {
    try {
      const { error } = await deleteBoard(id)
      return error ? requestError(`Delete native board ${id}`, error) : success(undefined)
    } catch (cause) {
      return requestError(`Delete native board ${id}`, cause)
    }
  },

  async publishDashboard(id) {
    const current = await loadNativeBoard(id)
    if (!current.ok) return current
    if (current.data.published && current.data.share_token) return boardToDashboard(current.data)
    try {
      const { data, error } = await publishBoard(id)
      if (error) return requestError(`Publish native board ${id}`, error)
      if (!data) return failure('Invalid publish native board response')
      return boardToDashboard(data)
    } catch (cause) {
      return requestError(`Publish native board ${id}`, cause)
    }
  },

  async duplicateDashboard(id) {
    const current = await loadNativeBoard(id)
    if (!current.ok) return current
    try {
      const { data, error } = await createBoard({
        name: `${current.data.name} Copy`,
        config: current.data.config ?? undefined,
        home_flag: 'N',
        menu_flag: current.data.menu_flag ?? undefined,
        description: current.data.description ?? undefined,
        remark: current.data.remark ?? undefined,
        vis_type: 'native',
        tenant_id: current.data.tenant_id
      })
      if (error) return requestError(`Duplicate native board ${id}`, error)
      if (!data) return failure('Invalid duplicate native board response')
      return boardToDashboard(data)
    } catch (cause) {
      return requestError(`Duplicate native board ${id}`, cause)
    }
  },

  async setHomeDashboard(id) {
    const current = await loadNativeBoard(id)
    if (!current.ok) return current
    try {
      const { error } = await updateBoard(fullUpdatePayload(current.data, { home_flag: 'Y' }))
      return error ? requestError(`Set native home board ${id}`, error) : success(undefined)
    } catch (cause) {
      return requestError(`Set native home board ${id}`, cause)
    }
  },

  async unsetHomeDashboard(id) {
    const current = await loadNativeBoard(id)
    if (!current.ok) return current
    try {
      const { error } = await updateBoard(fullUpdatePayload(current.data, { home_flag: 'N' }))
      return error ? requestError(`Unset native home board ${id}`, error) : success(undefined)
    } catch (cause) {
      return requestError(`Unset native home board ${id}`, cause)
    }
  },

  async getHomeDashboard(params) {
    try {
      const tenantId = params?.tenantId?.trim()
      const { data, error } = await fetchBoards({
        page: 1,
        page_size: 1,
        home_flag: 'Y',
        vis_type: 'native',
        ...(tenantId ? { tenant_id: tenantId } : {})
      })
      if (error) return requestError('Load native home board', error)
      if (!data || !Array.isArray(data.list)) return failure('Invalid native home board response')
      if (data.list.length === 0) return success(null)
      return boardToDashboard(data.list[0])
    } catch (cause) {
      return requestError('Load native home board', cause)
    }
  }
}
