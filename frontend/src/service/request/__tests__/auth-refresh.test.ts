/**
 * 文件用途：验证 token 静默续签客户端的关键行为和回归边界。
 * 核心逻辑：通过 Vitest 模块 mock 构造并发/失败/阈值场景，断言网络调用次数、storage 写入和事件广播。
 * 关键注意事项：模块内含单例状态与节流时间戳，用例间必须 resetModules 隔离，避免互相污染。
 * 重构建议：可补充多标签页同步与退避策略用例（当前契约未包含）。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const BASE_NOW = 1_700_000_000_000

const hoisted = vi.hoisted(() => {
  return {
    httpGet: vi.fn(),
    axiosCreate: vi.fn(),
    localStg: {
      get: vi.fn(),
      set: vi.fn(),
      remove: vi.fn()
    },
    loggerWarn: vi.fn()
  }
})

vi.mock('axios', () => ({
  default: {
    create: hoisted.axiosCreate
  }
}))

vi.mock('@/utils/storage', () => ({
  localStg: hoisted.localStg
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: hoisted.loggerWarn,
    error: vi.fn()
  })
}))

vi.mock('~/env.config', () => ({
  createProxyPattern: vi.fn(() => '/proxy-api'),
  createServiceConfig: vi.fn(() => ({
    otherBaseURL: {
      platform: 'https://platform.example/api'
    }
  }))
}))

type AuthRefreshModule = typeof import('../auth-refresh')

/** 每个用例重新加载被测模块，隔离 inflight 单例与节流时间戳等模块级状态 */
async function loadAuthRefreshModule(): Promise<AuthRefreshModule> {
  vi.resetModules()
  return import('../auth-refresh')
}

function makeSuccessResponse(token: string, expiresIn: number) {
  return {
    status: 200,
    data: { code: 200, message: 'ok', data: { token, expires_in: expiresIn } }
  }
}

