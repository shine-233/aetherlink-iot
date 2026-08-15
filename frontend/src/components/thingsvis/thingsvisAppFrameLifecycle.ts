/**
 * 文件说明：
 * - 承接 ThingsVisAppFrame 中 iframe 初始化、`tv:ready` 调度、viewer/editor 分流和卸载清理编排。
 * - 负责 token/url 初始化、dashboard preload、`tv:init` 发送、viewer 补水启动和 editor 预取触发。
 * 维护提示：
 * - 这里属于宿主生命周期编排层，不应重新吸收 transport、安全校验或设备目录业务细节。
 * - `tv:ready` 去重、iframe load reset、viewer/editor 分支和卸载清理顺序都属于敏感契约，修改前要逐条核对。
 */
import { onBeforeUnmount, onMounted, watch, type Ref } from 'vue'
import {
  buildThingsVisInitConfig,
  buildThingsVisInitMessage,
  loadDashboardPayloadForInit as loadDashboardPayloadForInitBridge
} from '@/components/thingsvis/thingsvisInitBridge'
import { createThingsVisInitScheduler } from '@/components/thingsvis/thingsvisInitSchedulerBridge'
import {
  buildThingsVisHostDiagnostic,
  type ThingsVisHostDiagnostic
} from '@/components/thingsvis/thingsvisHostErrorPayload'
import { buildThingsVisFrameUrl } from '@/components/thingsvis/thingsvisFrameBridge'
import { getThingsVisToken } from '@/utils/thingsvis'
import {
  THINGSVIS_COMPAT_PROVIDER,
  getPlatformApiBase,
  getThingsVisApiBase,
  getThingsVisStudioBaseUrl
} from '@/utils/thingsvis/constants'
import { localStg } from '@/utils/storage'

const THINGSVIS_SAVE_TARGET = 'host'

type ThingsVisAppFrameLifecycleOptions = {
  iframeRef: Ref<HTMLIFrameElement | undefined>
  token: Ref<string>
  url: Ref<string>
  getId: () => string
  getMode: () => string | undefined
  getSchema: () => any
  getTargetOrigin: () => string
  bindMessageHandler: (handler: (event: MessageEvent) => void | Promise<void>) => () => void
  handleMessage: (event: MessageEvent) => void | Promise<void>
  fetchDashboardWithRetry: (id: string) => Promise<any>
  normalizeDashboardConfig: (config: any) => any
  sanitizeDataSourcesForHostSave: (config: any) => any
  resolvePlatformBufferSize: (dataSources: unknown) => number
  resolveRuntimeDeviceId: (dashboardPayload: Record<string, unknown>) => string | undefined
  scheduleViewerHydration: () => void
  prefetchEditor: (dashboardPayload: Record<string, unknown>) => Promise<unknown> | void
  disposeEditorPrefetch: () => void
  disconnectAllDeviceWs: () => void
  resetViewerHydrationState: () => void
  disposeViewerHydration: () => void
  clearPlatformRuntimeDevices: () => void
  clearPlatformDataSourceBindings: () => void
  resetCatalog: () => void
  onDiagnostic?: (diagnostic: ThingsVisHostDiagnostic) => void
  onRecovered?: () => void
  logger: {
    warn: (message: string, ...args: any[]) => void
    error: (message: string, ...args: any[]) => void
  }
}

export type ThingsVisAppFrameLifecycle = {
  handleIframeLoad: () => void
  handleLoadedFrameMessage: () => void
  handleReadyFrameMessage: () => void
  handleRequestInitFrameMessage: () => void
}

