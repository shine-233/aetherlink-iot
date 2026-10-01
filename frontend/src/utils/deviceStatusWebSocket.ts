/*
 * 文件用途：封装设备状态 WebSocket 客户端，用于订阅设备在线状态变化。
 * 核心逻辑：管理连接、订阅参数、消息解析和响应式状态更新；重连与退避统一由
 *   createRealtimeClient 负责（此前这里单独用 vueuse 的 autoReconnect，与其余
 *   三个实时通道的重连策略不一致）。
 * 关键注意事项：订阅帧携带 token，静默续签后必须重建连接；取消订阅要真正断连。
 * 重构建议：可抽出协议消息解析并补充断线/重复订阅测试。
 */
import type { RealtimeClient } from '@/service/realtime/realtime-socket'
import { createRealtimeClient } from '@/service/realtime/realtime-socket'
import { getWebsocketServerUrl } from '@/utils/common/tool'
import { localStg } from '@/utils/storage'

interface DeviceStatusMessage {
  device_id: string
  is_online: number // 1: 在线, 0: 离线
}

interface SubscriptionParams {
  device_ids: string[]
  token: string
}

/** 与后端心跳窗口对齐：30s 一次 ping。 */
const PING_INTERVAL_MS = 30_000
/** 保留原实现的重连上限：连续失败 5 次后放弃，避免后台无限重连。 */
const MAX_RECONNECT_ATTEMPTS = 5

/**
 * 设备状态 WebSocket 管理器
 * 用于订阅和接收设备在线/离线状态通知
 */
export class DeviceStatusWebSocket {
  private client: RealtimeClient
  private currentDeviceIds: string[] = []
  private onStatusChangeCallback?: (deviceId: string, isOnline: boolean) => void

  constructor() {
    this.client = createRealtimeClient({
      logTag: 'DeviceStatusWebSocket',
      buildUrl: () => `${getWebsocketServerUrl()}/device/online/status/ws/batch`,
      ping: { intervalMs: PING_INTERVAL_MS, message: 'ping' },
      backoff: { maxAttempts: MAX_RECONNECT_ATTEMPTS },
      // 订阅帧里的 token 会在静默续签后失效，续签成功即重建连接。
      reconnectOnTokenRefresh: true,
      onOpen: () => {
        this.sendSubscription()
      },
      onMessage: (data) => {
        this.handleMessage(data)
      }
    })
  }

  /**
   * 连接 WebSocket 并订阅设备状态
   * @param deviceIds 设备ID列表
   * @param onStatusChange 状态变化回调函数
   */
  connect(deviceIds: string[], onStatusChange?: (deviceId: string, isOnline: boolean) => void) {
    if (onStatusChange) {
      this.onStatusChangeCallback = onStatusChange
    }

    // 如果设备列表为空，不建立连接
    if (!deviceIds || deviceIds.length === 0) {
      this.disconnect()
      return
    }

    // 已连接且订阅集合未变化：无需重建连接（分页来回切换时避免抖动）
    if (this.client.getStatus() === 'OPEN' && this.arraysEqual(this.currentDeviceIds, deviceIds)) {
      return
    }

    this.currentDeviceIds = [...deviceIds]
    this.client.start()
  }

  /**
   * 更新订阅的设备列表
   * @param deviceIds 新的设备ID列表
   */
  updateSubscription(deviceIds: string[], onStatusChange?: (deviceId: string, isOnline: boolean) => void) {
    if (onStatusChange) {
      this.onStatusChangeCallback = onStatusChange
    }

    if (!deviceIds || deviceIds.length === 0) {
      this.disconnect()
      return
    }

    // 已连接时直接改订阅帧，不重建连接
    if (this.client.getStatus() === 'OPEN') {
      this.currentDeviceIds = [...deviceIds]
      this.sendSubscription()
      return
    }

    this.connect(deviceIds, this.onStatusChangeCallback)
  }

  /**
   * 断开 WebSocket 连接
   */
  disconnect() {
    this.client.stop()
    this.currentDeviceIds = []
  }

  /**
   * 移除 token 刷新监听并断开连接（可选清理入口）
   */
  destroy() {
    this.disconnect()
  }

  /**
   * 获取当前连接状态
   */
  getStatus(): string {
    return this.client.getStatus()
  }

  /** 发送订阅帧；连接未就绪时静默跳过，等 onOpen 再补发。 */
  private sendSubscription() {
    const token = this.readToken()
    if (!token || this.currentDeviceIds.length === 0) return

    const subscriptionMessage: SubscriptionParams = {
      device_ids: this.currentDeviceIds,
      token
    }

    this.client.send(JSON.stringify(subscriptionMessage))
  }

  private readToken(): string | undefined {
    // 延迟读取，确保每次发帧都拿到最新的（续签后的）token。
    return localStg.get('token') ?? undefined
  }

  private handleMessage(data: unknown) {
    let payload: unknown
    try {
      payload = typeof data === 'string' ? JSON.parse(data) : data
    } catch {
      // 解析失败静默忽略
      return
    }

    const emit = (deviceId: string, isOnline: number) => {
      this.onStatusChangeCallback?.(deviceId, isOnline === 1)
    }

    // 支持批量消息格式（数组）
    if (Array.isArray(payload)) {
      payload.forEach((item: any) => {
        if (item?.device_id && typeof item.is_online === 'number') {
          emit(item.device_id, item.is_online)
        }
      })
      return
    }

    // 支持单条消息格式（对象）
    if (payload && typeof payload === 'object') {
      const item = payload as DeviceStatusMessage
      if (item.device_id && typeof item.is_online === 'number') {
        emit(item.device_id, item.is_online)
      }
    }
    // 其他格式静默忽略，不报错
  }

  /**
   * 比较两个数组是否相等
   */
  private arraysEqual(arr1: string[], arr2: string[]): boolean {
    if (arr1.length !== arr2.length) return false
    const sorted1 = [...arr1].sort()
    const sorted2 = [...arr2].sort()
    return sorted1.every((val, index) => val === sorted2[index])
  }
}

/**
 * 创建设备状态 WebSocket 实例的组合式函数
 * @returns WebSocket 管理器实例
 */
export function useDeviceStatusWebSocket() {
  const wsManager = new DeviceStatusWebSocket()

  return {
    connect: wsManager.connect.bind(wsManager),
    updateSubscription: wsManager.updateSubscription.bind(wsManager),
    disconnect: wsManager.disconnect.bind(wsManager),
    destroy: wsManager.destroy.bind(wsManager),
    getStatus: wsManager.getStatus.bind(wsManager)
  }
}
