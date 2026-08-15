/**
 * 文件用途: RDI composable useRdiShare 的单元测试。
 * 核心逻辑: 通过 mock API、store 或时间行为验证 composable 的状态输出、动作和异常分支。
 * 关键注意事项: 测试应聚焦 composable 契约，避免依赖 RDI 操作视图 DOM 细节。
 * 重构建议: 继续补成功、失败、空数据和清理生命周期用例，提升组合函数边界可信度。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCreateRdiShareToken, mockMessageSuccess } = vi.hoisted(() => ({
  mockCreateRdiShareToken: vi.fn(),
  mockMessageSuccess: vi.fn()
}))

vi.mock('@/service/api', () => ({
  createRdiShareToken: (...args: any[]) => mockCreateRdiShareToken(...args)
}))

vi.mock('@/utils/common/discrete', () => ({
  message: {
    success: mockMessageSuccess
  }
}))

import { useRdiShare } from '../useRdiShare'

function createComposable() {
  return useRdiShare(() => 'dev-1', (key: any) => String(key))
}

describe('useRdiShare', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(window.navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: vi.fn().mockResolvedValue(undefined)
      }
    })
  })

  it('exposes the expected default expiry options', () => {
    const composable = createComposable()

    expect(composable.shareExpiryOptions.value).toEqual([
      { label: 'oneDay', value: 24 * 60 * 60 },
      { label: 'sevenDays', value: 7 * 24 * 60 * 60 },
      { label: 'thirtyDays', value: 30 * 24 * 60 * 60 }
    ])
    expect(composable.shareExpiresIn.value).toBe(7 * 24 * 60 * 60)
    expect(composable.shareExpiresAt.value).toBe('')
  })

  it('resets the generated link and expiry back to the default option', () => {
    const composable = createComposable()
    composable.shareLink.value = 'http://localhost/shared/abc'
    composable.shareExpiresIn.value = 24 * 60 * 60

    const result = composable.resetShareState()

    expect(composable.shareLink.value).toBe('')
    expect(composable.shareExpiresIn.value).toBe(7 * 24 * 60 * 60)
    expect(result).toBe('')
  })

  it('creates and copies a share link from share_path', async () => {
    mockCreateRdiShareToken.mockResolvedValue({
      error: null,
      data: {
        token: 'token-1',
        share_path: '/shared/rdi/token-1'
      }
    })

    const composable = createComposable()
    composable.shareExpiresIn.value = 24 * 60 * 60

    await composable.createShareLink()

    expect(mockCreateRdiShareToken).toHaveBeenCalledWith('dev-1', { expires_in: 24 * 60 * 60 })
    expect(composable.shareLink.value).toBe(`${window.location.origin}/shared/rdi/token-1`)
    expect(window.navigator.clipboard.writeText).toHaveBeenCalledWith(`${window.location.origin}/shared/rdi/token-1`)
    expect(mockMessageSuccess).toHaveBeenCalledWith('sent')
    expect(composable.shareLoading.value).toBe(false)
  })

  it('falls back to the public share page when the API only returns a token', async () => {
    mockCreateRdiShareToken.mockResolvedValue({
      error: null,
      data: {
        token: 'token-2'
      }
    })

    const composable = createComposable()

    await composable.createShareLink()

    expect(composable.shareLink.value).toBe(`${window.location.origin}/device/share?share_token=token-2`)
    expect(window.navigator.clipboard.writeText).toHaveBeenCalledWith(
      `${window.location.origin}/device/share?share_token=token-2`
    )
  })

  it('copies through the textarea fallback when the Clipboard API is unavailable', async () => {
    Object.defineProperty(window.navigator, 'clipboard', {
      configurable: true,
      value: undefined
    })
    const execCommand = vi.fn().mockReturnValue(true)
    Object.defineProperty(document, 'execCommand', {
      configurable: true,
      value: execCommand
    })
    mockCreateRdiShareToken.mockResolvedValue({
      error: null,
      data: {
        token: 'token-fallback'
      }
    })

    const composable = createComposable()
    await composable.createShareLink()

    expect(execCommand).toHaveBeenCalledWith('copy')
    expect(mockMessageSuccess).toHaveBeenCalledWith('sent')
    expect(composable.shareLink.value).toContain('share_token=token-fallback')
  })

  it('does not copy or show success when no share link exists', async () => {
    const composable = createComposable()

    await composable.copyShareLink()

    expect(window.navigator.clipboard.writeText).toHaveBeenCalledTimes(0)
    expect(mockMessageSuccess).toHaveBeenCalledTimes(0)
  })

  it('clears loading and leaves the link untouched when share creation fails', async () => {
    mockCreateRdiShareToken.mockResolvedValue({
      error: 'request failed',
      data: null
    })

    const composable = createComposable()

    await composable.createShareLink()

    expect(composable.shareLink.value).toBe('')
    expect(window.navigator.clipboard.writeText).toHaveBeenCalledTimes(0)
    expect(mockMessageSuccess).toHaveBeenCalledTimes(0)
    expect(composable.shareLoading.value).toBe(false)
  })
})
