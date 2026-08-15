/**
 * 文件说明：
 * - 装配 ThingsVis AppFrame 的平台设备运行时会话，包括设备目录、字段补水、viewer 补水、editor 预取和平台写入回执。
 * - AppFrame 只需要把 lifecycle/device/dashboard 三组小接口接入 iframe 消息和初始化流程。
 * 维护提示：
 * - 本模块持有多类副作用适配器，但不负责可信消息校验、iframe URL 初始化或宿主保存/发布动作。
 * - 清理顺序由 lifecycle 调用方保持；这里仅暴露稳定动作，避免把 runtime/cache/WS 细节散回父组件。
 */
import type { Ref } from 'vue'
import { $t } from '@/locales'
import { createThingsVisViewerHydrationBridge } from '@/components/thingsvis/thingsvisViewerHydrationBridge'
import { createThingsVisEditorPrefetchOrchestrator } from '@/components/thingsvis/thingsvisEditorPrefetchOrchestrator'
import {
  createThingsVisFieldRequestHydrationBridge,
  type FieldRequestHydrationDependencies
} from '@/components/thingsvis/thingsvisFieldRequestHydrationBridge'
import { createThingsVisPlatformDeviceCatalogOrchestrator } from '@/components/thingsvis/thingsvisPlatformDeviceCatalogOrchestrator'
import { createThingsVisPlatformDeviceHostBridge } from '@/components/thingsvis/thingsvisPlatformDeviceHostBridge'
import { dashboardDataFromSchema } from '@/components/thingsvis/thingsvisInitBridge'
import { createThingsVisAppFrameRuntimeDeviceBridge } from '@/components/thingsvis/thingsvisAppFrameRuntimeDeviceBridge'
import { postRawThingsVisFrameMessage } from '@/components/thingsvis/thingsvisFrameBridge'
import type { ThingsVisFrameTransportBridge } from '@/components/thingsvis/thingsvisFrameTransportBridge'
import {
  normalizeDashboardConfig,
  retryThingsVisRequestAfterUnauthorized
} from '@/components/thingsvis/thingsvisDashboardConfigBridge'
import { createThingsVisPlatformWriteReplyBridge } from '@/components/thingsvis/thingsvisPlatformWriteReplyBridge'
import type { ThingsVisHostDiagnostic } from '@/components/thingsvis/thingsvisHostErrorPayload'
import {
  deviceGroupTree,
  deviceList,
  deviceListByGroup,
  deviceAlarmStatus,
  deviceDictProtocolServiceFirstLevel,
  getDeviceConfigList,
  telemetryDataCurrent,
  getAttributeDataSet,
  telemetryDataPub,
  attributeDataPub,
  commandDataPub
} from '@/service/api/device'
import { attributesApi, telemetryApi, commandsApi, eventsApi } from '@/service/api'
import { getTemplat } from '@/service/api/system-data'
import { getThingsVisDashboard } from '@/service/api/thingsvis'
import { rdiDeviceConfig } from '@/service/api/rdi'
import { localStg } from '@/utils/storage'

const EDITOR_TEMPLATE_FIELD_PAGE_SIZE = 1000
const EDITOR_DEVICE_CONFIG_PAGE_SIZE = 200
const EDITOR_GROUP_DEVICE_PAGE_SIZE = 200

type ThingsVisAppFrameSchema =
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

type ThingsVisPlatformSessionContext = {
  id: string
  mode?: string
  schema?: ThingsVisAppFrameSchema
}

type ThingsVisPlatformSessionLogger = {
  warn: (...args: any[]) => void
  error: (...args: any[]) => void
}

export type ThingsVisPlatformSessionBridge = {
  lifecycle: {
    fetchDashboardWithRetry: (id: string) => Promise<any>
    resolvePlatformBufferSize: (dataSources: unknown) => number
    resolveRuntimeDeviceId: (dashboardPayload: Record<string, unknown>) => string | undefined
    scheduleViewerHydration: () => void
    resetViewerHydrationState: () => void
    disposeViewerHydration: () => void
    disposeEditorPrefetch: () => void
    disconnectAllDeviceWs: () => void
    clearPlatformRuntimeDevices: () => void
    clearPlatformDataSourceBindings: () => void
    resetCatalog: () => void
    prefetchEditor: (dashboardPayload: Record<string, unknown>) => Promise<void>
  }
  dashboard: {
    platformWrite: (payload: Record<string, unknown>, requestId?: string) => Promise<void>
  }
  device: {
    requestFieldData: (payload: Record<string, unknown>) => Promise<void>
    requestDeviceGroups: () => Promise<void>
    requestDeviceFilterOptions: (payload: Record<string, unknown>) => Promise<void>
    requestDeviceById: (payload: Record<string, unknown>) => Promise<void>
    requestDevicesByGroup: (payload: Record<string, unknown>) => Promise<void>
    searchDevicesPaged: (payload: Record<string, unknown>) => Promise<void>
    requestDeviceFields: (payload: Record<string, unknown>) => Promise<void>
  }
}

