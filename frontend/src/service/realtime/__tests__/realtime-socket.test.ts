/**
 * 文件用途：验证统一实时通道（WebSocket）生命周期实现的退避与状态机。
 * 核心逻辑：无 token 时走 onUnavailable 降级；建连后发鉴权帧与心跳；断线按退避重连；
 *   stop 后不再重连；token 续签事件触发重建；'pong' 心跳响应不进业务回调。
 * 关键注意事项：全局 WebSocket 被桩替换，计时使用 fake timers，用例内必须 stop 清理。
 * 重构建议：后续接 MQTT/SSE 传输适配时，把本文件的桩抽成共享测试工具。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AUTH_TOKEN_REFRESHED_EVENT } from '@/service/request/auth-refresh'
import { createRealtimeClient, nextBackoffDelay } from '../realtime-socket'

const hoisted = vi.hoisted(() => ({
  localStg: {
    get: vi.fn(),
    set: vi.fn(),
    remove: vi.fn()
  }
}))

vi.mock('@/utils/storage', () => ({
  localStg: hoisted.localStg
}))

/** 极简 WebSocket 桩：手动驱动 open/message/close，记录 send 的帧。 */
class FakeWebSocket {
  static instances: FakeWebSocket[] = []
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  url: string
  readyState = FakeWebSocket.CONNECTING
  sent: string[] = []
  closed = false
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: ((event: unknown) => void) | null = null
  onclose: ((event: unknown) => void) | null = null

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  send(data: string) {
    this.sent.push(data)
  }

  close() {
    this.closed = true
    this.readyState = FakeWebSocket.CLOSED
  }

  /** 测试驱动：连接建立 */
  simulateOpen() {
    this.readyState = FakeWebSocket.OPEN
    this.onopen?.()
  }

  /** 测试驱动：收到服务端帧 */
  simulateMessage(data: string) {
    this.onmessage?.({ data })
  }

  /** 测试驱动：连接异常断开 */
  simulateClose() {
    this.readyState = FakeWebSocket.CLOSED
    this.onclose?.({ code: 1006 })
  }
}

function latestSocket(): FakeWebSocket {
  return FakeWebSocket.instances[FakeWebSocket.instances.length - 1]
}

