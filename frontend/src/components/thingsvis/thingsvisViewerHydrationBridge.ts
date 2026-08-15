/**
 * 文件说明：
 * - 封装 ThingsVis viewer 模式的平台数据补水调度，集中处理延迟触发、进行中保护、完成标记和 dashboard 配置缓存。
 * - AppFrame 只注入 dashboard 读取、字段读取、WebSocket ensure 和 postPlatformData 等副作用能力。
 * 维护提示：
 * - viewer 补水只应在 viewer 模式执行；editor 模式的预取和消息回推仍留在 AppFrame。
 * - dashboard 配置为空时不能标记完成，避免 ThingsVis iframe 比平台配置更早 ready 时丢失后续补水机会。
 * 审查建议：
 * - 后续补测试时应覆盖 schema 直传、接口回退、重复 schedule、reset 后重新补水和 dispose 清理。
 */
import {
  groupPlatformSourceDescriptorsByDevice,
  hydratePlatformSourceDescriptorGroup,
  type PlatformSourceDescriptor
} from '@/components/thingsvis/thingsvisFieldHydrationBridge'

type DashboardSchemaInput =
  | {
      id?: string
      name?: string
      canvasConfig?: Record<string, unknown>
      nodes?: unknown[]
      dataSources?: unknown[]
      variables?: unknown[]
    }
  | null
  | undefined

type ViewerHydrationContext = {
  id: string
  mode?: string
  schema?: DashboardSchemaInput
}

type ViewerHydrationOptions = {
  getContext: () => ViewerHydrationContext
  fetchDashboardWithRetry: (id: string) => Promise<{ data?: any; error?: any }>
  normalizeDashboardConfig: <T>(config: T) => T
  collectConfiguredDescriptors: (config: any) => PlatformSourceDescriptor[]
  ensureDeviceWs: (deviceId: string) => void
  ensureDeviceStatusWs: (deviceId: string) => void
  loadRequestedFieldData: (fieldIds: string[], deviceId: string) => Promise<Record<string, unknown>>
  postPlatformData: (fields: Record<string, unknown>, deviceId?: string, dataSourceId?: string) => void
  onLoadError?: (id: string, error: unknown) => void
  delay?: number
}

export type ThingsVisViewerHydrationBridge = {
  schedule: () => void
  reset: () => void
  dispose: () => void
}

export function createThingsVisViewerHydrationBridge(
  options: ViewerHydrationOptions
): ThingsVisViewerHydrationBridge {
  const delay = options.delay ?? 200

  let hydrationTimers: Array<ReturnType<typeof setTimeout>> = []
  let hydrationInFlight = false
  let hydrationDone = false
  let dashboardConfigCache: Record<string, unknown> | null = null
  let dashboardConfigPromise: Promise<Record<string, unknown> | null> | null = null
  let dashboardConfigCacheId: string | null = null

  function clearTimers() {
    hydrationTimers.forEach((timer) => clearTimeout(timer))
    hydrationTimers = []
  }

  function normalizeSchemaConfig(context: ViewerHydrationContext): Record<string, unknown> | null {
    if (!context.schema) return null
    return options.normalizeDashboardConfig({
      id: context.schema.id || context.id,
      name: context.schema.name,
      canvas: context.schema.canvasConfig,
      nodes: Array.isArray(context.schema.nodes) ? context.schema.nodes : [],
      dataSources: Array.isArray(context.schema.dataSources) ? context.schema.dataSources : [],
      variables: Array.isArray(context.schema.variables) ? context.schema.variables : []
    })
  }

  async function loadDashboardConfig(): Promise<Record<string, unknown> | null> {
    const context = options.getContext()
    if (context.mode !== 'viewer') return null

    const schemaConfig = normalizeSchemaConfig(context)
    if (schemaConfig) return schemaConfig

    if (dashboardConfigCache && dashboardConfigCacheId === context.id) return dashboardConfigCache
    if (dashboardConfigPromise) return dashboardConfigPromise

    dashboardConfigPromise = (async () => {
      try {
        const { data, error } = await options.fetchDashboardWithRetry(context.id)
        if (error || !data) return null

        dashboardConfigCacheId = context.id
        dashboardConfigCache = options.normalizeDashboardConfig({
          id: data.id,
          name: data.name,
          canvas: data.canvasConfig,
          nodes: Array.isArray(data.nodes) ? data.nodes : [],
          dataSources: Array.isArray(data.dataSources) ? data.dataSources : [],
          variables: Array.isArray(data.variables) ? data.variables : []
        })
        return dashboardConfigCache
      } catch (error) {
        options.onLoadError?.(context.id, error)
        return null
      } finally {
        dashboardConfigPromise = null
      }
    })()

    return dashboardConfigPromise
  }

  async function hydrateConfiguredSources(): Promise<boolean> {
    const config = await loadDashboardConfig()
    if (!config) return false

    const descriptors = options.collectConfiguredDescriptors(config)
    if (descriptors.length === 0) return true

    for (const [deviceId, group] of groupPlatformSourceDescriptorsByDevice(descriptors)) {
      await hydratePlatformSourceDescriptorGroup({
        deviceId,
        group,
        ensureDeviceWs: options.ensureDeviceWs,
        ensureDeviceStatusWs: options.ensureDeviceStatusWs,
        loadRequestedFieldData: options.loadRequestedFieldData,
        postPlatformData: options.postPlatformData
      })
    }

    return true
  }

  function schedule() {
    const context = options.getContext()
    if (context.mode !== 'viewer') return
    if (hydrationDone || hydrationInFlight) return

    clearTimers()

    const timer = setTimeout(async () => {
      if (hydrationDone || hydrationInFlight) return

      hydrationInFlight = true
      try {
        const hasConfig = await hydrateConfiguredSources()
        if (hasConfig) {
          hydrationDone = true
        }
      } finally {
        hydrationInFlight = false
      }
    }, delay)

    hydrationTimers.push(timer)
  }

  function reset() {
    clearTimers()
    hydrationInFlight = false
    hydrationDone = false
  }

  function dispose() {
    reset()
    dashboardConfigCache = null
    dashboardConfigCacheId = null
    dashboardConfigPromise = null
  }

  return {
    schedule,
    reset,
    dispose
  }
}
