/*
 * 文件用途：订阅平台告警实时推送（TB-30，/api/v1/alarm/status/ws），收到生命周期事件时回调。
 * 核心逻辑：首帧携带 token 认证，服务端推送 trigger/recovery/status 事件后触发回调；
 * 连接断开由统一实时客户端按指数退避重连。UI 回调应由调用方自行去抖，避免高频事件导致请求风暴。
 * 关键注意事项：组件卸载必须调用 stop()，否则 socket 泄漏。
 * 重构建议：协议帧解析可由调用方注入，本 hook 只保留通道生命周期。
 */
import { createRealtimeClient } from '@/service/realtime/realtime-socket'
import { getWebsocketServerUrl } from '@/utils/common/tool'

/** 构建告警状态 WebSocket 地址，与遥测 WS 共用同一 websocket 基地址。 */
function buildAlarmWsUrl(): string {
  return `${getWebsocketServerUrl()}/alarm/status/ws`
}

export function useAlarmStatusSocket(onEvent: () => void) {
  const client = createRealtimeClient({
    logTag: 'useAlarmStatusSocket',
    buildUrl: buildAlarmWsUrl,
    buildAuthFrame: (token) => JSON.stringify({ token }),
    onMessage: (data) => {
      if (typeof data !== 'string') return

      let parsed: { type?: string } | null = null
      try {
        parsed = JSON.parse(data)
      } catch {
        return
      }

      if (parsed?.type && parsed.type !== 'snapshot') {
        onEvent()
      }
    }
  })

  return { start: client.start, stop: client.stop }
}
