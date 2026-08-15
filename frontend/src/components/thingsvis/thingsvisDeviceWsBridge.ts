/**
 * 文件说明：
 * - 封装 ThingsVis 宿主侧设备 WebSocket 管理，包含实时遥测、在线状态、ping 与断线重连。
 * - 组件层只需要传入 postPlatformData 回调，避免 iframe 编排组件继续承载连接生命周期细节。
 * 维护提示：
 * - WebSocket 初始化 payload、字段映射和 tv:platform-data 响应属于 ThingsVis 宿主协议的一部分。
 * - 后续如增加更多实时通道，应优先扩展本桥接模块，而不是继续膨胀 ThingsVisAppFrame.vue。
 */
import { localStg } from '@/utils/storage'
import { getWebsocketServerUrl } from '@/utils/common/tool'
import { createLogger } from '@/utils/logger'
import type { PlatformField } from '@/utils/thingsvis/types'

const logger = createLogger('ThingsVisDeviceWsBridge')
const PING_INTERVAL_MS = 8_000
const WS_RECONNECT_DELAY_MS = 3_000

export type PlatformDeviceField = Pick<PlatformField, 'id' | 'name' | 'dataType'>

type DeviceWsEntry = {
  ws: WebSocket | null
  pingTimer: ReturnType<typeof setInterval> | null
  reconnectTimer: ReturnType<typeof setTimeout> | null
  destroyed: boolean
  device: { deviceId: string; fields: PlatformDeviceField[] }
}

type DeviceStatusWsEntry = {
  ws: WebSocket | null
  pingTimer: ReturnType<typeof setInterval> | null
  reconnectTimer: ReturnType<typeof setTimeout> | null
  destroyed: boolean
  deviceId: string
}

type ManagedDeviceWsEntry = Pick<
  DeviceWsEntry | DeviceStatusWsEntry,
  'ws' | 'pingTimer' | 'reconnectTimer' | 'destroyed'
>

type ManagedDeviceWsConfig = {
  buildUrl: () => string
  buildInitPayload: (token: string) => string
  onMessage: (event: MessageEvent) => void
  onInitError?: (error: unknown) => void
  onReconnect?: () => void
}

type ThingsVisDeviceWsBridgeOptions = {
  postPlatformData: (fields: Record<string, unknown>, deviceId?: string, dataSourceId?: string) => void
}

function destroyDeviceWsEntry(entry: ManagedDeviceWsEntry) {
  entry.destroyed = true
  if (entry.pingTimer) clearInterval(entry.pingTimer)
  if (entry.reconnectTimer) clearTimeout(entry.reconnectTimer)
  entry.ws?.close()
}

function getDeviceWsToken(): string | undefined {
  return localStg.get('token') as string | undefined
}

function scheduleDeviceWsReconnect(entry: ManagedDeviceWsEntry, openWs: () => void) {
  entry.reconnectTimer = setTimeout(openWs, WS_RECONNECT_DELAY_MS)
}

function clearDeviceWsPingTimer(entry: ManagedDeviceWsEntry) {
  if (!entry.pingTimer) return
  clearInterval(entry.pingTimer)
  entry.pingTimer = null
}

function startDeviceWsPing(entry: ManagedDeviceWsEntry) {
  clearDeviceWsPingTimer(entry)
  entry.pingTimer = setInterval(() => {
    if (entry.ws?.readyState === WebSocket.OPEN) entry.ws.send('ping')
  }, PING_INTERVAL_MS)
}

function parseJsonDeviceWsMessage(event: MessageEvent): unknown | null {
  if (typeof event.data !== 'string' || event.data === 'pong') return null
  try {
    return JSON.parse(event.data)
  } catch {
    return null
  }
}

function runManagedDeviceWs(entry: ManagedDeviceWsEntry, config: ManagedDeviceWsConfig) {
  function openWs() {
    if (entry.destroyed) return

    const token = getDeviceWsToken()
    if (!token) {
      scheduleDeviceWsReconnect(entry, openWs)
      return
    }

    try {
      entry.ws = new WebSocket(config.buildUrl())
    } catch (error) {
      config.onInitError?.(error)
      scheduleDeviceWsReconnect(entry, openWs)
      return
    }

    entry.ws.onopen = () => {
      if (!entry.ws) return
      entry.ws.send(config.buildInitPayload(token))
      startDeviceWsPing(entry)
    }

    entry.ws.onmessage = config.onMessage

    entry.ws.onerror = () => {
      /* reconnect handled by onclose */
    }

    entry.ws.onclose = () => {
      if (entry.destroyed) return
      clearDeviceWsPingTimer(entry)
      config.onReconnect?.()
      scheduleDeviceWsReconnect(entry, openWs)
    }
  }

  openWs()
}

