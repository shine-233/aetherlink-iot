/*
 * 文件用途：提供 ThingsVis SSO 认证服务。
 * 核心逻辑：读取当前用户空间，向后端交换 SSO token，并管理缓存、刷新和清理。
 * 关键注意事项：token、spaceId 和错误日志都属于敏感边界，不能泄露或跨用户复用。
 * 重构建议：可把请求、缓存和日志降噪拆分，补充失败/过期分支测试。
 */
/**
 * ThingsVis SSO Authentication Service
 * 实现 AetherLink IoT Token 与 ThingsVis JWT Token 的交换
 */

import { localStg } from '@/utils/storage'
import { createLogger } from '@/utils/logger'
import { shouldSuppressUnmockedTestConsole } from '@/utils/test-console'
import type { SSOExchangeRequest, SSOExchangeResponse } from './types'
import { THINGSVIS_API_PROXY_PATH, THINGSVIS_COMPAT_PLATFORM, getThingsVisApiBase } from './constants'
import { resolveThingsVisSpaceId, type UserInfoLike } from './space'

const ssoLogger = createLogger('ThingsVisSSO')

function isUserInfoLike(value: unknown): value is UserInfoLike {
  if (!value || typeof value !== 'object') {
    return false
  }

  const candidate = value as Record<string, unknown>

  return (
    typeof candidate.userId === 'string' ||
    typeof candidate.id === 'string' ||
    typeof candidate.userName === 'string' ||
    typeof candidate.email === 'string' ||
    typeof candidate.authority === 'string'
  )
}

function ssoLog(...args: any[]): void {
  if (!shouldSuppressUnmockedTestConsole(console.log)) ssoLogger.debug(...args)
}

function ssoWarn(...args: any[]): void {
  if (!shouldSuppressUnmockedTestConsole(console.warn)) ssoLogger.warn(...args)
}

function ssoError(...args: any[]): void {
  if (!shouldSuppressUnmockedTestConsole(console.error)) ssoLogger.error(...args)
}

/**
 * ThingsVis SSO 认证服务类
 */
export class ThingsVisAuthService {
  private cachedToken: string | null = null
  private tokenExpiry: number = 0
  private cachedIdentityKey: string | null = null
  private exchangePromise: Promise<string> | null = null
  private thingsvisApiUrl: string
  private readonly thingsvisApiTarget: string
  private readonly networkFailureCooldownMs = 30_000
  private lastNetworkFailureAt: number = 0
  private lastNetworkFailureReason: string | null = null

  constructor() {
    // SSO API 地址
    // 前端固定请求代理路径 /thingsvis-api 以避免 CORS。
    // Vite 会将其重写为 `${VITE_THINGSVIS_API_URL}/api/v1/*`。
    this.thingsvisApiUrl = THINGSVIS_API_PROXY_PATH
    this.thingsvisApiTarget = import.meta.env.VITE_THINGSVIS_API_URL || getThingsVisApiBase()
  }

  private getFailureCooldownRemaining(): number {
    if (!this.lastNetworkFailureAt) return 0

    return Math.max(0, this.networkFailureCooldownMs - (Date.now() - this.lastNetworkFailureAt))
  }

  private markNetworkFailure(reason: string): Error {
    this.lastNetworkFailureAt = Date.now()
    this.lastNetworkFailureReason = reason

    return new Error(`ThingsVis SSO backend unavailable: ${this.thingsvisApiTarget} (${reason})`)
  }

  private clearNetworkFailure(): void {
    this.lastNetworkFailureAt = 0
    this.lastNetworkFailureReason = null
  }

  private resetTokenCache(clearExchangePromise: boolean = false): void {
    this.cachedToken = null
    this.tokenExpiry = 0
    this.cachedIdentityKey = null

    if (clearExchangePromise) {
      this.exchangePromise = null
    }
  }

  private isNetworkError(error: unknown): boolean {
    const message = error instanceof Error ? error.message : String(error || '')
    const normalized = message.toLowerCase()

    return (
      normalized.includes('failed to fetch') ||
      normalized.includes('networkerror') ||
      normalized.includes('load failed') ||
      normalized.includes('econnrefused')
    )
  }

  private shouldTreatResponseAsUnavailable(status: number, errorText: string): boolean {
    if ([502, 503, 504].includes(status)) return true

    return errorText.toLowerCase().includes('econnrefused')
  }

  /**
   * 等待 userInfo 就绪（处理首次登录时的竞态条件）
   * 当 loginByToken 还未完成 localStg.set('userInfo') 时，首页组件可能已挂载
   */
  private async waitForUserInfo(maxRetries: number = 5, intervalMs: number = 200): Promise<UserInfoLike | null> {
    const initialUserInfo = localStg.get('userInfo')
    let userInfo = isUserInfoLike(initialUserInfo) ? initialUserInfo : null
    let retries = 0

    while (!userInfo && retries < maxRetries) {
      retries++
      ssoLog(`[SSO] 等待 userInfo 就绪... (${retries}/${maxRetries})`)
      await new Promise((resolve) => setTimeout(resolve, intervalMs))
      const nextUserInfo = localStg.get('userInfo')
      userInfo = isUserInfoLike(nextUserInfo) ? nextUserInfo : null
    }

    if (!userInfo) {
      ssoWarn(`[SSO] userInfo 仍未就绪，共尝试 ${retries} 次`)
    }

    return userInfo
  }

