/*
 * 文件用途：全站唯一的实时通道（WebSocket）生命周期实现。
 * 核心逻辑：统一负责建连、首帧鉴权、心跳、指数退避重连、token 续签重连与销毁清理。
 * 关键注意事项：
 *   - 之前 useAlarmStatusSocket / useRealtimePush / useAlarmPush / DeviceStatusWebSocket
 *     各写了一份"3 秒定时重连"，重连风暴与日志口径都不一致；现在只保留本文件的退避实现。
 *   - 重连退避带抖动（jitter），避免多标签页/多组件同时重连打爆网关。
 *   - 销毁后（stop）不再触发任何重连，杜绝组件卸载后的 socket 泄漏。
 *   - 无 token 时不会静默死等，而是通过 onUnavailable 通知调用方降级（如转轮询）。
 * 重构建议：新增 MQTT/SSE 通道时，复用本文件的退避与状态机，只替换底层传输适配。
 */
import { AUTH_TOKEN_REFRESHED_EVENT } from '@/service/request/auth-refresh'
import { localStg } from '@/utils/storage'

export interface BackoffOptions {
  /** 首次重连的基础延迟 */
  baseDelayMs?: number
  /** 退避上限，避免无限增长 */
  maxDelayMs?: number
  /** 每次失败的倍数 */
  factor?: number
  /** 是否叠加随机抖动 */
  jitter?: boolean
  /** 最大重试次数；小于 0 表示无限重连 */
  maxAttempts?: number
}

export const DEFAULT_BACKOFF: Required<BackoffOptions> = {
  baseDelayMs: 1_000,
  maxDelayMs: 30_000,
  factor: 2,
  jitter: true,
  maxAttempts: -1
}

/**
 * 计算第 attempt 次重连的延迟（attempt 从 1 开始）。
 */
export function nextBackoffDelay(attempt: number, options: BackoffOptions = {}): number {
  const { baseDelayMs, maxDelayMs, factor, jitter } = { ...DEFAULT_BACKOFF, ...options }
  const exponential = baseDelayMs * factor ** Math.max(0, attempt - 1)
  const capped = Math.min(exponential, maxDelayMs)

  if (!jitter) return Math.round(capped)

  // 半抖动：保留一半固定延迟 + 一半随机，兼顾可预期与去同步。
  return Math.round(capped / 2 + Math.random() * (capped / 2))
}

/** 心跳配置；设为 null 关闭心跳。 */
export interface PingOptions {
  intervalMs: number
  /** 心跳帧内容，默认 'ping' */
  message?: string
}

export type RealtimeStatus = 'IDLE' | 'CONNECTING' | 'OPEN' | 'CLOSED'

export interface RealtimeClientOptions {
  /** 日志前缀，便于定位是哪个通道 */
  logTag: string
  /** 连接地址构造器（每次重连都会重新求值，便于 URL 随环境变化） */
  buildUrl: () => string
  /** 建连后发送的首帧；多数通道用它携带 token / device_id 鉴权 */
  buildAuthFrame?: (token: string) => string
  /** 业务消息回调；已剔除心跳响应（'pong'） */
  onMessage?: (payload: unknown, event: MessageEvent) => void
  onOpen?: () => void
  onClose?: (event: CloseEvent) => void
  onError?: (event: Event) => void
  /** 无 token / 通道暂不可用时的降级通知（例如转轮询） */
  onUnavailable?: () => void
  /** 心跳配置，默认 8s 一次 'ping' */
  ping?: PingOptions | null
  /** 重连退避配置 */
  backoff?: BackoffOptions
  /** token 静默续签后是否重建连接（订阅类通道需要，推送类通道可关） */
  reconnectOnTokenRefresh?: boolean
}

export interface RealtimeClient {
  start: () => void
  stop: () => void
  /** 手动触发一次重连（保留当前重试计数） */
  reconnect: () => void
  /** 向已建立的连接发送文本帧；未连接时返回 false */
  send: (data: string) => boolean
  isDestroyed: () => boolean
  getStatus: () => RealtimeStatus
}

const DEFAULT_PING: PingOptions = { intervalMs: 8_000, message: 'ping' }

function getAuthToken(): string | undefined {
  return localStg.get('token') as string | undefined
}

/**
 * 创建一个带统一重连/退避的 WebSocket 客户端。
 */
