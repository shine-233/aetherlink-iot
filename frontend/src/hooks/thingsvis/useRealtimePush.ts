/*
 * 文件用途：订阅 ThingsVis 实时遥测和设备状态推送，为嵌入看板提供实时数据源。
 * 核心逻辑：构建 telemetry/status WebSocket 地址，完成设备鉴权、消息解析和字段提取；
 *   连接生命周期（心跳、重连退避、销毁）统一交给 createRealtimeClient。
 * 关键注意事项：token、deviceId、WebSocket 生命周期和 JSON frame 解析都是关键边界。
 * 重构建议：建议把协议帧解析和 socket 控制器拆分测试。
 */
/**
 * useRealtimePush — tp-03
 * 使用 WebSocket 订阅设备遥测实时数据并推送到 ThingsVis。
 * 仅走 WS 通道；连接异常时统一退避重连（不再各自维护 3s 定时重连）。
 *
 * WS 端点：/api/v1/telemetry/datas/current/ws
 * 协议流程：
 *   1. 建立连接
 *   2. 客户端发送认证消息 { device_id, token }
 *   3. 服务端首先返回当前遥测属性
 *   4. 随后设备有推送便自动返回新数据
 *   5. 返回数据格式：{"humidity":5,"systime":"...","temperature":16.27}
 *   6. 客户端需发 ping 保持连接（间隔 < 60s）
 */

import { type Ref, ref } from 'vue'
import type { PlatformField } from '@/utils/thingsvis/types'
import { createRealtimeClient } from '@/service/realtime/realtime-socket'
import { getWebsocketServerUrl } from '@/utils/common/tool'

/**
 * 构建遥测 WebSocket URL
 *
 * 统一复用项目已有的 websocket 基地址，避免与 request/baseURL、代理前缀不一致。
 */
function buildTelemetryWsUrl(): string {
  return `${getWebsocketServerUrl()}/telemetry/datas/current/ws`
}

function buildDeviceStatusWsUrl(): string {
  return `${getWebsocketServerUrl()}/device/online/status/ws`
}

function normalizeFlatTelemetryObject(obj: Record<string, unknown>) {
  const fields: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(obj)) {
    if (k === 'systime') continue
    fields[k] = v
  }
  return fields
}

function extractArrayFields(payload: unknown[]) {
  const fields: Record<string, unknown> = {}
  payload.forEach((item) => {
    if (!item || typeof item !== 'object') return
    const key = (item as any).key ?? (item as any).label
    if (!key || key === 'systime') return
    if ((item as any).value !== undefined) fields[key] = (item as any).value
  })
  return fields
}

function extractObjectFields(obj: Record<string, unknown>) {
  if (obj.fields && typeof obj.fields === 'object' && !Array.isArray(obj.fields)) {
    return normalizeFlatTelemetryObject(obj.fields as Record<string, unknown>)
  }

  if (obj.data !== undefined) {
    return extractFields(obj.data)
  }

  if (obj.payload !== undefined) {
    return extractFields(obj.payload)
  }

  return normalizeFlatTelemetryObject(obj)
}

function extractFields(payload: unknown): Record<string, unknown> {
  if (!payload) return {}

  if (Array.isArray(payload)) {
    return extractArrayFields(payload)
  }

  if (typeof payload !== 'object') return {}
  return extractObjectFields(payload as Record<string, unknown>)
}

interface TelemetryFrameController {
  resetFrameState: () => void
  handleMessage: (data: unknown) => void
}

interface TelemetryFrameControllerOptions {
  platformFields: Ref<PlatformField[]>
  pushData: (fields: Record<string, unknown>) => void
  fetchLatest: () => Promise<void>
}

interface TelemetryFrameState {
  loggedFirstBusinessFrame: boolean
  warnedUnmappedPayload: boolean
  businessFrameCount: number
}

interface DeviceStatusFrameController {
  resetFrameState: () => void
  handleMessage: (data: unknown) => void
}

interface FrameBatchedPush {
  push: (fields: Record<string, unknown>) => void
  flush: () => void
}

function parseJsonBusinessFrame(data: unknown): unknown | undefined {
  if (typeof data !== 'string' || data === 'pong') return undefined

  try {
    return JSON.parse(data)
  } catch {
    return undefined
  }
}

function isObjectRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