  /**
   * 交换 AetherLink IoT Token -> ThingsVis Token
   */
  async exchangeToken(): Promise<string> {
    try {
      const cooldownRemaining = this.getFailureCooldownRemaining()
      if (cooldownRemaining > 0) {
        const retryAfterSeconds = Math.ceil(cooldownRemaining / 1000)
        const reason = this.lastNetworkFailureReason || 'recent network failure'
        throw new Error(
          `ThingsVis SSO backend unavailable: ${this.thingsvisApiTarget} (${reason}; retry in ${retryAfterSeconds}s)`
        )
      }

      // 1. 获取当前 AetherLink IoT 用户信息
      // 注意：首次登录时，userInfo 可能尚未写入 localStorage（竞态条件）
      // 需要等待 userInfo 就绪
      const tpToken = localStg.get('token')
      const userInfo = await this.waitForUserInfo()

      if (!tpToken) {
        throw new Error('AetherLink IoT token not found')
      }

      if (!userInfo) {
        throw new Error('User info not found')
      }

      const resolvedSpaceId = resolveThingsVisSpaceId(userInfo)
      this.cachedIdentityKey = `${userInfo.userId || userInfo.id || ''}::${resolvedSpaceId}`

      // 2. 构建 SSO 请求
      const request: SSOExchangeRequest = {
        platform: THINGSVIS_COMPAT_PLATFORM,
        platformToken: tpToken,
        userInfo: {
          id: userInfo.userId || userInfo.id || '',
          email: userInfo.email || `${userInfo.userName}@aetherlink.local`,
          name: userInfo.userName || 'AetherLink User',
          tenantId: resolvedSpaceId
        }
      }

      // 3. 映射 AetherLink IoT authority -> ThingsVis role，用于注册时初始化默认看板
      const authority = userInfo.authority
      let role: SSOExchangeRequest['role']

      if (authority === 'SYS_ADMIN') {
        role = 'SUPER_ADMIN'
      } else if (authority === 'TENANT_ADMIN') {
        role = 'TENANT_ADMIN'
      } else {
        role = 'EDITOR'
      }

      request.role = role
      ssoLog('[SSO] 角色映射:', { authority, role })

      // 3. 调用 ThingsVis SSO API (通过代理)
      // /thingsvis-api/auth/sso -> ${VITE_THINGSVIS_API_URL}/api/v1/auth/sso
      const ssoUrl = `${this.thingsvisApiUrl}/auth/sso`
      ssoLog('[SSO] 📡 调用 SSO API:', ssoUrl)
      ssoLog('[SSO] 请求目标:', this.thingsvisApiTarget)
      ssoLog('[SSO] 请求数据:', {
        platform: request.platform,
        userEmail: request.userInfo.email,
        role: request.role
      })

      const response = await fetch(ssoUrl, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(request)
      })

      ssoLog('[SSO] 响应状态:', response.status)

      if (!response.ok) {
        const errorText = await response.text()
        ssoError('[SSO] 响应错误:', errorText)
        if (this.shouldTreatResponseAsUnavailable(response.status, errorText)) {
          throw this.markNetworkFailure(`HTTP ${response.status}: ${errorText || 'proxy unavailable'}`)
        }
        throw new Error(`Token exchange failed: ${response.status} - ${errorText}`)
      }

      const data: SSOExchangeResponse = await response.json()
      this.clearNetworkFailure()

      // 4. 缓存 Token
      this.cachedToken = data.accessToken
      this.tokenExpiry = Date.now() + (data.expiresIn || 7200) * 1000 // 默认 2 小时

      ssoLog('✅ SSO Token exchange successful')

      return data.accessToken
    } catch (error) {
      // 清除缓存的 token
      this.resetTokenCache()

      if (this.isNetworkError(error)) {
        const unavailableError = this.markNetworkFailure(error instanceof Error ? error.message : String(error))
        ssoError('❌ SSO Token exchange failed:', unavailableError)
        throw unavailableError
      }

      ssoError('❌ SSO Token exchange failed:', error)
      throw error
    }
  }

  /**
   * 获取有效的 ThingsVis Token (自动刷新)
   */
  async getValidToken(): Promise<string> {
    // 注意：首次登录时，userInfo 可能尚未写入 localStorage
    // identityKey 检查需要等待 userInfo 就绪
    let userInfo = await this.waitForUserInfo(3, 100)
    const identityKey = userInfo
      ? `${userInfo.userId || userInfo.id || ''}::${resolveThingsVisSpaceId(userInfo)}`
      : null

    if (this.cachedIdentityKey && identityKey && this.cachedIdentityKey !== identityKey) {
      this.clearToken()
    }

    // Token 未过期，直接返回
    if (this.cachedToken && Date.now() < this.tokenExpiry) {
      ssoLog('🔄 Using cached ThingsVis token')
      return this.cachedToken
    }

    if (this.exchangePromise) {
      ssoLog('🔄 Waiting for in-flight ThingsVis token exchange...')
      return this.exchangePromise
    }

    // Token 过期或不存在，重新交换
    ssoLog('🔄 Token expired or not found, exchanging...')
    this.exchangePromise = this.exchangeToken().finally(() => {
      this.exchangePromise = null
    })
    return await this.exchangePromise
  }

  /**
   * 清除缓存的 Token
   */
  clearToken(): void {
    this.resetTokenCache(true)
  }

  /**
   * 检查 Token 是否有效
   */
  isTokenValid(): boolean {
    return Boolean(this.cachedToken && Date.now() < this.tokenExpiry)
  }

  /**
   * 获取 Token 过期时间
   */
  getTokenExpiry(): Date | null {
    if (!this.tokenExpiry) return null
    return new Date(this.tokenExpiry)
  }
}

/**
 * 单例实例
 */
export const thingsvisAuthService = new ThingsVisAuthService()

/**
 * 便捷方法：获取有效的 ThingsVis Token
 */
export async function getThingsVisToken(): Promise<string> {
  return thingsvisAuthService.getValidToken()
}

/**
 * 便捷方法：清除 ThingsVis Token
 */
export function clearThingsVisToken(): void {
  thingsvisAuthService.clearToken()
}
