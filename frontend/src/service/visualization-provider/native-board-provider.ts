import { createBoard, deleteBoard, fetchBoardById, fetchBoards, updateBoard } from '@/service/api/board'
import { fetchPublishedBoardByShareToken, publishBoard, type BoardDetail, type UpdateBoardPayload } from '@/service/api/board'
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
    return normalized.ok
      ? success(parsed)
      : failure(`Native board config is invalid: ${board.id}: ${normalized.error}`)
  } catch (cause) {
    return failure(`Native board config is not valid JSON: ${board.id}`, cause)
  }
}

function boardToDashboard(board: BoardDetail): VisualizationResult<VisualizationDashboardSchema> {
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
    projectId: NATIVE_BOARD_PROJECT_ID,
    createdAt: board.created_at,
    updatedAt: board.updated_at
  })
}

function boardToSummary(board: BoardDetail): VisualizationResult<VisualizationDashboardSummary> {
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
    projectId: NATIVE_BOARD_PROJECT_ID,
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
const isNonNeutral = (value: unknown): boolean => value !== undefined
  && value !== null
  && (!Array.isArray(value) || value.length > 0)
  && (typeof value !== 'object' || Array.isArray(value) || Object.keys(value).length > 0)
const unsupportedProject = <T>(): VisualizationResult<T> => unsupported('Native visualization uses one built-in project')

export const nativeBoardProvider: LocalVisualizationProvider = {
  id: NATIVE_BOARD_PROVIDER_ID,
  kind: 'local',
  deploymentMode: 'local-default',

  async listProjects(params) {
    const page = Math.max(1, Math.floor(params?.page ?? 1))
    const limit = Math.max(1, Math.floor(params?.limit ?? 20))
    return success({
      items: page === 1 ? [{ ...NATIVE_PROJECT }] : [],
      page,
      limit,
      total: 1,
      totalPages: 1
    })
  },

  async getProject(id) {
    return id === NATIVE_BOARD_PROJECT_ID ? success({ ...NATIVE_PROJECT }) : failure(`Visualization project not found: ${id}`)
  },

  async createProject() { return unsupportedProject() },
  async updateProject() { return unsupportedProject() },
  async deleteProject() { return unsupportedProject() },

  async listDashboards(params) {
    if (params.projectId !== NATIVE_BOARD_PROJECT_ID) return failure(`Visualization project not found: ${params.projectId}`)
    const page = Math.max(1, Math.floor(params.page ?? 1))
    const limit = Math.max(1, Math.floor(params.limit ?? 20))
    const name = params.name?.trim()
    try {
      const { data, error } = await fetchBoards({
        page,
        page_size: limit,
        vis_type: 'native',
        ...(name ? { name } : {}),
        ...(params.tenantId ? { tenant_id: params.tenantId } : {})
      })
      if (error) return requestError('List native boards', error)
      if (!data || !Array.isArray(data.list) || typeof data.total !== 'number') return failure('Invalid native board list response')
      const items: VisualizationDashboardSummary[] = []
      for (const board of data.list) {
        const summary = boardToSummary(board)
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
    return board.ok ? boardToDashboard(board.data) : board
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
    if (payload.projectId !== NATIVE_BOARD_PROJECT_ID) return failure(`Visualization project not found: ${payload.projectId}`)
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
      return boardToDashboard(data)
    } catch (cause) {
      return requestError('Create native board', cause)
    }
  },

  async updateDashboard(id, payload) {
    if ([payload.thumbnail, payload.canvasConfig, payload.nodes, payload.dataSources, payload.variables].some(isNonNeutral)) {
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
      const { data, error } = await updateBoard(fullUpdatePayload(current.data, {
        name: payload.name ?? current.data.name,
        description: payload.description ?? current.data.description ?? undefined,
        config
      }))
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