export function createRealtimeClient(options: RealtimeClientOptions): RealtimeClient {
  const {
    logTag,
    buildUrl,
    buildAuthFrame,
    onMessage,
    onOpen,
    onClose,
    onError,
    onUnavailable,
    ping = DEFAULT_PING,
    backoff,
    reconnectOnTokenRefresh = false
  } = options

  const maxAttempts = backoff?.maxAttempts ?? DEFAULT_BACKOFF.maxAttempts

  let socket: WebSocket | null = null
  let pingTimer: ReturnType<typeof setInterval> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let attempt = 0
  let destroyed = false
  let status: RealtimeStatus = 'IDLE'

  const clearPingTimer = () => {
    if (!pingTimer) return
    clearInterval(pingTimer)
    pingTimer = null
  }

  const clearReconnectTimer = () => {
    if (!reconnectTimer) return
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }

  const closeSocket = () => {
    if (!socket) return
    // 解绑 onclose 后再关，避免"主动关闭"被当成"异常断开"而触发重连。
    socket.onclose = null
    socket.onmessage = null
    socket.onerror = null
    socket.onopen = null
    socket.close()
    socket = null
  }

  const scheduleReconnect = () => {
    if (destroyed || reconnectTimer) return

    attempt += 1
    if (maxAttempts >= 0 && attempt > maxAttempts) {
      console.warn(`[${logTag}] Realtime channel gave up after ${maxAttempts} reconnect attempts`)
      status = 'CLOSED'
      return
    }

    const delay = nextBackoffDelay(attempt, backoff)
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, delay)
  }

  const startPing = () => {
    if (!ping) return
    clearPingTimer()
    pingTimer = setInterval(() => {
      if (socket?.readyState === WebSocket.OPEN) {
        socket.send(ping.message ?? 'ping')
      }
    }, ping.intervalMs)
  }

  const connect = () => {
    if (destroyed) return

    const token = getAuthToken()
    if (!token) {
      onUnavailable?.()
      scheduleReconnect()
      return
    }

    clearReconnectTimer()
    closeSocket()
    status = 'CONNECTING'

    try {
      socket = new WebSocket(buildUrl())
    } catch (error) {
      console.warn(`[${logTag}] Failed to open realtime channel, retrying:`, error)
      status = 'CLOSED'
      scheduleReconnect()
      return
    }

    socket.onopen = () => {
      // 连接成功即清零退避计数，下次断线从 baseDelay 重新起步。
      attempt = 0
      status = 'OPEN'
      if (buildAuthFrame) {
        socket?.send(buildAuthFrame(token))
      }
      startPing()
      onOpen?.()
    }

    socket.onmessage = (event: MessageEvent) => {
      if (typeof event.data === 'string' && event.data === 'pong') return
      onMessage?.(event.data, event)
    }

    socket.onerror = (event: Event) => {
      onError?.(event)
      console.warn(`[${logTag}] Realtime channel error`, event)
    }

    socket.onclose = (event: CloseEvent) => {
      clearPingTimer()
      socket = null
      status = 'CLOSED'
      onClose?.(event)
      if (destroyed) return
      scheduleReconnect()
    }
  }

  const handleTokenRefreshed = () => {
    if (destroyed) return
    // 订阅帧里的 token 已失效，用最新 token 重建一次连接。
    attempt = 0
    clearReconnectTimer()
    closeSocket()
    connect()
  }

  return {
    start: () => {
      destroyed = false
      attempt = 0
      clearReconnectTimer()
      if (reconnectOnTokenRefresh) {
        window.addEventListener(AUTH_TOKEN_REFRESHED_EVENT, handleTokenRefreshed)
      }
      connect()
    },
    stop: () => {
      destroyed = true
      clearPingTimer()
      clearReconnectTimer()
      closeSocket()
      status = 'IDLE'
      if (reconnectOnTokenRefresh) {
        window.removeEventListener(AUTH_TOKEN_REFRESHED_EVENT, handleTokenRefreshed)
      }
    },
    reconnect: () => {
      if (destroyed) return
      clearReconnectTimer()
      connect()
    },
    send: (data: string) => {
      if (socket?.readyState !== WebSocket.OPEN) return false
      socket.send(data)
      return true
    },
    isDestroyed: () => destroyed,
    getStatus: () => status
  }
}