beforeEach(() => {
  FakeWebSocket.instances = []
  vi.stubGlobal('WebSocket', FakeWebSocket as unknown as typeof WebSocket)
  vi.useFakeTimers()
  hoisted.localStg.get.mockReturnValue('token-1')
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('nextBackoffDelay', () => {
  it('关闭抖动时按指数增长并被上限截断', () => {
    const opts = { baseDelayMs: 1_000, maxDelayMs: 30_000, factor: 2, jitter: false }
    expect(nextBackoffDelay(1, opts)).toBe(1_000)
    expect(nextBackoffDelay(2, opts)).toBe(2_000)
    expect(nextBackoffDelay(3, opts)).toBe(4_000)
    expect(nextBackoffDelay(6, opts)).toBe(30_000)
    expect(nextBackoffDelay(20, opts)).toBe(30_000)
  })

  it('开启抖动时延迟落在 [cap/2, cap] 区间', () => {
    const opts = { baseDelayMs: 1_000, maxDelayMs: 30_000, factor: 2, jitter: true }
    for (let attempt = 1; attempt <= 6; attempt += 1) {
      const delay = nextBackoffDelay(attempt, opts)
      const cap = Math.min(1_000 * 2 ** (attempt - 1), 30_000)
      expect(delay).toBeGreaterThanOrEqual(cap / 2)
      expect(delay).toBeLessThanOrEqual(cap)
    }
  })
})

describe('createRealtimeClient', () => {
  function createClient(overrides: Partial<Parameters<typeof createRealtimeClient>[0]> = {}) {
    return createRealtimeClient({
      logTag: 'test-channel',
      buildUrl: () => 'ws://example.test/channel',
      ...overrides
    })
  }

  it('无 token 时触发 onUnavailable 降级且不建连', () => {
    hoisted.localStg.get.mockReturnValue(undefined)
    const onUnavailable = vi.fn()
    const client = createClient({ onUnavailable, backoff: { maxAttempts: 0 } })

    client.start()

    expect(FakeWebSocket.instances).toHaveLength(0)
    expect(onUnavailable).toHaveBeenCalledTimes(1)
    expect(client.getStatus()).toBe('CLOSED')
  })

  it('建连后发送鉴权帧与心跳，业务帧进入 onMessage 而 pong 不进入', () => {
    const onOpen = vi.fn()
    const onMessage = vi.fn()
    const client = createClient({
      buildAuthFrame: (token) => `auth:${token}`,
      onOpen,
      onMessage,
      ping: { intervalMs: 1_000, message: 'ping' }
    })

    client.start()
    const socket = latestSocket()
    socket.simulateOpen()

    expect(onOpen).toHaveBeenCalledTimes(1)
    expect(socket.sent).toContain('auth:token-1')

    vi.advanceTimersByTime(1_000)
    expect(socket.sent).toContain('ping')

    socket.simulateMessage('{"kind":"push"}')
    socket.simulateMessage('pong')
    expect(onMessage).toHaveBeenCalledTimes(1)
    expect(onMessage).toHaveBeenCalledWith('{"kind":"push"}', expect.anything())

    client.stop()
  })

  it('send 仅在连接 OPEN 时可用', () => {
    const client = createClient()
    expect(client.send('early')).toBe(false)

    client.start()
    latestSocket().simulateOpen()
    expect(client.send('hello')).toBe(true)

    client.stop()
  })

  it('异常断开后按退避重连，连接成功后重试计数清零', () => {
    const client = createClient({ backoff: { baseDelayMs: 1_000, jitter: false } })

    client.start()
    expect(FakeWebSocket.instances).toHaveLength(1)

    latestSocket().simulateClose()
    vi.advanceTimersByTime(1_000)
    expect(FakeWebSocket.instances).toHaveLength(2)

    // 第二条连上后计数清零：再次断线仍从 baseDelay 起步
    latestSocket().simulateOpen()
    latestSocket().simulateClose()
    vi.advanceTimersByTime(1_000)
    expect(FakeWebSocket.instances).toHaveLength(3)

    client.stop()
  })

  it('stop() 后断线不再重连，避免组件卸载后的 socket 泄漏', () => {
    const client = createClient({ backoff: { baseDelayMs: 1_000, jitter: false } })

    client.start()
    client.stop()
    latestSocket().simulateClose()
    vi.advanceTimersByTime(60_000)

    expect(FakeWebSocket.instances).toHaveLength(1)
    expect(client.isDestroyed()).toBe(true)
  })

  it('达到 maxAttempts 后放弃重连', () => {
    const client = createClient({ backoff: { baseDelayMs: 1_000, jitter: false, maxAttempts: 2 } })

    client.start()
    // 第 1 次重连延迟 = baseDelay * 2^0 = 1000
    latestSocket().simulateClose()
    vi.advanceTimersByTime(1_000)
    // 第 2 次重连延迟 = baseDelay * 2^1 = 2000
    latestSocket().simulateClose()
    vi.advanceTimersByTime(2_000)
    // 第 2 次重连已用完：这次断开后不再产生新连接
    latestSocket().simulateClose()
    vi.advanceTimersByTime(60_000)

    expect(FakeWebSocket.instances).toHaveLength(3)
    expect(client.getStatus()).toBe('CLOSED')
  })

  it('token 静默续签事件触发连接重建', () => {
    const client = createClient({ reconnectOnTokenRefresh: true })

    client.start()
    latestSocket().simulateOpen()
    expect(FakeWebSocket.instances).toHaveLength(1)

    window.dispatchEvent(new CustomEvent(AUTH_TOKEN_REFRESHED_EVENT))
    // 重建：旧连接被关闭，新连接立即建立
    const sockets = FakeWebSocket.instances
    expect(sockets).toHaveLength(2)
    expect(sockets[0].onclose).toBeNull()
    expect(sockets[0].closed).toBe(true)
    expect(client.getStatus()).toBe('CONNECTING')

    client.stop()
  })
})