function mapToPlatformFieldIds(
  rawFields: Record<string, unknown>,
  platformFields: PlatformField[]
): { fields: Record<string, unknown>; matched: boolean } {
  const mapped: Record<string, unknown> = {}
  const fields = platformFields || []
  if (fields.length === 0) {
    return { fields: rawFields, matched: false }
  }

  fields.forEach((field) => {
    const idVal = rawFields[field.id]
    const nameVal = rawFields[field.name]
    if (idVal !== undefined) {
      mapped[field.id] = idVal
    } else if (nameVal !== undefined) {
      mapped[field.id] = nameVal
    }
  })

  // Fallback: if no mapping matched, keep the original payload to avoid dropping data.
  if (Object.keys(mapped).length === 0) {
    return { fields: rawFields, matched: false }
  }
  return { fields: mapped, matched: true }
}

function buildOnlineStatusFields(isOnline: number): Record<string, unknown> {
  return {
    is_online: isOnline,
    online_text: isOnline === 1 ? 'Online' : 'Offline',
    online_status_updated_at: Date.now()
  }
}

function scheduleFrameFlush(callback: () => void): () => void {
  if (typeof window !== 'undefined' && typeof window.requestAnimationFrame === 'function') {
    const handle = window.requestAnimationFrame(callback)
    return () => window.cancelAnimationFrame(handle)
  }

  const handle = setTimeout(callback, 16)
  return () => clearTimeout(handle)
}

function createFrameBatchedPush(pushData: (fields: Record<string, unknown>) => void): FrameBatchedPush {
  let pendingFields: Record<string, unknown> | null = null
  let cancelScheduledFlush: (() => void) | null = null

  const flush = () => {
    cancelScheduledFlush?.()
    cancelScheduledFlush = null
    const fields = pendingFields
    pendingFields = null

    if (fields && Object.keys(fields).length > 0) {
      pushData(fields)
    }
  }

  const push = (fields: Record<string, unknown>) => {
    if (Object.keys(fields).length === 0) return

    pendingFields = {
      ...pendingFields,
      ...fields
    }

    if (!cancelScheduledFlush) {
      cancelScheduledFlush = scheduleFrameFlush(flush)
    }
  }

  return { push, flush }
}

function createTelemetryFrameState(): TelemetryFrameState {
  return {
    loggedFirstBusinessFrame: false,
    warnedUnmappedPayload: false,
    businessFrameCount: 0
  }
}

function resetTelemetryFrameState(state: TelemetryFrameState) {
  state.loggedFirstBusinessFrame = false
  state.warnedUnmappedPayload = false
  state.businessFrameCount = 0
}

function logFirstTelemetryFrame(
  state: TelemetryFrameState,
  fetchLatest: () => Promise<void>,
  rawFields: Record<string, unknown>,
  mappedFields: Record<string, unknown>
) {
  if (state.loggedFirstBusinessFrame) return

  state.loggedFirstBusinessFrame = true
  if (import.meta.env.DEV) {
    console.info('[useRealtimePush] First telemetry frame received', {
      rawKeys: Object.keys(rawFields).slice(0, 12),
      mappedKeys: Object.keys(mappedFields).slice(0, 12)
    })
  }
  fetchLatest().catch(console.error)
}

function logTelemetryProgress(
  businessFrameCount: number,
  rawFields: Record<string, unknown>,
  mappedFields: Record<string, unknown>
) {
  if (!import.meta.env.DEV || businessFrameCount % 10 !== 0) return

  console.info('[useRealtimePush] Telemetry frame progress', {
    count: businessFrameCount,
    lastRawKeys: Object.keys(rawFields).slice(0, 12),
    lastMappedKeys: Object.keys(mappedFields).slice(0, 12)
  })
}

function warnUnmappedTelemetryPayload(
  state: TelemetryFrameState,
  platformFields: Ref<PlatformField[]>,
  rawFields: Record<string, unknown>,
  matched: boolean
) {
  if (state.warnedUnmappedPayload || matched) return

  state.warnedUnmappedPayload = true
  console.warn('[useRealtimePush] Telemetry payload did not map to platformFields', {
    rawKeys: Object.keys(rawFields).slice(0, 12),
    fieldIds: platformFields.value.map((f) => f.id).slice(0, 12),
    fieldNames: platformFields.value.map((f) => f.name).slice(0, 12)
  })
}