describe('service/request/auth-refresh.ts', () => {
  let dispatchEventSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(BASE_NOW)

    hoisted.axiosCreate.mockReset()
    hoisted.axiosCreate.mockReturnValue({ get: hoisted.httpGet })
    hoisted.httpGet.mockReset()
    hoisted.localStg.get.mockReset()
    hoisted.localStg.set.mockReset()
    hoisted.localStg.remove.mockReset()
    hoisted.loggerWarn.mockClear()

    // 默认已登录态：token 存在，session 剩余 90 分钟
    hoisted.localStg.get.mockImplementation((key: string) => {
      if (key === 'token') return 'old-token'
      if (key === 'lang') return null
      if (key === 'token_expires_in') return String(BASE_NOW + 90 * 60 * 1000)
      return null
    })

    dispatchEventSpy = vi.spyOn(window, 'dispatchEvent')
  })

  afterEach(() => {
    dispatchEventSpy.mockRestore()
    vi.useRealTimers()
  })

  /** 稳定清空微任务队列，等待刷新链路完成 */
  async function flushMicrotasks() {
    for (let i = 0; i < 10; i += 1) {
      await Promise.resolve()
    }
  }

  describe('refreshAuthToken', () => {
    it('merges concurrent calls into a single network request and persists the refreshed token', async () => {
      const mod = await loadAuthRefreshModule()
      hoisted.httpGet.mockResolvedValue(makeSuccessResponse('new-token', 3600))

      const results = await Promise.all([mod.refreshAuthToken(), mod.refreshAuthToken(), mod.refreshAuthToken()])

      expect(results).toEqual([true, true, true])
      // 并发合并：只发起一次网络请求
      expect(hoisted.httpGet).toHaveBeenCalledTimes(1)
      expect(hoisted.httpGet.mock.calls[0][0]).toBe('/user/refresh')
      expect(hoisted.httpGet.mock.calls[0][1]).toMatchObject({ headers: { 'x-token': 'old-token' } })

      // 与登录流程一致的存储口径：token_expires_in 存绝对过期时间戳(ms)
      expect(hoisted.localStg.set).toHaveBeenCalledWith('token', 'new-token')
      expect(hoisted.localStg.set).toHaveBeenCalledWith('token_expires_in', String(BASE_NOW + 3600 * 1000))

      // 成功后广播一次窗口事件
      expect(dispatchEventSpy).toHaveBeenCalledTimes(1)
      const dispatchedEvent = dispatchEventSpy.mock.calls[0][0] as CustomEvent
      expect(dispatchedEvent.type).toBe(mod.AUTH_TOKEN_REFRESHED_EVENT)
      expect(mod.AUTH_TOKEN_REFRESHED_EVENT).toBe('aetherlink:token-refreshed')
    })

    it('returns false on network failure without recursing and keeps storage untouched', async () => {
      const mod = await loadAuthRefreshModule()
      hoisted.httpGet.mockRejectedValue(new Error('network down'))

      const result = await mod.refreshAuthToken()

      expect(result).toBe(false)
      // 失败不递归：单次调用链只发一次请求
      expect(hoisted.httpGet).toHaveBeenCalledTimes(1)
      expect(hoisted.localStg.set).not.toHaveBeenCalled()
      expect(dispatchEventSpy).not.toHaveBeenCalled()
    })

    it('returns false when concurrent calls all fail and shares the same rejected attempt', async () => {
      const mod = await loadAuthRefreshModule()
      hoisted.httpGet.mockRejectedValue(new Error('boom'))

      const results = await Promise.all([mod.refreshAuthToken(), mod.refreshAuthToken(), mod.refreshAuthToken()])

      expect(results).toEqual([false, false, false])
      expect(hoisted.httpGet).toHaveBeenCalledTimes(1)
    })

    it('returns false when backend answers HTTP 200 with a rejected business code', async () => {
      const mod = await loadAuthRefreshModule()
      hoisted.httpGet.mockResolvedValue({
        status: 200,
        data: { code: 40101, message: 'invalid token' }
      })

      const result = await mod.refreshAuthToken()

      expect(result).toBe(false)
      expect(hoisted.httpGet).toHaveBeenCalledTimes(1)
      expect(hoisted.localStg.set).not.toHaveBeenCalled()
      expect(dispatchEventSpy).not.toHaveBeenCalled()
    })

    it('returns false when the success payload misses the token field', async () => {
      const mod = await loadAuthRefreshModule()
      hoisted.httpGet.mockResolvedValue({
        status: 200,
        data: { code: 200, data: { expires_in: 3600 } }
      })

      const result = await mod.refreshAuthToken()

      expect(result).toBe(false)
      expect(hoisted.localStg.set).not.toHaveBeenCalled()
      expect(dispatchEventSpy).not.toHaveBeenCalled()
    })

    it('is a no-op returning false when no token exists in storage', async () => {
      const mod = await loadAuthRefreshModule()
      hoisted.localStg.get.mockImplementation(() => null)

      const result = await mod.refreshAuthToken()

      expect(result).toBe(false)
      expect(hoisted.httpGet).not.toHaveBeenCalled()
    })
  })

  describe('scheduleProactiveTokenRefresh', () => {
    it('triggers a silent refresh when remaining lifetime is below the 45-minute threshold', async () => {
      const mod = await loadAuthRefreshModule()
      hoisted.httpGet.mockResolvedValue(makeSuccessResponse('proactive-token', 3600))
      // 剩余 10 分钟（<45 分钟）
      hoisted.localStg.get.mockImplementation((key: string) => {
        if (key === 'token') return 'old-token'
        if (key === 'token_expires_in') return String(BASE_NOW + 10 * 60 * 1000)
        return null
      })

      mod.scheduleProactiveTokenRefresh()
      await flushMicrotasks()

      expect(hoisted.httpGet).toHaveBeenCalledTimes(1)
      expect(hoisted.localStg.set).toHaveBeenCalledWith('token', 'proactive-token')
    })

    it('does nothing while the capped remaining lifetime stays above the threshold', async () => {
      const mod = await loadAuthRefreshModule()
      // session 剩余 50h；按 min(session, 24h) 封顶后仍远大于 45 分钟，不应触发
      hoisted.localStg.get.mockImplementation((key: string) => {
        if (key === 'token') return 'old-token'
        if (key === 'token_expires_in') return String(BASE_NOW + 50 * 60 * 60 * 1000)
        return null
      })

      mod.scheduleProactiveTokenRefresh()
      await flushMicrotasks()

      expect(hoisted.httpGet).not.toHaveBeenCalled()
    })

    it('throttles repeated checks within the check interval', async () => {
      const mod = await loadAuthRefreshModule()
      hoisted.httpGet.mockResolvedValue(makeSuccessResponse('token-2', 600))
      const setRemainingMinutes = (minutes: number) => {
        hoisted.localStg.get.mockImplementation((key: string) => {
          if (key === 'token') return 'old-token'
          if (key === 'token_expires_in') return String(Date.now() + minutes * 60 * 1000)
          return null
        })
      }

      // 剩余 10 分钟（<45 分钟）：首次调度触发静默刷新
      setRemainingMinutes(10)
      mod.scheduleProactiveTokenRefresh()
      await flushMicrotasks()
      expect(hoisted.httpGet).toHaveBeenCalledTimes(1)

      // 第一次刷新已完成，但仍在检查节流间隔内：第二次调度不应再触发
      setRemainingMinutes(10)
      mod.scheduleProactiveTokenRefresh()
      await flushMicrotasks()

      expect(hoisted.httpGet).toHaveBeenCalledTimes(1)

      // 推进超过节流间隔后恢复检查能力
      vi.setSystemTime(BASE_NOW + 6 * 60 * 1000)
      setRemainingMinutes(4)
      mod.scheduleProactiveTokenRefresh()
      await flushMicrotasks()

      expect(hoisted.httpGet).toHaveBeenCalledTimes(2)
    })

    it('does nothing when the user is logged out', async () => {
      const mod = await loadAuthRefreshModule()
      hoisted.localStg.get.mockImplementation(() => null)

      mod.scheduleProactiveTokenRefresh()
      await flushMicrotasks()

      expect(hoisted.httpGet).not.toHaveBeenCalled()
    })
  })
})
