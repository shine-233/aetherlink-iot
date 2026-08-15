import type { Ref } from 'vue'
import { bindWindowGuestMessage } from '@/components/thingsvis/hostBridge'
import type { ThingsVisFrameTransportBridge } from '@/components/thingsvis/thingsvisFrameTransportBridge'
import { createThingsVisFrameMessageDispatcher } from '@/components/thingsvis/thingsvisFrameMessageDispatcher'
import { useThingsVisAppFrameLifecycle } from '@/components/thingsvis/thingsvisAppFrameLifecycle'
import { createThingsVisHostActionsBridge } from '@/components/thingsvis/thingsvisHostActionsBridge'
import { sanitizeDataSourcesForHostSave } from '@/components/thingsvis/thingsvisHostSaveBridge'
import { createThingsVisPlatformSessionBridge } from '@/components/thingsvis/thingsvisPlatformSessionBridge'
import {
  normalizeCanvasBackground,
  normalizeDashboardConfig,
  retryThingsVisRequestAfterUnauthorized
} from '@/components/thingsvis/thingsvisDashboardConfigBridge'
import type { ThingsVisHostDiagnostic } from '@/components/thingsvis/thingsvisHostErrorPayload'
import { publishThingsVisDashboard, updateThingsVisDashboard } from '@/service/api/thingsvis'

export type ThingsVisAppFrameSchema = {
  id?: string
  name?: string
  thumbnail?: string | null
  canvasConfig?: Record<string, unknown>
  nodes?: unknown[]
  dataSources?: unknown[]
  variables?: unknown[]
} | null

type ThingsVisAppFrameLogger = {
  error: (...args: any[]) => void
  warn?: (...args: any[]) => void
  info?: (...args: any[]) => void
  debug?: (...args: any[]) => void
}

export type ThingsVisAppFrameHostRuntimeOptions = {
  iframeRef: Ref<HTMLIFrameElement | undefined>
  token: Ref<string>
  url: Ref<string>
  frameTransportBridge: ThingsVisFrameTransportBridge
  getContext: () => {
    id: string
    mode?: string
    schema?: ThingsVisAppFrameSchema
  }
  onDiagnostic: (diagnostic: ThingsVisHostDiagnostic) => void
  onRecovered: () => void
  onFrameContentHeight?: (height: number) => void
  emitHostSaveSuccess: (payload: { id: string; name?: string }) => void
  logger: ThingsVisAppFrameLogger
  message?: {
    success?: (message: string) => void
    error?: (message: string) => void
    warning?: (message: string) => void
  }
  fallbackAlert: (message: string) => void
  openPreview: (href: string) => void
}

