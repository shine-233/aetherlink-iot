<!--
  Embeds a ThingsVis runtime iframe inside the AetherLink frontend.
  The host owns widget bootstrap, message routing, device data requests, and save handoff.
  Host keys, message types, and compatibility aliases are external contracts and must stay stable.
-->
<!--
  文件说明：
  - 在 AetherLink 前端中嵌入 ThingsVis 运行时 iframe，承担宿主层桥接职责。
  - 负责 widget 初始化、host/guest 消息路由、字段读写、历史数据补齐与保存回传。
  维护提示：
  - message type、provider、saveTarget、兼容别名都属于跨系统契约，改动要非常谨慎。
  - 这里同时覆盖编辑态与预览态，任何设备上下文判断都要考虑 thing-model/current-device/dashboard 三种场景。
-->
<template>
  <div ref="container" class="thingsvis-widget-container"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { bindWindowGuestMessage } from '@/components/thingsvis/hostBridge'
import { createThingsVisWidgetFieldHistoryBridge } from '@/components/thingsvis/thingsvisWidgetFieldHistoryBridge'
import { createThingsVisWidgetFieldRequestHandler } from '@/components/thingsvis/thingsvisWidgetFieldRequestBridge'
import { createThingsVisWidgetPlatformWriteHandler } from '@/components/thingsvis/thingsvisWidgetPlatformWriteBridge'
import {
  THINGSVIS_CONTENT_HEIGHT_MESSAGE_TYPES,
  createThingsVisContentHeightReporter,
  readThingsVisContentHeight,
  type ThingsVisContentHeightReporter
} from '@/components/thingsvis/thingsvisContentHeightReporter'
import {
  getThingsVisFieldRoot as getFieldRoot,
  normalizeThingsVisWidgetLoadConfig,
  parseThingsVisFieldBindingExpression as parseFieldBindingExpression
} from '@/components/thingsvis/thingsvisWidgetConfigNormalizer'
import {
  THINGSVIS_WIDGET_TEMPLATE_DEVICE_ID,
  createThingsVisWidgetRuntimeContract
} from '@/components/thingsvis/thingsvisWidgetRuntimeContractBridge'
import { createThingsVisWidgetWriteNormalizer } from '@/components/thingsvis/thingsvisWidgetWriteNormalizer'
import { ThingsVisClient } from '@/utils/thingsvis/sdk/client'
import {
  attributeDataPub,
  commandDataPub,
  deviceAlarmStatus,
  telemetryDataHistoryList,
  telemetryDataPub
} from '@/service/api/device'
import { getThingsVisStudioBaseUrl } from '@/utils/thingsvis/constants'
import { getThingsVisToken } from '@/utils/thingsvis'

const HISTORY_FIELD_SUFFIX = '__history'
const DEVICE_ALARM_STATUS_FIELD_IDS = new Set([
  'device_alarm_active',
  'device_alarm_count',
  'device_alarm_highest_level',
  'latest_device_alarm_title',
  'latest_device_alarm_level',
  'latest_device_alarm_time'
])
const RUNTIME_STATUS_FIELD_IDS = new Set(['is_online', 'online_text', 'online_status_updated_at'])

const clone = <T,>(value: T): T => {
  if (value === undefined || value === null) return value
  if (typeof structuredClone === 'function') {
    try {
      return structuredClone(value)
    } catch {
      // Vue proxies and SDK mocks can contain non-cloneable internals; JSON keeps the widget payload contract plain.
    }
  }
  return JSON.parse(JSON.stringify(value)) as T
}

const props = defineProps<{
  /** Initial configuration (JSON page schema) */
  config: any
  /** Optional: real-time data pushed by the host */
  data?: Record<string, any>
  /** Optional: platform field schema forwarded to the ThingsVis editor */
  platformFields?: any[]
  /** Optional: device entries forwarded to the ThingsVis editor (enables Device Fields in Field Picker) */
  platformDevices?: any[]
  /** Optional: iframe height */
  height?: string
  /** Render mode: 'viewer' (read-only preview) | 'editor' (visual editor) */
  mode?: 'viewer' | 'editor'
  /** Optional: current device ID used to route tv:platform-write back to the platform API */
  deviceId?: string
  /** Optional: ring buffer capacity for current device platform data. */
  bufferSize?: number
}>()

