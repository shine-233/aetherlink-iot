/*
 * 文件用途：订阅平台告警实时推送（TB-30），并把平台字段映射为 ThingsVis 可展示数据。
 * 核心逻辑：优先走 /api/v1/alarm/status/ws WebSocket 订阅（首帧携带 token，服务端推
 * snapshot/trigger/recovery/status 事件）；连接建立前先用一次 REST 拉取当前告警状态，
 * WS 不可用时降级为 30s 轮询，恢复订阅后停止轮询。重连退避复用统一实时客户端。
 * 关键注意事项：需要关注连接清理、字段映射和异常消息过滤，避免跨设备污染。
 * 重构建议：后续可抽出消息解析器并补充 malformed payload 测试。 */
import { type Ref } from 'vue'
import { deviceAlarmStatus } from '@/service/api/device'
import { createRealtimeClient } from '@/service/realtime/realtime-socket'
import { createVisibleInterval } from '@/hooks/common/useVisibleInterval'
import { getWebsocketServerUrl } from '@/utils/common/tool'
import type { PlatformField } from '@/utils/thingsvis/types'

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

  // 降级轮询只在页面可见时运行：后台标签页不再每 30s 打后端，恢复可见时立即补拉一次。
  const poller = createVisibleInterval(() => {
    void fetchAlarmStatus()
  }, POLL_INTERVAL_MS)
  const startPolling = () => poller.start()
  const stopPolling = () => poller.stop()

  const client = createRealtimeClient({
    logTag: 'useAlarmPush',
    buildUrl: buildAlarmWsUrl,
    buildAuthFrame: (token) => JSON.stringify({ token }),
    onOpen: () => {
      // WS 可用即停止降级轮询，避免双通道重复推送。
      stopPolling()
    },
    // 无 token 或连接暂不可用时降级轮询，保证告警可见性不回退。
    onUnavailable: startPolling,
    onClose: startPolling,
    onMessage: handleAlarmMessage
  })

  const start = () => {
    if (!eventFields().length) return
    // 订阅建立前先拉一次当前状态，避免快照字段映射不到时界面空白。
    void fetchAlarmStatus()
    client.start()
  }

  const stop = () => {
    stopPolling()
    client.stop()
  }

  return { start, stop }
}
