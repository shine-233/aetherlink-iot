/**
 * 文件用途: 平台 token 静默续签客户端，供请求层 401 重试与主动续签复用。
 * 核心逻辑: 通过独立裸 axios 实例调用 GET /user/refresh，单例合并并发刷新，
 *           成功后写回 localStorage 并广播窗口事件供 WebSocket 等模块重建连接。
 * 关键注意事项: 刷新必须走独立实例，避免经过全局 401 拦截器形成递归；
 *           expires_in 是 Redis session TTL（可达 100h），不能当作 JWT 寿命使用。
 * 重构建议: 若后续需要多标签页同步或退避策略，可将节流与广播逻辑参数化并补充竞态测试。
 */
import axios from 'axios'
import type { CreateAxiosDefaults } from 'axios'
import { createProxyPattern, createServiceConfig } from '~/env.config'
import { localStg } from '@/utils/storage'
import { createLogger } from '@/utils/logger'

const logger = createLogger('AuthRefresh')

/** token 刷新成功后派发的窗口自定义事件名，WebSocket 等模块监听后重建连接 */
export const AUTH_TOKEN_REFRESHED_EVENT = 'aetherlink:token-refreshed'

/** JWT exp 默认寿命上限（后端 24h）。expires_in 是 session TTL（可达 100h），寿命计算须以此封顶 */
const JWT_MAX_AGE_MS = 24 * 60 * 60 * 1000
/** token 剩余寿命低于该阈值时触发主动静默刷新 */
const PROACTIVE_REFRESH_THRESHOLD_MS = 45 * 60 * 1000
/** 主动刷新判断的节流间隔，避免每次请求都重复执行剩余寿命检查 */
const PROACTIVE_CHECK_INTERVAL_MS = 5 * 60 * 1000

const { otherBaseURL } = createServiceConfig(import.meta.env)
const isHttpProxy = import.meta.env.VITE_HTTP_PROXY === 'Y'
const platformApiBaseUrl = otherBaseURL.platform || `${window.location.origin}/api/v1`

/**
 * 刷新专用 axios 实例（与全局 request 实例同一 API 地址口径）。
 * 关键约束：绝不能复用全局 request 实例——它的 onError 会消费 401，
 * 复用会让“刷新失败的 401”再次进入刷新流程造成递归。
 */
const refreshHttpConfig: CreateAxiosDefaults = {
  baseURL: isHttpProxy ? createProxyPattern() : platformApiBaseUrl,
  timeout: 15_000
}
const refreshHttp = axios.create(refreshHttpConfig)

interface RefreshPayloadResponse {
  code?: number
  message?: string
  data?: {
    token?: string
    expires_in?: number
  }
}

/** 单例 in-flight Promise：并发触发的刷新合并为同一次网络请求 */
let inflightRefresh: Promise<boolean> | null = null
/** 主动续签检查的节流时间戳 */
let lastProactiveCheckAt = 0
/** 最近一次确认的 token 签发时间。页面冷加载后未知（按乐观值处理），本模块刷新成功后更新 */
let lastTokenIssuedAt = 0

/**
 * 执行一次刷新网络请求。
 * 任何失败（网络、HTTP、业务码、载荷缺失）都只返回 false，绝不递归触发刷新。
 */
async function requestRefresh(): Promise<boolean> {
  try {
    const currentToken = localStg.get('token')
    if (!currentToken) {
      return false
    }

    const response = await refreshHttp.get<RefreshPayloadResponse>('/user/refresh', {
      headers: {
        'x-token': currentToken
      }
    })

    const payload = response.data
    const newToken = payload?.data?.token
    if (payload?.code !== 200 || !newToken) {
      logger.warn('[AuthRefresh] 刷新被拒绝:', payload?.code ?? response.status, payload?.message)
      return false
    }

    const issuedAt = Date.now()
    // 与登录流程一致的存储口径：token_expires_in 存绝对过期时间戳(ms)
    localStg.set('token', newToken)
    const expiresIn = Number(payload.data?.expires_in)
    const expiresAtMs = Number.isFinite(expiresIn) ? issuedAt + expiresIn * 1000 : issuedAt
    localStg.set('token_expires_in', expiresAtMs.toString())
    lastTokenIssuedAt = issuedAt

    // 广播给 WebSocket 等持有旧 token 副本的模块，供其重建连接
    window.dispatchEvent(new CustomEvent(AUTH_TOKEN_REFRESHED_EVENT))
    return true
  } catch (error) {
    logger.warn('[AuthRefresh] 刷新请求失败:', error instanceof Error ? error.message : error)
    return false
  }
}

/**
 * 刷新平台 token：并发调用合并为一次网络请求。
 * 成功写回 localStorage 并广播事件返回 true；任何失败返回 false 且不递归。
 */
export function refreshAuthToken(): Promise<boolean> {
  if (!inflightRefresh) {
    inflightRefresh = requestRefresh().finally(() => {
      inflightRefresh = null
    })
  }
  return inflightRefresh
}

/**
 * 计算当前 token 的有效剩余寿命：
 * min(session TTL 剩余, JWT 24h 上限剩余)。未登录或无存储数据返回 null。
 */
function getTokenRemainingLifetimeMs(now: number): number | null {
  if (!localStg.get('token')) return null

  const storedExpiry = Number.parseInt(localStg.get('token_expires_in') || '', 10)
  if (Number.isNaN(storedExpiry)) return null

  const remainingBySessionTtl = storedExpiry - now
  // 冷加载后签发时间未知，按乐观值处理（视为刚签发，仅受 session TTL 约束）
  const jwtAgeMs = lastTokenIssuedAt > 0 ? now - lastTokenIssuedAt : 0
  const remainingByJwtMaxAge = Math.max(JWT_MAX_AGE_MS - jwtAgeMs, 0)

  return Math.min(remainingBySessionTtl, remainingByJwtMaxAge)
}

/**
 * 惰性主动续签入口（fire-and-forget，由 onRequest 调用）。
 * 以模块级时间戳节流：同一间隔内重复调用不会重复检查或发起刷新。
 */
export function scheduleProactiveTokenRefresh(): void {
  try {
    const now = Date.now()
    if (now - lastProactiveCheckAt < PROACTIVE_CHECK_INTERVAL_MS) return
    lastProactiveCheckAt = now

    const remainingMs = getTokenRemainingLifetimeMs(now)
    if (remainingMs === null || remainingMs >= PROACTIVE_REFRESH_THRESHOLD_MS) return

    void refreshAuthToken()
  } catch {
    // 主动续签属于尽力而为，任何异常都不能影响业务请求链路
  }
}
