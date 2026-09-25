/*
 * 文件用途：订阅平台告警实时推送（TB-30，/api/v1/alarm/status/ws），收到生命周期事件时回调。
 * 核心逻辑：首帧携带 token 认证，服务端推送 trigger/recovery/status 事件后触发回调；
 * 连接断开自动重连。UI 回调应由调用方自行去抖，避免高频事件导致请求风暴。
 * 关键注意事项：组件卸载必须调用 stop()，否则 socket 泄漏。
 * 重构建议：与 useAlarmPush/useRealtimePush 存在相似的 socket 生命周期骨架，可后续抽公共模块。
 */
import { localStg } from '@/utils/storage'
import { getWebsocketServerUrl } from '@/utils/common/tool'

/** ping 间隔。服务端心跳窗口较短，需与 useRealtimePush 保持一致（8s）。 */
const PING_INTERVAL_MS = 8_000
const WS_RECONNECT_DELAY_MS = 3000

/** 构建告警状态 WebSocket 地址，与遥测 WS 共用同一 websocket 基地址。 */
function buildAlarmWsUrl(): string {
  return `${getWebsocketServerUrl()}/alarm/status/ws`
}

export function useAlarmStatusSocket(onEvent: () => void) {
  let socket: WebSocket | null = null
  let pingTimer: ReturnType<typeof setInterval> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let destroyed = false

  const clearPingTimer = () => {
    if (!pingTimer) return
    clearInterval(pingTimer)
    pingTimer = null
  }

  const scheduleReconnect = () => {
    if (destroyed || reconnectTimer) return
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, WS_RECONNECT_DELAY_MS)
  }

  const connect = () => {
    if (destroyed) return
    const token = localStg.get('token') as string | undefined
    if (!token) return

    try {
      socket = new WebSocket(buildAlarmWsUrl())
    } catch (error) {
      console.warn('[useAlarmStatusSocket] Failed to open alarm websocket:', error)
      scheduleReconnect()
      return
    }

    socket.onopen = () => {
      socket?.send(JSON.stringify({ token }))
      pingTimer = setInterval(() => {
        if (socket?.readyState === WebSocket.OPEN) {
          socket.send('ping')
        }
      }, PING_INTERVAL_MS)
    }

    socket.onmessage = (event: MessageEvent) => {
      let parsed: { type?: string } | null = null
      try {
        parsed = JSON.parse(String(event.data))
      } catch {
        return
      }
      if (parsed && parsed.type && parsed.type !== 'snapshot') {
        onEvent()
      }
    }

    socket.onerror = (event) => {
      console.warn('[useAlarmStatusSocket] Alarm websocket error:', event)
    }

    socket.onclose = () => {
      clearPingTimer()
      socket = null
      scheduleReconnect()
    }
  }

  const start = () => {
    connect()
  }

  const stop = () => {
    destroyed = true
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