const emit = defineEmits<{
  (e: 'save', config: any): void
  (e: 'change', config: any): void
  (e: 'ready'): void
}>()

const container = ref<HTMLElement | null>(null)
let client: ThingsVisClient | null = null
let platformFieldDataFrame: number | null = null
let widgetConfigFrame: number | null = null
let widgetSchemaFrame: number | null = null
let widgetInitGeneration = 0
let widgetDisposed = false
let contentHeightReporter: ThingsVisContentHeightReporter | null = null
const pendingPlatformFieldData = new Map<string, Record<string, unknown>>()

const runtimeContract = createThingsVisWidgetRuntimeContract({
  getDeviceId: () => props.deviceId,
  getMode: () => props.mode,
  getBufferSize: () => props.bufferSize,
  getPlatformFields: () => props.platformFields,
  getPlatformDevices: () => props.platformDevices,
  cloneValue: clone
})

const getPreviewDeviceId = runtimeContract.getPreviewDeviceId
const getFieldValueTypeMap = runtimeContract.getFieldValueTypeMap
const getFieldDataTypeMap = runtimeContract.getFieldDataTypeMap

const normalizeWriteData = createThingsVisWidgetWriteNormalizer({
  getConfig: () => props.config,
  historyFieldSuffix: HISTORY_FIELD_SUFFIX,
  getFieldValueTypeMap,
  parseFieldBindingExpression
})

const fieldHistoryBridge = createThingsVisWidgetFieldHistoryBridge({
  getConfig: () => props.config,
  historyFieldSuffix: HISTORY_FIELD_SUFFIX,
  templateDeviceId: THINGSVIS_WIDGET_TEMPLATE_DEVICE_ID,
  runtimeStatusFieldIds: RUNTIME_STATUS_FIELD_IDS,
  getFieldDataTypeMap,
  getFieldRoot,
  parseFieldBindingExpression,
  loadTelemetryHistory: telemetryDataHistoryList
})

const getLoadOptions = runtimeContract.getLoadOptions

const flushPlatformFieldData = () => {
  platformFieldDataFrame = null
  if (!client) {
    pendingPlatformFieldData.clear()
    return
  }

  pendingPlatformFieldData.forEach((fields, deviceId) => {
    if (!deviceId) return
    const payload = clone(fields) || {}
    client?.pushPlatformFieldData(payload, deviceId)
  })
  pendingPlatformFieldData.clear()
}

const schedulePlatformFieldDataFlush = () => {
  if (platformFieldDataFrame !== null) return
  platformFieldDataFrame = window.requestAnimationFrame(flushPlatformFieldData)
}

const pushPlatformFieldData = (fields: Record<string, unknown>, deviceId?: string) => {
  if (!client) return
  if (!deviceId) return
  pendingPlatformFieldData.set(deviceId, {
    ...(pendingPlatformFieldData.get(deviceId) || {}),
    ...fields
  })
  schedulePlatformFieldDataFlush()
}

const pushPlatformFieldDataNow = (fields: Record<string, unknown>, deviceId?: string) => {
  pushPlatformFieldData(fields, deviceId)
  if (platformFieldDataFrame !== null) {
    window.cancelAnimationFrame(platformFieldDataFrame)
    platformFieldDataFrame = null
  }
  flushPlatformFieldData()
}

function normalizeLoadConfig(config: any) {
  return normalizeThingsVisWidgetLoadConfig(config, {
    mode: props.mode,
    previewDeviceId: getPreviewDeviceId(),
    fieldValueTypes: getFieldValueTypeMap()
  })
}

const pushPlatformFieldHistory = (
  fieldId: string,
  history: Array<{ value: unknown; ts: number }>,
  deviceId?: string
) => {
  if (!client) return
  if (!deviceId) return
  client.pushPlatformFieldHistory(fieldId, clone(history) || [], deviceId)
}

const isTrustedThingsVisMessageEvent = (event: MessageEvent) => {
  return !!client?.isTrustedMessageEvent(event)
}

const handlePlatformWrite = createThingsVisWidgetPlatformWriteHandler({
  isTrustedMessageEvent: isTrustedThingsVisMessageEvent,
  postMessageToGuest: (message) => client?.postMessageToGuest(message),
  getPreviewDeviceId,
  getFieldDataTypeMap,
  normalizeWriteData,
  publishAttributeData: attributeDataPub,
  publishCommandData: commandDataPub,
  publishTelemetryData: telemetryDataPub
})