function handleTelemetryFields(
  options: TelemetryFrameControllerOptions,
  state: TelemetryFrameState,
  rawFields: Record<string, unknown>
) {
  if (Object.keys(rawFields).length === 0) return

  state.businessFrameCount += 1
  const { fields: mappedFields, matched } = mapToPlatformFieldIds(rawFields, options.platformFields.value || [])

  logFirstTelemetryFrame(state, options.fetchLatest, rawFields, mappedFields)
  logTelemetryProgress(state.businessFrameCount, rawFields, mappedFields)
  warnUnmappedTelemetryPayload(state, options.platformFields, rawFields, matched)
  options.pushData(mappedFields)
}

function createTelemetryFrameController(options: TelemetryFrameControllerOptions): TelemetryFrameController {
  const state = createTelemetryFrameState()

  const resetFrameState = () => resetTelemetryFrameState(state)
  const handleMessage = (data: unknown) => {
    try {
      const msg = parseJsonBusinessFrame(data)
      if (msg === undefined) return

      handleTelemetryFields(options, state, extractFields(msg))
    } catch {
      // ignore non-JSON frames
    }
  }

  return { resetFrameState, handleMessage }
}

function parseDeviceOnlineStatus(data: unknown): number | undefined {
  const msg = parseJsonBusinessFrame(data)
  if (!isObjectRecord(msg) || typeof msg.is_online !== 'number') return undefined

  return msg.is_online
}

function logFirstDeviceStatusFrame(isOnline: number) {
  if (import.meta.env.DEV) {
    console.info('[useRealtimePush] First device status frame received', { is_online: isOnline })
  }
}

function createDeviceStatusFrameController(
  pushData: (fields: Record<string, unknown>) => void
): DeviceStatusFrameController {
  let loggedFirstStatusFrame = false

  const resetFrameState = () => {
    loggedFirstStatusFrame = false
  }

  const handleMessage = (data: unknown) => {
    try {
      const isOnline = parseDeviceOnlineStatus(data)
      if (isOnline === undefined) return

      if (!loggedFirstStatusFrame) {
        loggedFirstStatusFrame = true
        logFirstDeviceStatusFrame(isOnline)
      }

      pushData(buildOnlineStatusFields(isOnline))
    } catch {
      // ignore non-JSON frames
    }
  }

  return { resetFrameState, handleMessage }
}

export function useRealtimePush(
  deviceId: Ref<string>,
  platformFields: Ref<PlatformField[]>,
  /** 推送单批次字段值到 ThingsVis */
  pushData: (fields: Record<string, unknown>) => void,
  /** 建连后拉一帧当前值，避免等待下一条 WS 才更新 */
  fetchLatest: () => Promise<void>
) {
  const usingWebSocket = ref(false)
  const telemetryPush = createFrameBatchedPush(pushData)
  const telemetryFrames = createTelemetryFrameController({ fetchLatest, platformFields, pushData: telemetryPush.push })
  const statusFrames = createDeviceStatusFrameController(pushData)

  const telemetrySocket = createRealtimeClient({
    logTag: 'useRealtimePush:telemetry',
    buildUrl: buildTelemetryWsUrl,
    buildAuthFrame: (token) => JSON.stringify({ device_id: deviceId.value, token }),
    onOpen: () => {
      usingWebSocket.value = true
      telemetryFrames.resetFrameState()
      // 建连后拉一帧当前值，避免等待下一条 WS 才更新
      fetchLatest().catch(console.error)
    },
    onClose: () => {
      usingWebSocket.value = false
    },
    onMessage: telemetryFrames.handleMessage
  })

  const statusSocket = createRealtimeClient({
    logTag: 'useRealtimePush:status',
    buildUrl: buildDeviceStatusWsUrl,
    buildAuthFrame: (token) => JSON.stringify({ device_id: deviceId.value, token }),
    onOpen: () => {
      statusFrames.resetFrameState()
    },
    onMessage: statusFrames.handleMessage
  })

  const start = () => {
    telemetrySocket.start()
    statusSocket.start()
  }

  const stop = () => {
    telemetrySocket.stop()
    statusSocket.stop()
    usingWebSocket.value = false
    telemetryPush.flush()
  }

  return { start, stop, usingWebSocket }
}