export function createThingsVisPlatformSessionBridge(options: {
  iframeRef: Ref<HTMLIFrameElement | undefined>
  frameTransportBridge: ThingsVisFrameTransportBridge
  getContext: () => ThingsVisPlatformSessionContext
  onDiagnostic?: (diagnostic: ThingsVisHostDiagnostic) => void
  onRecovered?: () => void
  logger: ThingsVisPlatformSessionLogger
}): ThingsVisPlatformSessionBridge {
  const { iframeRef, frameTransportBridge, getContext, logger } = options

  const runtimeDeviceBridge = createThingsVisAppFrameRuntimeDeviceBridge({
    getCurrentDashboardConfig: () => {
      const context = getContext()
      return dashboardDataFromSchema(context.id, context.schema)
    },
    postPlatformData: frameTransportBridge.postPlatformData
  })

  const catalogBridge = createThingsVisPlatformDeviceCatalogOrchestrator({
    dependencies: {
      loadDeviceConfigs: getDeviceConfigList,
      loadServiceCatalog: deviceDictProtocolServiceFirstLevel,
      loadGroupTree: deviceGroupTree,
      listDevices: deviceList,
      listDevicesByGroup: deviceListByGroup,
      loadTemplate: getTemplat,
      loadTelemetry: telemetryApi,
      loadAttributes: attributesApi,
      loadCommands: commandsApi,
      loadEvents: eventsApi
    },
    labels: {
      allGroups: $t('rdi.thingsvis.allDeviceGroups'),
      protocolPrefix: '协议：',
      servicePrefix: '服务：'
    },
    pageSizes: {
      templateField: EDITOR_TEMPLATE_FIELD_PAGE_SIZE,
      deviceConfig: EDITOR_DEVICE_CONFIG_PAGE_SIZE,
      groupDevice: EDITOR_GROUP_DEVICE_PAGE_SIZE
    },
    getLanguage: () => localStg.get('lang') as unknown as string | undefined,
    logger
  })

  const platformWriteReplyBridge = createThingsVisPlatformWriteReplyBridge({
    resolveDeviceId: (payload) => runtimeDeviceBridge.resolveBindingFromPayload(payload).deviceId,
    getDeviceFields: (deviceId) => runtimeDeviceBridge.getDeviceFields(deviceId),
    postResult: (requestId, payload) => {
      if (!requestId) return
      postRawThingsVisFrameMessage(
        iframeRef.value?.contentWindow,
        {
          type: 'tv:platform-write-result',
          requestId,
          ...payload
        },
        frameTransportBridge.getTargetOrigin()
      )
    },
    dependencies: {
      telemetryDataPub,
      attributeDataPub,
      commandDataPub
    },
    logger
  })

  const fieldRequestHydrationBridge = createThingsVisFieldRequestHydrationBridge({
    isFrameReady: () => Boolean(iframeRef.value?.contentWindow),
    resolveBindingFromPayload: (payload) => runtimeDeviceBridge.resolveBindingFromPayload(payload ?? {}),
    ensureDevice: (deviceId) => runtimeDeviceBridge.ensureDevice(deviceId),
    ensureDeviceWs: runtimeDeviceBridge.ensureDeviceWs,
    ensureDeviceStatusWs: runtimeDeviceBridge.ensureDeviceStatusWs,
    postPlatformData: frameTransportBridge.postPlatformData,
    postToThingsVis: frameTransportBridge.postToThingsVis,
    dependencies: {
      loadTelemetryCurrent: telemetryDataCurrent,
      loadAttributeDataSet: getAttributeDataSet,
      loadRdiDeviceConfig: rdiDeviceConfig,
      loadDeviceAlarmStatus: deviceAlarmStatus
    } as FieldRequestHydrationDependencies,
    onAlarmError: (targetDeviceId, error) => {
      logger.warn('[AppFrame] Failed to load requested device alarm status:', targetDeviceId, error)
    }
  })

  async function fetchDashboardWithRetry(id: string) {
    return retryThingsVisRequestAfterUnauthorized(() => getThingsVisDashboard(id))
  }

  const viewerHydrationBridge = createThingsVisViewerHydrationBridge({
    getContext,
    fetchDashboardWithRetry,
    normalizeDashboardConfig,
    collectConfiguredDescriptors: (config) => runtimeDeviceBridge.collectConfiguredDescriptors(config),
    ensureDeviceWs: runtimeDeviceBridge.ensureDeviceWs,
    ensureDeviceStatusWs: runtimeDeviceBridge.ensureDeviceStatusWs,
    loadRequestedFieldData: (fieldIds, deviceId) =>
      fieldRequestHydrationBridge.loadRequestedFieldData(fieldIds, deviceId),
    postPlatformData: frameTransportBridge.postPlatformData,
    onLoadError: (id, error) => {
      logger.warn('[AppFrame] Failed to load viewer dashboard config for hydration:', id, error)
    }
  })

  const editorPrefetchOrchestrator = createThingsVisEditorPrefetchOrchestrator({
    collectConfiguredDescriptors: (dashboardPayload) =>
      runtimeDeviceBridge.collectConfiguredDescriptors(dashboardPayload),
    loadDeviceById: (deviceId) => catalogBridge.loadDeviceById(deviceId),
    registerDevices: (devices) => runtimeDeviceBridge.registerDevices(devices),
    loadRequestedFieldData: (fieldIds, deviceId) =>
      fieldRequestHydrationBridge.loadRequestedFieldData(fieldIds, deviceId),
    postDeviceById: (payload) => frameTransportBridge.postToThingsVis('tv:device-by-id', payload),
    postPlatformData: frameTransportBridge.postPlatformData
  })

  const platformDeviceHostBridge = createThingsVisPlatformDeviceHostBridge({
    catalog: catalogBridge,
    isFrameReady: () => Boolean(iframeRef.value?.contentWindow),
    registerDevices: (devices) => runtimeDeviceBridge.registerDevices(devices),
    updateDeviceFields: runtimeDeviceBridge.updateDeviceFields,
    postToThingsVis: frameTransportBridge.postToThingsVis,
    onDiagnostic: options.onDiagnostic,
    onRecovered: options.onRecovered,
    logger
  })

  return {
    lifecycle: {
      fetchDashboardWithRetry,
      resolvePlatformBufferSize: runtimeDeviceBridge.resolvePlatformBufferSize,
      resolveRuntimeDeviceId: (dashboardPayload) =>
        runtimeDeviceBridge.resolveRuntimeDeviceId(dashboardPayload, getContext().mode),
      scheduleViewerHydration: viewerHydrationBridge.schedule,
      resetViewerHydrationState: viewerHydrationBridge.reset,
      disposeViewerHydration: viewerHydrationBridge.dispose,
      disposeEditorPrefetch: editorPrefetchOrchestrator.dispose,
      disconnectAllDeviceWs: runtimeDeviceBridge.disconnectAllDeviceWs,
      clearPlatformRuntimeDevices: runtimeDeviceBridge.clearDevices,
      clearPlatformDataSourceBindings: runtimeDeviceBridge.clearDataSourceBindings,
      resetCatalog: catalogBridge.reset,
      prefetchEditor: (dashboardPayload) => editorPrefetchOrchestrator.prefetch(dashboardPayload)
    },
    dashboard: {
      platformWrite: platformWriteReplyBridge.handlePlatformWrite
    },
    device: {
      requestFieldData: fieldRequestHydrationBridge.handleFieldDataRequest,
      requestDeviceGroups: platformDeviceHostBridge.requestDeviceGroups,
      requestDeviceFilterOptions: platformDeviceHostBridge.requestDeviceFilterOptions,
      requestDeviceById: platformDeviceHostBridge.requestDeviceById,
      requestDevicesByGroup: platformDeviceHostBridge.requestDevicesByGroup,
      searchDevicesPaged: platformDeviceHostBridge.searchDevicesPaged,
      requestDeviceFields: platformDeviceHostBridge.requestDeviceFields
    }
  }
}