const handleFieldDataRequest = createThingsVisWidgetFieldRequestHandler({
  isTrustedMessageEvent: isTrustedThingsVisMessageEvent,
  getPreviewDeviceId,
  historyFieldSuffix: HISTORY_FIELD_SUFFIX,
  templateDeviceId: THINGSVIS_WIDGET_TEMPLATE_DEVICE_ID,
  alarmStatusFieldIds: DEVICE_ALARM_STATUS_FIELD_IDS,
  getCurrentData: () => props.data,
  collectConfiguredHistoryFields: fieldHistoryBridge.collectConfiguredHistoryFields,
  shouldPrefillHistoryForDataSource: fieldHistoryBridge.shouldPrefillHistoryForDataSource,
  fetchTelemetryHistoryField: fieldHistoryBridge.fetchTelemetryHistoryField,
  pushPlatformFieldHistory,
  loadAlarmStatus: (deviceId) => deviceAlarmStatus({ device_id: deviceId, page: 1, page_size: 20 }),
  pushPlatformFieldData: pushPlatformFieldDataNow
})

// Resolve the device context currently bound to this widget.
const getCurrentPlatformDeviceId = () => props.deviceId || getPreviewDeviceId()

const fetchThingsVisUrlToken = async () => {
  try {
    return (await getThingsVisToken()) || ''
  } catch (error) {
    console.error('[ThingsVisWidget] getThingsVisToken failed, continuing without URL token:', error)
    return ''
  }
}

const createThingsVisWidgetClient = (hostElement: HTMLElement, targetUrl: string) => {
  return new ThingsVisClient({
    container: hostElement,
    mode: 'widget',
    url: targetUrl,
    style: {
      height: props.height || '100%',
      minHeight: '400px'
    }
  })
}

const loadCurrentWidgetConfig = () => {
  client?.loadWidgetConfig(
    normalizeLoadConfig(clone(props.config || {})),
    clone(props.platformFields || []),
    getLoadOptions()
  )
}

const updateCurrentWidgetSchema = () => {
  if (props.platformFields) client?.updateSchema(clone(props.platformFields))
}

const scheduleWidgetConfigLoad = () => {
  if (widgetConfigFrame !== null) return
  widgetConfigFrame = window.requestAnimationFrame(() => {
    widgetConfigFrame = null
    loadCurrentWidgetConfig()
  })
}

const scheduleWidgetSchemaUpdate = () => {
  if (widgetSchemaFrame !== null) return
  widgetSchemaFrame = window.requestAnimationFrame(() => {
    widgetSchemaFrame = null
    updateCurrentWidgetSchema()
  })
}

const pushCurrentWidgetData = () => {
  if (props.data) pushPlatformFieldDataNow(props.data, getCurrentPlatformDeviceId())
}

const handleThingsVisClientReady = () => {
  // loadWidgetConfig (tv:init) must run before emit('ready') so parent pushes
  // arrive after the iframe has registered platform data sources.
  loadCurrentWidgetConfig()
  updateCurrentWidgetSchema()
  pushCurrentWidgetData()
  emit('ready')
}

const handleThingsVisSaveConfig = (payload: any) => {
  // Keep accepting both save payload shapes exposed by existing guest runtimes:
  // 1. triggerSave path: payload = { canvas, nodes, dataBindings, thumbnail, meta }
  // 2. request-save path: payload = { config: { meta, canvas, nodes, dataSources } }
  const config = payload?.config || payload
  emit('save', config)
  emit('change', config)
}

const handleThingsVisContentHeight = (payload: any) => {
  const height = readThingsVisContentHeight(payload)
  if (height === null) return

  if (container.value) {
    container.value.style.minHeight = `${Math.ceil(height)}px`
  }

  contentHeightReporter?.report(height, {
    source: 'thingsvis-widget',
    mode: props.mode || 'viewer'
  })
}

const registerThingsVisClientHandlers = () => {
  const handlers: Array<[string, (...args: any[]) => void]> = [
    ['ready', handleThingsVisClientReady],
    ['tv:save-config', handleThingsVisSaveConfig],
    ...THINGSVIS_CONTENT_HEIGHT_MESSAGE_TYPES.map(
      eventName => [eventName, handleThingsVisContentHeight] as [string, (...args: any[]) => void]
    )
  ]

  handlers.forEach(([eventName, handler]) => {
    client?.on(eventName, handler)
  })
}

