/*
 * 文件用途：订阅 ThingsVis 实时遥测和设备状态推送，为嵌入看板提供实时数据源。
 * 核心逻辑：构建 telemetry/status WebSocket 地址，完成设备鉴权、消息解析和字段提取。
 * 关键注意事项：token、deviceId、WebSocket 生命周期和 JSON frame 解析都是关键边界。
 * 重构建议：建议把协议帧解析和 socket 控制器拆分测试。
 */
/**
 * useRealtimePush — tp-03
 * 使用 WebSocket 订阅设备遥测实时数据并推送到 ThingsVis。
 * 仅走 WS 通道；连接异常时自动重连。
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
import { localStg } from '@/utils/storage'
import { getWebsocketServerUrl } from '@/utils/common/tool'

/** ping 间隔。服务端心跳窗口较短，需与现有稳定模块保持一致（8s）。 */
const PING_INTERVAL_MS = 8_000
const WS_RECONNECT_DELAY_MS = 3000

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

interface RealtimeSocketControllerOptions {
  buildUrl: () => string
  getDestroyed: () => boolean
  noTokenMessage: string
  initFailedMessage: string
  errorMessage: string
  closeMessage: string
  onOpen: (socket: WebSocket, token: string) => void
  onMessage: (event: MessageEvent) => void
  onClose?: () => void
  onStop?: () => void
}

interface RealtimeSocketController {
  start: () => void
  stop: () => void
  clearReconnectTimer: () => void
}

interface RealtimeSocketRuntime {
  socket: WebSocket | null
  pingTimer: ReturnType<typeof setInterval> | null
  reconnectTimer: ReturnType<typeof setTimeout> | null
}

interface TelemetryFrameController {
  resetFrameState: () => void
  handleMessage: (event: MessageEvent) => void
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
  handleMessage: (event: MessageEvent) => void
}

interface FrameBatchedPush {
  push: (fields: Record<string, unknown>) => void
  flush: () => void
}

interface PushSocketOptions {
  deviceId: Ref<string>
  getDestroyed: () => boolean
}

interface TelemetryPushSocketOptions extends PushSocketOptions {
  fetchLatest: () => Promise<void>
  frames: TelemetryFrameController
  usingWebSocket: Ref<boolean>
}