export function useThingsVisAppFrameHostRuntime(options: ThingsVisAppFrameHostRuntimeOptions) {
  const platformSessionBridge = createThingsVisPlatformSessionBridge({
    iframeRef: options.iframeRef,
    frameTransportBridge: options.frameTransportBridge,
    getContext: options.getContext,
    onDiagnostic: options.onDiagnostic,
    onRecovered: options.onRecovered,
    logger: options.logger as { warn: (...args: any[]) => void; error: (...args: any[]) => void }
  })

  const hostActionsBridge = createThingsVisHostActionsBridge({
    getCurrentId: () => options.getContext().id,
    mode: options.getContext().mode,
    normalizeCanvasBackground,
    normalizeDashboardConfig,
    saveDashboard: (id, payload) => retryThingsVisRequestAfterUnauthorized(() => updateThingsVisDashboard(id, payload)),
    publishDashboard: (id) => publishThingsVisDashboard(id),
    resolvePreviewHref: (id) => `/visualization/thingsvis-preview?id=${encodeURIComponent(id)}`,
    openPreview: options.openPreview,
    emitHostSaveSuccess: options.emitHostSaveSuccess,
    logger: options.logger,
    message: options.message,
    fallbackAlert: options.fallbackAlert
  })

  const handleHostSave = async (payload: Record<string, unknown>) => {
    await hostActionsBridge.save(payload)
  }

  const handlePreviewMessage = (projectId: unknown) => {
    hostActionsBridge.preview(projectId)
  }

  const handlePublishMessage = async (projectId: unknown) => {
    await hostActionsBridge.publish(projectId)
  }

  const readHeightValue = (value: unknown): number | null => {
    if (typeof value === 'number' && Number.isFinite(value)) return value
    if (typeof value !== 'string') return null

    const parsed = Number.parseFloat(value)
    return Number.isFinite(parsed) ? parsed : null
  }

  const handleFrameContentHeight = (payload: Record<string, unknown>, raw: Record<string, unknown>) => {
    const height =
      readHeightValue(payload.height) ??
      readHeightValue(payload.contentHeight) ??
      readHeightValue(payload.clientHeight) ??
      readHeightValue(payload.documentHeight) ??
      readHeightValue(raw.height) ??
      readHeightValue(raw.contentHeight) ??
      readHeightValue(raw.clientHeight) ??
      readHeightValue(raw.documentHeight)

    if (height === null) return
    options.onFrameContentHeight?.(height)
  }

  const handleMessage = async (event: MessageEvent) => {
    const message = options.frameTransportBridge.getTrustedThingsVisFrameMessage(event)
    if (!message) return

    await frameMessageDispatcher.dispatch(message)
  }

  const { handleIframeLoad, handleLoadedFrameMessage, handleReadyFrameMessage, handleRequestInitFrameMessage } =
    useThingsVisAppFrameLifecycle({
    iframeRef: options.iframeRef,
    token: options.token,
    url: options.url,
    getId: () => options.getContext().id,
    getMode: () => options.getContext().mode,
    getSchema: () => options.getContext().schema,
    getTargetOrigin: options.frameTransportBridge.getTargetOrigin,
    bindMessageHandler: (handler) => bindWindowGuestMessage(handler),
    handleMessage,
    fetchDashboardWithRetry: platformSessionBridge.lifecycle.fetchDashboardWithRetry,
    normalizeDashboardConfig,
    sanitizeDataSourcesForHostSave: sanitizeDataSourcesForHostSave as unknown as (config: any) => any,
    resolvePlatformBufferSize: platformSessionBridge.lifecycle.resolvePlatformBufferSize,
    resolveRuntimeDeviceId: platformSessionBridge.lifecycle.resolveRuntimeDeviceId,
    scheduleViewerHydration: platformSessionBridge.lifecycle.scheduleViewerHydration,
    prefetchEditor: platformSessionBridge.lifecycle.prefetchEditor,
    disposeEditorPrefetch: platformSessionBridge.lifecycle.disposeEditorPrefetch,
    disconnectAllDeviceWs: platformSessionBridge.lifecycle.disconnectAllDeviceWs,
    resetViewerHydrationState: platformSessionBridge.lifecycle.resetViewerHydrationState,
    disposeViewerHydration: platformSessionBridge.lifecycle.disposeViewerHydration,
    clearPlatformRuntimeDevices: platformSessionBridge.lifecycle.clearPlatformRuntimeDevices,
    clearPlatformDataSourceBindings: platformSessionBridge.lifecycle.clearPlatformDataSourceBindings,
    resetCatalog: platformSessionBridge.lifecycle.resetCatalog,
    onDiagnostic: options.onDiagnostic,
    onRecovered: options.onRecovered,
      logger: options.logger as unknown as {
        warn: (message: string, ...args: any[]) => void
        error: (message: string, ...args: any[]) => void
      }
    })

  const frameMessageDispatcher = createThingsVisFrameMessageDispatcher({
    dashboard: {
      save: handleHostSave,
      platformWrite: platformSessionBridge.dashboard.platformWrite,
      preview: handlePreviewMessage,
      publish: handlePublishMessage
    },
    device: {
      requestFieldData: platformSessionBridge.device.requestFieldData,
      requestDeviceGroups: platformSessionBridge.device.requestDeviceGroups,
      requestDeviceFilterOptions: platformSessionBridge.device.requestDeviceFilterOptions,
      requestDeviceById: platformSessionBridge.device.requestDeviceById,
      requestDevicesByGroup: platformSessionBridge.device.requestDevicesByGroup,
      searchDevicesPaged: platformSessionBridge.device.searchDevicesPaged,
      requestDeviceFields: platformSessionBridge.device.requestDeviceFields
    },
    lifecycle: {
      loaded: handleLoadedFrameMessage,
      ready: handleReadyFrameMessage,
      requestInit: handleRequestInitFrameMessage,
      contentHeight: handleFrameContentHeight
    }
  })

  return {
    handleIframeLoad
  }
}