type WindowMessageHandler = (event: MessageEvent) => void

let unbindWindowMessageHandlers: Array<() => void> = []

const registerWindowMessageHandlers = () => {
  const handlers: WindowMessageHandler[] = [handlePlatformWrite, handleFieldDataRequest]
  unbindWindowMessageHandlers = handlers.map((handler) => bindWindowGuestMessage(handler))
}

const unregisterWindowMessageHandlers = () => {
  unbindWindowMessageHandlers.forEach((unbind) => unbind())
  unbindWindowMessageHandlers = []
}

const startContentHeightReporting = () => {
  contentHeightReporter?.stop()
  contentHeightReporter = createThingsVisContentHeightReporter({
    getElement: () => container.value,
    getExtraPayload: () => ({
      source: 'thingsvis-widget-shell',
      mode: props.mode || 'viewer'
    })
  })
  contentHeightReporter.start()
}

const initializeThingsVisWidget = async () => {
  const hostElement = container.value
  if (!hostElement) return

  widgetDisposed = false
  const initGeneration = ++widgetInitGeneration

  const baseUrl = getThingsVisStudioBaseUrl()
  const token = await fetchThingsVisUrlToken()
  if (widgetDisposed || initGeneration !== widgetInitGeneration || container.value !== hostElement) return

  const targetUrl = runtimeContract.buildWidgetUrl(baseUrl, token)

  client = createThingsVisWidgetClient(hostElement, targetUrl)
  registerThingsVisClientHandlers()
}

const runWhenClientReady = (value: unknown, action: () => void) => {
  if (client?.ready && value) {
    action()
  }
}

onMounted(async () => {
  // Keep host message listeners alive for the lifetime of this component instance.
  registerWindowMessageHandlers()
  startContentHeightReporting()
  await initializeThingsVisWidget()
})

watch(
  () => props.config,
  (newVal) => {
    runWhenClientReady(newVal, scheduleWidgetConfigLoad)
  },
  { deep: true }
)

watch(
  () => props.data,
  (newVal) => {
    runWhenClientReady(newVal, () => {
      pushPlatformFieldData(newVal as Record<string, unknown>, getCurrentPlatformDeviceId())
    })
  },
  { deep: true }
)

watch(
  () => props.platformFields,
  (newVal) => {
    runWhenClientReady(newVal, scheduleWidgetSchemaUpdate)
  },
  { deep: true }
)

// Host -> Guest -> Host save handshake.
const triggerSave = () => {
  client?.requestSave()
}

/**
 * Forward a real-time field value batch to the embedded ThingsVis widget.
 * Uses closure reference to `client` so it always reflects the live instance,
 * unlike the exposed `client` property which is snapshotted as null at setup time.
 */
const pushPlatformData = (fields: Record<string, unknown>, deviceId?: string) => {
  pushPlatformFieldDataNow(fields, deviceId)
}

const destroyThingsVisWidget = () => {
  widgetDisposed = true
  widgetInitGeneration += 1
  if (platformFieldDataFrame !== null) {
    window.cancelAnimationFrame(platformFieldDataFrame)
    platformFieldDataFrame = null
  }
  if (widgetConfigFrame !== null) {
    window.cancelAnimationFrame(widgetConfigFrame)
    widgetConfigFrame = null
  }
  if (widgetSchemaFrame !== null) {
    window.cancelAnimationFrame(widgetSchemaFrame)
    widgetSchemaFrame = null
  }
  pendingPlatformFieldData.clear()
  contentHeightReporter?.stop()
  contentHeightReporter = null
  if (!client) return
  client.destroy()
  client = null
}

onBeforeUnmount(() => {
  unregisterWindowMessageHandlers()
  destroyThingsVisWidget()
})

defineExpose({
  triggerSave,
  client,
  pushPlatformData
})
</script>

<style scoped>
.thingsvis-widget-container {
  width: 100%;
  min-height: 100%;
  position: relative;
  /* Do not clip the guest iframe content. */
  overflow: auto;
  /* Keep a minimum host height so the iframe cannot collapse. */
  min-height: 400px;
}
</style>
