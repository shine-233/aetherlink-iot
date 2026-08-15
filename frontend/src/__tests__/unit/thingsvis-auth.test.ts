/**
 * 文件用途：验证 ThingsVis 认证参数和嵌入令牌的单元契约。
 * 核心逻辑：构造运行时输入，断言 token、用户态和鉴权字段的组合结果。
 * 关键注意事项：测试保护嵌入边界，不应放宽为仅校验快照或存在性。
 * 重构建议：后续可补充异常 token、过期态和租户隔离的负例矩阵。
 */
/**
 * 文件：ThingsVis 鉴权单元测试。
 * 作用：验证 SSO 失败冷却逻辑和 AetherLink 平台别名请求体。
 * 依赖：依赖 Vitest、storage mock 与 ThingsVis 鉴权工具的动态导入。
 * 维护：鉴权请求字段或错误策略变化时同步更新断言和 mock 数据。
 */

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { THINGSVIS_COMPAT_ALIAS } from '@/utils/thingsvis/constants'

const mockGet = vi.fn()

vi.mock('@/utils/storage', () => ({
  localStg: {
    get: mockGet
  }
}))

describe('thingsvis-auth', () => {
  beforeEach(() => {
    vi.resetModules()
    vi.clearAllMocks()
    mockGet.mockImplementation((key: string) => {
      if (key === 'token') return 'platform-token'
      if (key === 'userInfo') {
        return {
          userId: 'user-1',
          userName: 'tester',
          email: 'tester@example.com',
          tenantId: 'tenant-1'
        }
      }
      return null
    })
  })

  it('enters cooldown after a network failure and skips the next fetch attempt', async () => {
    const fetchMock = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'))
    vi.stubGlobal('fetch', fetchMock)

    const { getThingsVisToken } = await import('@/utils/thingsvis/thingsvis-auth')

    await expect(getThingsVisToken()).rejects.toThrow(/ThingsVis SSO backend unavailable/)
    await expect(getThingsVisToken()).rejects.toThrow(/retry in/i)

    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('sends the AetherLink ThingsVis platform alias in the SSO request body', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ accessToken: 'thingsvis-token', expiresIn: 3600 })
    })
    vi.stubGlobal('fetch', fetchMock)

    const { getThingsVisToken } = await import('@/utils/thingsvis/thingsvis-auth')

    await expect(getThingsVisToken()).resolves.toBe('thingsvis-token')

    expect(fetchMock).toHaveBeenCalledWith(
      '/thingsvis-api/auth/sso',
      expect.objectContaining({
        method: 'POST',
        body: expect.any(String)
      })
    )

    const [, requestInit] = fetchMock.mock.calls[0]
    const body = JSON.parse(requestInit.body)
    expect(body).toMatchObject({
      platform: THINGSVIS_COMPAT_ALIAS,
      platformToken: 'platform-token',
      userInfo: {
        id: 'user-1',
        email: 'tester@example.com',
        name: 'tester',
        tenantId: 'tenant-1'
      },
      role: 'EDITOR'
    })
  })
})