/** Extract flat key-value map from various WS response shapes. */
export function extractWsFields(payload: unknown): Record<string, unknown> {
  if (!payload || typeof payload !== 'object') return {}
  const obj = payload as Record<string, unknown>

  if (obj.fields && typeof obj.fields === 'object' && !Array.isArray(obj.fields)) {
    return extractWsFields(obj.fields)
  }
  if (obj.data !== undefined) return extractWsFields(obj.data)
  if (obj.payload !== undefined) return extractWsFields(obj.payload)

  if (Array.isArray(payload)) {
    const fields: Record<string, unknown> = {}
    ;(payload as Array<{ key?: string; label?: string; value?: unknown }>).forEach((item) => {
      if (!item) return
      const k = item.key ?? item.label
      if (!k || k === 'systime') return
      if (item.value !== undefined) fields[k] = item.value
    })
    return fields
  }

  const fields: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(obj)) {
    if (k !== 'systime') fields[k] = v
  }
  return fields
}

export function mapFieldIds(
  rawFields: Record<string, unknown>,
  deviceFields: PlatformDeviceField[]
): Record<string, unknown> {
  if (deviceFields.length === 0) return rawFields
  const mapped: Record<string, unknown> = {}
  for (const field of deviceFields) {
    if (!field.id) continue
    const byId = rawFields[field.id]
    const byName = field.name !== undefined ? rawFields[field.name] : undefined
    if (byId !== undefined) mapped[field.id] = byId
    else if (byName !== undefined) mapped[field.id] = byName
  }
  return Object.keys(mapped).length > 0 ? mapped : rawFields
}

export function createThingsVisDeviceWsBridge(options: ThingsVisDeviceWsBridgeOptions) {
  const deviceWsMap = new Map<string, DeviceWsEntry>()
  const deviceStatusWsMap = new Map<string, DeviceStatusWsEntry>()

  function connectTelemetry(device: { deviceId: string; fields: PlatformDeviceField[] }) {
    const { deviceId } = device
    const prev = deviceWsMap.get(deviceId)
    if (prev) destroyDeviceWsEntry(prev)

    const entry: DeviceWsEntry = {
      ws: null,
      pingTimer: null,
      reconnectTimer: null,
      destroyed: false,
      device
    }
    deviceWsMap.set(deviceId, entry)

    runManagedDeviceWs(entry, {
      buildUrl: () => `${getWebsocketServerUrl()}/telemetry/datas/current/ws`,
      buildInitPayload: (token) => JSON.stringify({ device_id: deviceId, token }),
      onMessage: (event) => {
        const message = parseJsonDeviceWsMessage(event)
        if (!message) return

        const rawFields = extractWsFields(message)
        if (Object.keys(rawFields).length === 0) return

        const fields = mapFieldIds(rawFields, entry.device.fields)
        options.postPlatformData(fields, deviceId)
      },
      onInitError: (error) => {
        logger.warn('[DeviceWsBridge] WS init failed for device', deviceId, error)
      },
      onReconnect: () => {
        logger.warn('[DeviceWsBridge] WS closed for device', deviceId, '- scheduling reconnect')
      }
    })
  }

  function connectStatus(deviceId: string) {
    const prev = deviceStatusWsMap.get(deviceId)
    if (prev) destroyDeviceWsEntry(prev)

    const entry: DeviceStatusWsEntry = {
      ws: null,
      pingTimer: null,
      reconnectTimer: null,
      destroyed: false,
      deviceId
    }
    deviceStatusWsMap.set(deviceId, entry)

    runManagedDeviceWs(entry, {
      buildUrl: () => `${getWebsocketServerUrl()}/device/online/status/ws`,
      buildInitPayload: (token) => JSON.stringify({ device_id: deviceId, token }),
      onMessage: (event) => {
        const message = parseJsonDeviceWsMessage(event)
        if (!message || typeof message !== 'object') return

        const payload = message as Record<string, unknown>
        if (typeof payload.is_online !== 'number') return

        options.postPlatformData(
          {
            is_online: payload.is_online,
            online_text: payload.is_online === 1 ? '在线' : '离线',
            online_status_updated_at: Date.now()
          },
          deviceId
        )
      },
      onInitError: (error) => {
        logger.warn('[DeviceWsBridge] Status WS init failed for device', deviceId, error)
      }
    })
  }

  function ensureTelemetry(device?: { deviceId: string; fields: PlatformDeviceField[] }) {
    if (!device?.deviceId) return
    const existing = deviceWsMap.get(device.deviceId)
    if (existing && !existing.destroyed) return
    connectTelemetry(device)
  }

  function ensureStatus(deviceId?: string) {
    if (!deviceId) return
    const existing = deviceStatusWsMap.get(deviceId)
    if (existing && !existing.destroyed) return
    connectStatus(deviceId)
  }

  function updateDeviceFields(deviceId: string, fields: PlatformDeviceField[]) {
    const wsEntry = deviceWsMap.get(deviceId)
    if (wsEntry) wsEntry.device.fields = fields
  }

  function disconnectAll() {
    for (const entry of deviceWsMap.values()) {
      destroyDeviceWsEntry(entry)
    }
    deviceWsMap.clear()

    for (const entry of deviceStatusWsMap.values()) {
      destroyDeviceWsEntry(entry)
    }
    deviceStatusWsMap.clear()
  }

  return {
    connectTelemetry,
    connectStatus,
    ensureTelemetry,
    ensureStatus,
    updateDeviceFields,
    disconnectAll
  }
}