interface StatusPushSocketOptions extends PushSocketOptions {
  frames: DeviceStatusFrameController
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

function sendDeviceAuth(socket: WebSocket, deviceId: string, token: string) {
  socket.send(
    JSON.stringify({
      device_id: deviceId,
      token
    })
  )
}

function getAuthToken(): string | undefined {
  return localStg.get('token') as string | undefined
}

function createSocketRuntime(): RealtimeSocketRuntime {
  return {
    socket: null,
    pingTimer: null,
    reconnectTimer: null
  }
}

function clearPingTimer(runtime: RealtimeSocketRuntime) {
  if (runtime.pingTimer) {
    clearInterval(runtime.pingTimer)
    runtime.pingTimer = null
  }
}

function clearReconnectTimer(runtime: RealtimeSocketRuntime) {
  if (runtime.reconnectTimer) {
    clearTimeout(runtime.reconnectTimer)
    runtime.reconnectTimer = null
  }
}

function startPing(runtime: RealtimeSocketRuntime) {
  runtime.pingTimer = setInterval(() => {
    if (runtime.socket?.readyState === WebSocket.OPEN) {
      runtime.socket.send('ping')
    }
  }, PING_INTERVAL_MS)
}

function closeCurrentSocket(runtime: RealtimeSocketRuntime) {
  if (!runtime.socket) return

  runtime.socket.onclose = null
  runtime.socket.close()
  runtime.socket = null
}

function createSocketOrReconnect(
  options: RealtimeSocketControllerOptions,
  scheduleReconnect: () => void
): WebSocket | null {
  try {
    return new WebSocket(options.buildUrl())
  } catch (err) {
    console.warn(options.initFailedMessage, err)
    scheduleReconnect()
    return null
  }
}

function bindRealtimeSocketHandlers(
  runtime: RealtimeSocketRuntime,
  options: RealtimeSocketControllerOptions,
  token: string,
  scheduleReconnect: () => void
) {
  if (!runtime.socket) return

  runtime.socket.onopen = () => {
    if (!runtime.socket) return
    clearReconnectTimer(runtime)
    options.onOpen(runtime.socket, token)
    startPing(runtime)
  }

  runtime.socket.onmessage = options.onMessage
  runtime.socket.onerror = (event) => {
    console.warn(options.errorMessage, event)
  }
  runtime.socket.onclose = (event) => {
    if (options.getDestroyed()) return
    options.onClose?.()
    clearPingTimer(runtime)
    console.warn(options.closeMessage, { code: event.code, reason: event.reason })
    scheduleReconnect()
  }
}

function createRealtimeSocketController(options: RealtimeSocketControllerOptions): RealtimeSocketController {
  const runtime = createSocketRuntime()

  const scheduleReconnect = () => {
    clearReconnectTimer(runtime)
    runtime.reconnectTimer = setTimeout(() => {
      if (!options.getDestroyed()) {
        start()
      }
    }, WS_RECONNECT_DELAY_MS)
  }

  const stop = () => {
    clearPingTimer(runtime)
    clearReconnectTimer(runtime)
    closeCurrentSocket(runtime)
    options.onStop?.()
  }

  const start = () => {
    if (options.getDestroyed()) return
    stop()
    clearReconnectTimer(runtime)

    const token = getAuthToken()
    if (!token) {
      console.warn(options.noTokenMessage)
      scheduleReconnect()
      return
    }

    runtime.socket = createSocketOrReconnect(options, scheduleReconnect)
    bindRealtimeSocketHandlers(runtime, options, token, scheduleReconnect)
  }

  return { start, stop, clearReconnectTimer: () => clearReconnectTimer(runtime) }
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
  const handleMessage = (event: MessageEvent) => {
    try {
      const msg = parseJsonBusinessFrame(event.data)
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

  const handleMessage = (event: MessageEvent) => {
    try {
      const isOnline = parseDeviceOnlineStatus(event.data)
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

function createTelemetryPushSocket({
  deviceId,
  fetchLatest,
  frames,
  getDestroyed,
  usingWebSocket
}: TelemetryPushSocketOptions) {
  return createRealtimeSocketController({
    buildUrl: buildTelemetryWsUrl,
    getDestroyed,
    noTokenMessage: '[useRealtimePush] No auth token, retrying websocket later',
    initFailedMessage: '[useRealtimePush] WebSocket init failed, retrying:',
    errorMessage: '[useRealtimePush] WebSocket error:',
    closeMessage: '[useRealtimePush] WebSocket closed:',
    onStop: () => {
      usingWebSocket.value = false
    },
    onClose: () => {
      usingWebSocket.value = false
    },
    onOpen: (socket, token) => {
      usingWebSocket.value = true
      frames.resetFrameState()
      if (import.meta.env.DEV) {
        console.info('[useRealtimePush] Telemetry WS connected', { deviceId: deviceId.value, url: socket.url })
      }

      // Send device auth after the socket is connected.
      sendDeviceAuth(socket, deviceId.value, token)
      fetchLatest().catch(console.error)
    },
    onMessage: frames.handleMessage
  })
}

function createStatusPushSocket({ deviceId, frames, getDestroyed }: StatusPushSocketOptions) {
  return createRealtimeSocketController({
    buildUrl: buildDeviceStatusWsUrl,
    getDestroyed,
    noTokenMessage: '[useRealtimePush] No auth token for status websocket, retrying later',
    initFailedMessage: '[useRealtimePush] Status WebSocket init failed, retrying:',
    errorMessage: '[useRealtimePush] Device status WebSocket error:',
    closeMessage: '[useRealtimePush] Device status WebSocket closed:',
    onOpen: (socket, token) => {
      frames.resetFrameState()
      if (import.meta.env.DEV) {
        console.info('[useRealtimePush] Device status WS connected', { deviceId: deviceId.value, url: socket.url })
      }

      sendDeviceAuth(socket, deviceId.value, token)
    },
    onMessage: frames.handleMessage
  })
}

export function useRealtimePush(
  deviceId: Ref<string>,
  platformFields: Ref<PlatformField[]>,
  /** 推送单批次字段值到 ThingsVis */
  pushData: (fields: Record<string, unknown>) => void,
  /** 建连后拉一帧当前值，避免等待下一条 WS 才更新 */
  fetchLatest: () => Promise<void>
) {
  let destroyed = false
  const usingWebSocket = ref(false)
  const telemetryPush = createFrameBatchedPush(pushData)
  const telemetryFrames = createTelemetryFrameController({ fetchLatest, platformFields, pushData: telemetryPush.push })
  const statusFrames = createDeviceStatusFrameController(pushData)

  const getDestroyed = () => destroyed
  const telemetrySocket = createTelemetryPushSocket({
    deviceId,
    fetchLatest,
    frames: telemetryFrames,
    getDestroyed,
    usingWebSocket
  })
  const statusSocket = createStatusPushSocket({ deviceId, frames: statusFrames, getDestroyed })

  const clearReconnectTimer = () => {
    telemetrySocket.clearReconnectTimer()
    statusSocket.clearReconnectTimer()
  }

  const start = () => {
    destroyed = false
    clearReconnectTimer()
    telemetrySocket.start()
    statusSocket.start()
  }

  const stop = () => {
    destroyed = true
    clearReconnectTimer()
    telemetrySocket.stop()
    statusSocket.stop()
    telemetryPush.flush()
  }

  return { start, stop, usingWebSocket }
}
