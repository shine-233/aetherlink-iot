/**
 * 文件说明：
 * - 编排 ThingsVis editor 模式在 `tv:init` 之后的设备与字段预取。
 * - 负责从 dashboard 配置中收集平台数据源、预加载设备详情、注册运行时设备，并按设备分组补齐字段数据。
 * 维护提示：
 * - editor 预取保持 fire-and-forget 行为，不在初始化链路中等待完成，避免拖慢 iframe ready 后的可编辑状态。
 * - 本模块不启动 WebSocket，也不管理 viewer 补水 timer；viewer 模式的实时补水仍由 `thingsvisViewerHydrationBridge.ts` 负责。
 * 审查建议：
 * - 后续补测试时重点覆盖设备成功预取后先注册再回推、单设备多数据源字段合并读取、异常静默降级三条行为。
 */
import {
  collectPlatformSourceDeviceIds,
  groupPlatformSourceDescriptorsByDevice,
  hydratePlatformSourceDescriptorGroup,
  type PlatformSourceDescriptor
} from '@/components/thingsvis/thingsvisFieldHydrationBridge'
import type { PlatformDeviceEntry } from '@/components/thingsvis/thingsvisPlatformDeviceCatalogOrchestrator'

type EditorPrefetchOrchestratorOptions = {
  collectConfiguredDescriptors: (dashboardPayload: Record<string, unknown>) => PlatformSourceDescriptor[]
  loadDeviceById: (deviceId: string) => Promise<PlatformDeviceEntry | null>
  registerDevices: (devices: PlatformDeviceEntry[]) => void
  loadRequestedFieldData: (fieldIds: unknown[], deviceId?: string) => Promise<Record<string, unknown>>
  postDeviceById: (payload: { reqId: string; deviceId: string; device: PlatformDeviceEntry }) => void
  postPlatformData: (fields: Record<string, unknown>, deviceId?: string, dataSourceId?: string) => void
  idleTimeout?: number
  fallbackDelay?: number
}

export type ThingsVisEditorPrefetchOrchestrator = {
  prefetch: (dashboardPayload: Record<string, unknown>) => Promise<void>
  dispose: () => void
}

export function createThingsVisEditorPrefetchOrchestrator(
  options: EditorPrefetchOrchestratorOptions
): ThingsVisEditorPrefetchOrchestrator {
  const idleTimeout = options.idleTimeout ?? 1200
  const fallbackDelay = options.fallbackDelay ?? 120

  let prefetchGeneration = 0
  let pendingIdleHandle: number | null = null
  let pendingFallbackTimer: ReturnType<typeof setTimeout> | null = null

  function isCurrentPrefetch(generation: number) {
    return generation === prefetchGeneration
  }

  function clearScheduledPrefetch() {
    if (pendingIdleHandle !== null && typeof window !== 'undefined' && typeof window.cancelIdleCallback === 'function') {
      window.cancelIdleCallback(pendingIdleHandle)
      pendingIdleHandle = null
    }
    if (pendingFallbackTimer) {
      clearTimeout(pendingFallbackTimer)
      pendingFallbackTimer = null
    }
  }

  function schedulePrefetch(callback: () => void) {
    clearScheduledPrefetch()

    if (typeof window !== 'undefined' && typeof window.requestIdleCallback === 'function') {
      pendingIdleHandle = window.requestIdleCallback(
        () => {
          pendingIdleHandle = null
          callback()
        },
        { timeout: idleTimeout }
      )
      return
    }

    pendingFallbackTimer = setTimeout(() => {
      pendingFallbackTimer = null
      callback()
    }, fallbackDelay)
  }

  async function prefetchDevices(deviceIds: string[], generation: number) {
    for (const deviceId of deviceIds) {
      if (!isCurrentPrefetch(generation)) return
      try {
        const device = await options.loadDeviceById(deviceId)
        if (!isCurrentPrefetch(generation)) return
        if (!device) continue

        options.registerDevices([device])
        options.postDeviceById({
          reqId: `__prefetch__${deviceId}`,
          deviceId,
          device
        })
      } catch {
        if (!isCurrentPrefetch(generation)) return
        // editor 预取是体验优化，单个设备失败不能阻断 iframe 初始化。
      }
    }
  }

  async function prefetchDescriptorFields(descriptors: PlatformSourceDescriptor[], generation: number) {
    for (const [deviceId, group] of groupPlatformSourceDescriptorsByDevice(descriptors)) {
      if (!isCurrentPrefetch(generation)) return
      try {
        await hydratePlatformSourceDescriptorGroup({
          deviceId,
          group,
          loadRequestedFieldData: async (fieldIds, targetDeviceId) => {
            if (!isCurrentPrefetch(generation)) return {}
            const fields = await options.loadRequestedFieldData(fieldIds, targetDeviceId)
            return isCurrentPrefetch(generation) ? fields : {}
          },
          postPlatformData: (fields, targetDeviceId, dataSourceId) => {
            if (!isCurrentPrefetch(generation)) return
            options.postPlatformData(fields, targetDeviceId, dataSourceId)
          }
        })
      } catch {
        if (!isCurrentPrefetch(generation)) return
        // 字段补齐失败时保持静默降级，避免 editor 初始化阶段出现噪音。
      }
    }
  }

  async function runPrefetch(dashboardPayload: Record<string, unknown>, generation: number) {
    if (!isCurrentPrefetch(generation)) return

    const descriptors = options.collectConfiguredDescriptors(dashboardPayload)
    await prefetchDevices(collectPlatformSourceDeviceIds(descriptors), generation)
    if (!isCurrentPrefetch(generation)) return

    await prefetchDescriptorFields(descriptors, generation)
  }

  async function prefetch(dashboardPayload: Record<string, unknown>) {
    const generation = prefetchGeneration + 1
    prefetchGeneration = generation

    schedulePrefetch(() => {
      void runPrefetch(dashboardPayload, generation)
    })
  }

  function dispose() {
    prefetchGeneration += 1
    clearScheduledPrefetch()
  }

  return {
    prefetch,
    dispose
  }
}