export function useThingsVisAppFrameLifecycle(options: ThingsVisAppFrameLifecycleOptions): ThingsVisAppFrameLifecycle {
  let unbindWindowGuestMessage: (() => void) | null = null

  function cancelEditorPrefetch() {
    options.disposeEditorPrefetch()
  }

  function startEditorPrefetch(dashboardPayload: Record<string, unknown>) {
    cancelEditorPrefetch()

    void options.prefetchEditor(dashboardPayload)
  }

  function buildInitSignature() {
    return JSON.stringify({
      id: options.getId(),
      mode: options.getMode(),
      token: options.token.value,
      url: options.url.value
    })
  }

  async function loadDashboardPayloadForInit(): Promise<Record<string, unknown> | null> {
    return loadDashboardPayloadForInitBridge({
      propsId: options.getId(),
      schema: options.getSchema(),
      mode: options.getMode(),
      fetchDashboardWithRetry: options.fetchDashboardWithRetry,
      normalizeDashboardConfig: options.normalizeDashboardConfig,
      sanitizeDataSourcesForHostSave: options.sanitizeDataSourcesForHostSave,
      onPreloadUnavailable: (id, error) => {
        options.logger.warn('[AppFrame] Dashboard preload unavailable, deferring init:', id, error)
      },
      onPreloadError: (id, error) => {
        options.logger.warn('[AppFrame] Failed to preload dashboard schema for embed init:', id, error)
      }
    })
  }

  function getPlatformTokenConfig(): { platformToken?: string } {
    const platformToken = localStg.get('token') as string | undefined
    return platformToken ? { platformToken } : {}
  }

  function postInitMessage(
    dashboardPayload: Record<string, unknown>,
    platformBufferSize: number,
    runtimeDeviceId?: string
  ) {
    options.iframeRef.value?.contentWindow?.postMessage(
      buildThingsVisInitMessage(
        dashboardPayload,
        platformBufferSize,
        buildThingsVisInitConfig({
          token: options.token.value,
          platformToken: getPlatformTokenConfig().platformToken,
          thingsvisApiBaseUrl: getThingsVisApiBase(),
          platformApiBaseUrl: getPlatformApiBase(),
          runtimeDeviceId
        })
      ),
      options.getTargetOrigin()
    )
  }

  function afterInitPosted(dashboardPayload: Record<string, unknown>) {
    cancelEditorPrefetch()

    if (options.getMode() === 'viewer') {
      options.scheduleViewerHydration()
      return
    }

    options.disconnectAllDeviceWs()
    startEditorPrefetch(dashboardPayload)
  }

  async function runInitOnce(): Promise<boolean> {
    if (!options.iframeRef.value?.contentWindow || !options.token.value || !options.getId()) return false

    const dashboardPayload = await loadDashboardPayloadForInit()
    if (!dashboardPayload) return false

    const platformBufferSize = Math.max(100, options.resolvePlatformBufferSize(dashboardPayload.dataSources))
    const runtimeDeviceId = options.resolveRuntimeDeviceId(dashboardPayload)

    postInitMessage(dashboardPayload, platformBufferSize, runtimeDeviceId)
    afterInitPosted(dashboardPayload)
    return true
  }

  const initScheduler = createThingsVisInitScheduler({
    canInit: () => Boolean(options.iframeRef.value?.contentWindow && options.token.value && options.getId()),
    getSignature: buildInitSignature,
    runInit: runInitOnce
  })

  function scheduleInit(delay = 150) {
    initScheduler.schedule(delay)
  }

  function handleIframeLoad() {
    cancelEditorPrefetch()
    initScheduler.resetAfterFrameLoad()
  }

  function handleLoadedFrameMessage() {
    if (options.getMode() === 'viewer') {
      options.scheduleViewerHydration()
    }
  }

  function handleReadyFrameMessage() {
    scheduleInit()
  }

  function handleRequestInitFrameMessage() {
    initScheduler.invalidate()
    scheduleInit()
  }

  onMounted(async () => {
    unbindWindowGuestMessage = options.bindMessageHandler(options.handleMessage)

    try {
      const tokenStr = await getThingsVisToken()
      if (tokenStr) {
        options.token.value = tokenStr
        options.url.value = buildThingsVisFrameUrl({
          token: tokenStr,
          mode: options.getMode(),
          studioBaseUrl: getThingsVisStudioBaseUrl(),
          provider: THINGSVIS_COMPAT_PROVIDER,
          saveTarget: THINGSVIS_SAVE_TARGET,
          thingsVisApiBaseUrl: getThingsVisApiBase(),
          platformApiBaseUrl: getPlatformApiBase()
        })
        options.onRecovered?.()
      } else {
        options.logger.warn('[AppFrame] Token acquisition returned null')
        options.onDiagnostic?.(
          buildThingsVisHostDiagnostic('auth', 'thingsvis_token', 'ThingsVis token acquisition returned empty.')
        )
      }
    } catch (error) {
      options.logger.error('[AppFrame] Failed to initialize ThingsVis authentication', error)
      options.onDiagnostic?.(buildThingsVisHostDiagnostic('auth', 'thingsvis_token', error))
    }
  })

  watch(
    () => options.getId(),
    (nextId) => {
      cancelEditorPrefetch()
      options.resetViewerHydrationState()
      if (!nextId || !options.token.value) return
      scheduleInit()
    }
  )

  onBeforeUnmount(() => {
    unbindWindowGuestMessage?.()
    unbindWindowGuestMessage = null
    cancelEditorPrefetch()
    initScheduler.dispose()
    options.disposeViewerHydration()
    options.clearPlatformRuntimeDevices()
    options.resetCatalog()
    options.clearPlatformDataSourceBindings()
    options.disconnectAllDeviceWs()
  })

  return {
    handleIframeLoad,
    handleLoadedFrameMessage,
    handleReadyFrameMessage,
    handleRequestInitFrameMessage
  }
}
