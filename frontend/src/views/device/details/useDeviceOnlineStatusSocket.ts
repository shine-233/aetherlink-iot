/**
 * 文件用途: 设备详情页在线状态 WebSocket 订阅 composable。
 * 核心逻辑: 建立 /device/online/status/ws 连接（8s ping 心跳），每帧只更新当前设备的在线态与更新时间；
 * 详情加载成功后调用 subscribe(deviceId) 发送订阅报文。
 * 关键注意事项: 帧结构兼容逻辑在 device-online-status-frame.ts；非 JSON 文本帧与 pong 静默忽略。
 */
import { computed, ref } from 'vue'
import { useWebSocket } from '@vueuse/core'
import { localStg } from '@/utils/storage'
import { getWebsocketServerUrl } from '@/utils/common/tool'
import { formatDateTime } from '@/utils/common/datetime'
import { normalizeOnlineStatus, normalizeOnlineStatusUpdatedAt } from './device-online-status-frame'

export function useDeviceOnlineStatusSocket(getDeviceId: () => string) {
  const isOnline = ref(0)
  const updatedAt = ref('')
  const updatedAtDisplay = computed(() => formatDateTime(updatedAt.value) || '--')

  function applyFrame(frame: string) {
    if (!frame || frame === 'pong') return
    try {
      const payload = JSON.parse(frame)
      const status = normalizeOnlineStatus(payload, getDeviceId())
      if (status !== null) {
        isOnline.value = status
        updatedAt.value = normalizeOnlineStatusUpdatedAt(payload, getDeviceId()) || new Date().toISOString()
      }
    } catch {
      // 忽略心跳外的非 JSON 文本帧，避免无意义日志污染。
    }
  }

  const { send } = useWebSocket(`${getWebsocketServerUrl()}/device/online/status/ws`, {
    heartbeat: { message: 'ping', interval: 8000, pongTimeout: 3000 },
    onMessage(_ws: WebSocket, event: MessageEvent) {
      applyFrame(event.data)
    }
  })

  function subscribe(deviceId: string) {
    send(JSON.stringify({ device_id: deviceId, token: localStg.get('token') }))
  }

  function reset() {
    isOnline.value = 0
    updatedAt.value = ''
  }

  return { isOnline, updatedAt, updatedAtDisplay, applyFrame, subscribe, reset }
}

export type DeviceOnlineStatusSocket = ReturnType<typeof useDeviceOnlineStatusSocket>
