import type { UpdateDashboardData } from '@/service/api/thingsvis'
import { THINGSVIS_HOST_DATA_SOURCE_ID_PREFIX } from '@/utils/thingsvis/constants'

const GENERATED_HOST_DATA_SOURCE_ID_RE = new RegExp(
  `^(?:__platform_.+__|aetherlink_.+|${THINGSVIS_HOST_DATA_SOURCE_ID_PREFIX}.+)$`
)
const HOST_DATA_SOURCE_ID_RE = new RegExp(`^${THINGSVIS_HOST_DATA_SOURCE_ID_PREFIX}.+$`)
const DATA_SOURCE_EXPRESSION_RE = /ds\.([^\s.}]+)\./g

export type HostSaveBridgeOptions = {
  mode?: string
  normalizeCanvasBackground: (background: unknown) => Record<string, unknown>
  normalizeDashboardConfig: <T>(config: T) => T
}

export function collectReferencedDataSourceIds(value: unknown, referencedIds = new Set<string>()): Set<string> {
  if (typeof value === 'string') {
    DATA_SOURCE_EXPRESSION_RE.lastIndex = 0
    let match: RegExpExecArray | null
    while ((match = DATA_SOURCE_EXPRESSION_RE.exec(value))) {
      if (match[1]) referencedIds.add(match[1])
    }
    return referencedIds
  }

  if (Array.isArray(value)) {
    value.forEach((item) => collectReferencedDataSourceIds(item, referencedIds))
    return referencedIds
  }

  if (value && typeof value === 'object') {
    Object.entries(value as Record<string, unknown>).forEach(([key, item]) => {
      if (key === 'dataSourceId' && typeof item === 'string') {
        referencedIds.add(item)
      }
      collectReferencedDataSourceIds(item, referencedIds)
    })
  }

  return referencedIds
}

export function sanitizeDataSourcesForHostSave(
  mode: string | undefined,
  nodes: unknown,
  dataSources: unknown
): unknown[] {
  if (!Array.isArray(dataSources)) return []

  const referencedIds = collectReferencedDataSourceIds(nodes)

  return dataSources
    .filter((dataSource: any) => {
      const id = typeof dataSource?.id === 'string' ? dataSource.id : ''
      if (mode === 'editor' && HOST_DATA_SOURCE_ID_RE.test(id)) return true
      if (!GENERATED_HOST_DATA_SOURCE_ID_RE.test(id)) return true
      return referencedIds.has(id)
    })
    .map((dataSource: any) => {
      if (!dataSource?.__editorAutoManual) return dataSource
      const { __editorAutoManual: _editorAutoManual, mode: _mode, ...rest } = dataSource
      return rest
    })
}

export function buildHostSaveUpdatePayload(
  payload: Record<string, unknown>,
  options: HostSaveBridgeOptions
): UpdateDashboardData {
  const rawConfig = resolveHostSaveRawConfig(payload)
  const config = options.normalizeDashboardConfig(rawConfig) as Record<string, unknown>
  const meta = (config.meta as Record<string, unknown> | undefined) || {}
  const canvas = resolveHostSaveCanvas(config)
  const dataSources = resolveHostSaveDataSources(config)
  const updatePayload: UpdateDashboardData = {}

  if (typeof meta.name === 'string') {
    updatePayload.name = meta.name
  } else if (typeof config.name === 'string') {
    updatePayload.name = config.name
  }
  if (canvas && typeof canvas === 'object') {
    const normalizedCanvas = { ...(canvas as Record<string, unknown>) }
    normalizedCanvas.background = options.normalizeCanvasBackground(normalizedCanvas.background)
    updatePayload.canvasConfig = normalizedCanvas
  }
  if (Array.isArray(config.nodes)) {
    updatePayload.nodes = config.nodes
  }
  if (Array.isArray(dataSources)) {
    updatePayload.dataSources = sanitizeDataSourcesForHostSave(options.mode, config.nodes, dataSources)
  }
  if (config.variables !== undefined) {
    updatePayload.variables = config.variables as unknown[]
  }

  const thumbnail =
    typeof meta.thumbnail === 'string'
      ? meta.thumbnail
      : typeof payload.thumbnail === 'string'
        ? payload.thumbnail
        : undefined

  if (thumbnail !== undefined) {
    updatePayload.thumbnail = thumbnail
  }

  return updatePayload
}

function resolveHostSaveRawConfig(payload: Record<string, unknown>) {
  if (payload.config && typeof payload.config === 'object') {
    return payload.config as Record<string, unknown>
  }
  return payload
}

function resolveHostSaveCanvas(config: Record<string, unknown>) {
  if (config.canvas && typeof config.canvas === 'object') {
    return config.canvas
  }
  if (config.canvasConfig && typeof config.canvasConfig === 'object') {
    return config.canvasConfig
  }
  return null
}

function resolveHostSaveDataSources(config: Record<string, unknown>) {
  if (Array.isArray(config.dataSources)) {
    return config.dataSources
  }
  if (Array.isArray(config.dataBindings)) {
    return config.dataBindings
  }
  return []
}
