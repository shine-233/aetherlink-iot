/**
 * 文件用途：验证 ThingsVis URL 构建的 token 安全边界。
 * 核心逻辑：SSO 成功时注入 SSO token；SSO 失败/无 token 时不得回退注入平台 JWT。
 * 关键注意事项：平台 JWT 一旦进入 iframe URL hash 会经浏览器历史与第三方页面泄露。
 * 重构建议：补齐 viewer/editor 模式与特殊字符参数的组合用例。
 */
import { describe, expect, it, vi, beforeEach } from 'vitest'

vi.mock('@/utils/thingsvis/thingsvis-auth', () => ({
  getThingsVisToken: vi.fn()
}))

vi.mock('@/utils/thingsvis/constants', () => ({
  getThingsVisStudioBaseUrl: () => 'https://studio.example.com',
  getThingsVisApiBase: () => 'https://api.example.com/thingsvis',
  getPlatformApiBase: () => 'https://api.example.com/platform'
}))

import { buildThingsVisUrl } from '@/utils/thingsvis/url-builder'
import { getThingsVisToken } from '@/utils/thingsvis/thingsvis-auth'

const mockedGetToken = vi.mocked(getThingsVisToken)

function tokenParamOf(url: string): string | null {
  const query = url.split('#')[1]?.split('?')[1] ?? ''
  return new URLSearchParams(query).get('token')
}

beforeEach(() => {
  mockedGetToken.mockReset()
})

describe('buildThingsVisUrl token handling', () => {
  it('injects the SSO token when exchange succeeds', async () => {
    mockedGetToken.mockResolvedValue('thingsvis-sso-token')

    const url = await buildThingsVisUrl({ mode: 'viewer' })

    expect(tokenParamOf(url)).toBe('thingsvis-sso-token')
  })

  it('does not fall back to the platform JWT when SSO returns no token', async () => {
    mockedGetToken.mockResolvedValue(null)
    localStorage.setItem('token', 'platform-secret-jwt')

    try {
      const url = await buildThingsVisUrl({ mode: 'viewer' })
      expect(tokenParamOf(url)).toBeNull()
      expect(url).not.toContain('platform-secret-jwt')
    } finally {
      localStorage.removeItem('token')
    }
  })

  it('does not fall back to the platform JWT when SSO exchange throws', async () => {
    mockedGetToken.mockRejectedValue(new Error('sso unavailable'))
    localStorage.setItem('token', 'platform-secret-jwt')

    try {
      const url = await buildThingsVisUrl({ mode: 'editor' })
      expect(tokenParamOf(url)).toBeNull()
      expect(url).not.toContain('platform-secret-jwt')
    } finally {
      localStorage.removeItem('token')
    }
  })
})
