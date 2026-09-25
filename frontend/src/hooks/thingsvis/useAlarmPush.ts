/*
 * 文件用途：订阅平台告警实时推送（TB-30），并把平台字段映射为 ThingsVis 可展示数据。
 * 核心逻辑：优先走 /api/v1/alarm/status/ws WebSocket 订阅（首帧携带 token，服务端推
 * snapshot/trigger/recovery/status 事件）；连接建立前先用一次 REST 拉取当前告警状态，
 * WS 不可用时自动降级为 30s 轮询，恢复订阅后停止轮询。
 * 关键注意事项：需要关注连接清理、字段映射和异常消息过滤，避免跨设备污染。
 * 重构建议：后续可抽出消息解析器并补充 malformed payload 测试。
 */
import { type Ref } from 'vue'
import { deviceAlarmStatus } from '@/service/api/device'
import { localStg } from '@/utils/storage'
import { getWebsocketServerUrl } from '@/utils/common/tool'
import type { PlatformField } from '@/utils/thingsvis/types'

/** ping 间隔。服务端心跳窗口较短，需与 useRealtimePush 保持一致（8s）。 */
const PING_INTERVAL_MS = 8_000
const WS_RECONNECT_DELAY_MS = 3000
const POLL_INTERVAL_MS = 30_000

interface AlarmRealtimeEvent {
  type?: string
  name?: string
  level?: string
  content?: string
  timestamp?: number
  device_ids?: string[]
  items?: Array<{ name?: string; level?: string; content?: string; device_ids?: string[]; create_at?: number }>
}

/** 构建告警状态 WebSocket 地址，与遥测 WS 共用同一 websocket 基地址。 */
function buildAlarmWsUrl(): string {
  return `${getWebsocketServerUrl()}/alarm/status/ws`
}

export function useAlarmPush(
  deviceId: Ref<string>,
  platformFields: Ref<PlatformField[]>,
  pushData: (fields: Record<string, unknown>) => void
) {
  let alarmTimer: ReturnType<typeof setInterval> | null = null
  let socket: WebSocket | null = null
  let pingTimer: ReturnType<typeof setInterval> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let destroyed = false

  const eventFields = () => platformFields.value.filter((field) => field.dataType === 'event')

  const fetchAlarmStatus = async () => {
    const fields = eventFields()
    if (!fields.length || !deviceId.value) return

    try {
      const res = await deviceAlarmStatus({ device_id: deviceId.value })
      if (!Array.isArray(res?.data)) return

      const alarmFields: Record<string, unknown> = {}
      fields.forEach((field) => {
        const alarm = res.data.find((item: any) => item.alarm_name === field.id || item.key === field.id)
        if (!alarm) return

        alarmFields[field.id] = {
          active: alarm.is_active ?? false,
          level: alarm.alarm_level,
          message: alarm.alarm_description,
          time: alarm.last_trigger_time
        }
      })

      if (Object.keys(alarmFields).length > 0) {
        pushData(alarmFields)
      }
    } catch (error) {
      console.warn('[useAlarmPush] Failed to fetch alarm status:', error)
    }
  }

  /** 事件是否与当前设备相关：事件未挂设备（配置级）视为相关。 */
  const eventMatchesDevice = (deviceIds: unknown) => {
    if (!Array.isArray(deviceIds) || deviceIds.length === 0) return true
    if (!deviceId.value) return true
    return deviceIds.includes(deviceId.value)
  }

  const applyAlarmEvent = (name: string | undefined, active: boolean, level: unknown, message: unknown, time: unknown) => {
    if (!name) return
    const field = eventFields().find((item) => item.id === name)
    if (!field) return
    pushData({
      [field.id]: {
        active,
        level,
        message,
        time
      }
    })
  }

  const handleAlarmMessage = (raw: unknown) => {
    const event = (typeof raw === 'string' ? safeParse(raw) : raw) as AlarmRealtimeEvent | null
    if (!event || typeof event !== 'object') return

    if (event.type === 'snapshot' && Array.isArray(event.items)) {
      event.items.forEach((item) => {
        if (!item || !eventMatchesDevice(item.device_ids)) return
        applyAlarmEvent(item.name, true, item.level, item.content, item.create_at)
      })
      return
    }

    if (event.type === 'trigger' || event.type === 'recovery' || event.type === 'status') {
      if (!eventMatchesDevice(event.device_ids)) return
      applyAlarmEvent(event.name, event.type === 'trigger', event.level, event.content, event.timestamp)
    }
  }

  function safeParse(raw: string): unknown {
    try {
      return JSON.parse(raw)
    } catch {
      console.warn('[useAlarmPush] Malformed alarm websocket payload dropped')
      return null
    }
  }

  const startPolling = () => {
    if (alarmTimer) return
    alarmTimer = setInterval(() => {
      void fetchAlarmStatus()
    }, POLL_INTERVAL_MS)
  }

  const stopPolling = () => {
    if (!alarmTimer) return
    clearInterval(alarmTimer)
    alarmTimer = null
  }

  const clearPingTimer = () => {
    if (!pingTimer) return
    clearInterval(pingTimer)
    pingTimer = null
  }

  const scheduleReconnect = () => {
    if (destroyed || reconnectTimer) return
    // WS 不可用期间降级轮询，保证告警可见性不回退。
    startPolling()
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, WS_RECONNECT_DELAY_MS)
  }

  const connect = () => {
    if (destroyed) return
    const token = localStg.get('token') as string | undefined
    if (!token) {
      startPolling()
      return
    }

    try {
      socket = new WebSocket(buildAlarmWsUrl())
    } catch (error) {
      console.warn('[useAlarmPush] Failed to open alarm websocket:', error)
      scheduleReconnect()
      return
    }

    socket.onopen = () => {
      stopPolling()
      socket?.send(JSON.stringify({ token }))
      pingTimer = setInterval(() => {
        if (socket?.readyState === WebSocket.OPEN) {
          socket.send('ping')
        }
      }, PING_INTERVAL_MS)
    }

    socket.onmessage = (event: MessageEvent) => {
      handleAlarmMessage(event.data)
    }

    socket.onerror = (event) => {
      console.warn('[useAlarmPush] Alarm websocket error:', event)
    }

    socket.onclose = () => {
      clearPingTimer()
      socket = null
      scheduleReconnect()
    }
  }

  const start = () => {
    if (!eventFields().length) return
    // 订阅建立前先拉一次当前状态，避免快照字段映射不到时界面空白。
    void fetchAlarmStatus()
    connect()
  }

  const stop = () => {
    destroyed = true
    stopPolling()
    clearPingTimer()
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (socket) {
      socket.onclose = null
      socket.close()
      socket = null
    }
  }

  return { start, stop }
}
