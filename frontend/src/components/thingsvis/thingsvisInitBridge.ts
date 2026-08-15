type DashboardSchemaInput =
  | {
      id?: string
      name?: string
      thumbnail?: string | null
      canvasConfig?: Record<string, unknown>
      nodes?: unknown[]
      dataSources?: unknown[]
      variables?: unknown[]
    }
  | null
  | undefined

type DashboardData = {
  id: string
  name?: string
  thumbnail?: string | null
  canvasConfig: Record<string, unknown>
  nodes: unknown[]
  dataSources: unknown[]
  variables?: unknown[]
}

type LoadDashboardPayloadForInitOptions = {
  propsId: string
  schema: DashboardSchemaInput
  mode?: string
  fetchDashboardWithRetry: (id: string) => Promise<{ data?: any; error?: any }>
  normalizeDashboardConfig: <T>(config: T) => T
  sanitizeDataSourcesForHostSave: (mode: string | undefined, nodes: unknown, dataSources: unknown) => unknown[]
  onPreloadUnavailable?: (id: string, error: unknown) => void
  onPreloadError?: (id: string, error: unknown) => void
}

type ThingsVisInitConfigOptions = {
  token: string
  platformToken?: string
  thingsvisApiBaseUrl: string
  platformApiBaseUrl: string
  runtimeDeviceId?: string
}

export function hasCompleteDashboardSchema(schema: DashboardSchemaInput): schema is NonNullable<DashboardSchemaInput> {
  if (!schema || !schema.canvasConfig || typeof schema.canvasConfig !== 'object') return false
  if (!Array.isArray(schema.nodes)) return false
  if (!Array.isArray(schema.dataSources)) return false
  return true
}

export function dashboardDataFromSchema(propsId: string, schema: DashboardSchemaInput): DashboardData | null {
  if (!hasCompleteDashboardSchema(schema)) return null

  return {
    id: schema.id || propsId,
    name: schema.name,
    thumbnail: schema.thumbnail ?? null,
    canvasConfig: schema.canvasConfig!,
    nodes: schema.nodes!,
    dataSources: schema.dataSources!,
    variables: schema.variables
  }
}

export function buildDashboardPayloadForInit(
  mode: string | undefined,
  data: DashboardData | Record<string, any>,
  normalizeDashboardConfig: <T>(config: T) => T,
  sanitizeDataSourcesForHostSave: (mode: string | undefined, nodes: unknown, dataSources: unknown) => unknown[]
) {
  const nodes = Array.isArray(data.nodes) ? data.nodes : []

  return normalizeDashboardConfig({
    meta: {
      id: data.id,
      name: data.name,
      thumbnail: data.thumbnail
    },
    canvas: data.canvasConfig,
    nodes,
    dataSources: sanitizeDataSourcesForHostSave(mode, nodes, data.dataSources),
    variables: Array.isArray(data.variables) ? data.variables : []
  })
}

export async function loadDashboardPayloadForInit(
  options: LoadDashboardPayloadForInitOptions
): Promise<Record<string, unknown> | null> {
  try {
    const dashboardData = dashboardDataFromSchema(options.propsId, options.schema)
    const fetched = dashboardData
      ? { data: dashboardData, error: null }
      : await options.fetchDashboardWithRetry(options.propsId)
    const { data, error } = fetched

    if (!error && data) {
      return buildDashboardPayloadForInit(
        options.mode,
        data,
        options.normalizeDashboardConfig,
        options.sanitizeDataSourcesForHostSave
      )
    }

    if (!dashboardData) {
      options.onPreloadUnavailable?.(options.propsId, error)
    }
    return null
  } catch (error) {
    options.onPreloadError?.(options.propsId, error)
    return null
  }
}

export function buildThingsVisInitConfig(options: ThingsVisInitConfigOptions) {
  return {
    mode: 'app',
    saveTarget: 'host',
    token: options.token,
    ...(options.platformToken ? { platformToken: options.platformToken } : {}),
    thingsvisApiBaseUrl: options.thingsvisApiBaseUrl,
    platformApiBaseUrl: options.platformApiBaseUrl,
    ...(options.runtimeDeviceId ? { deviceId: options.runtimeDeviceId } : {})
  }
}

export function buildThingsVisInitMessage(
  dashboardPayload: Record<string, unknown>,
  platformBufferSize: number,
  config: Record<string, unknown>
) {
  return {
    type: 'tv:init',
    payload: {
      platformBufferSize,
      data: dashboardPayload,
      config
    }
  }
}
