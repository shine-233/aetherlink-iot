/**
 * 文件用途：验证共享剪贴板工具在标准 Clipboard API 和受限上下文回退路径下的行为。
 * 核心逻辑：覆盖成功写入、textarea 回退、空文本和回退异常后的 DOM 清理。
 * 关键注意事项：测试必须保证临时 textarea 始终被移除，避免客户页面积累隐藏节点。
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { writeClipboardText } from '../clipboard'

describe('writeClipboardText', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('uses the Clipboard API when it succeeds', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(window.navigator, 'clipboard', {
      configurable: true,
      value: { writeText }
    })

    await expect(writeClipboardText('share-link')).resolves.toBe(true)
    expect(writeText).toHaveBeenCalledWith('share-link')
    expect(document.querySelector('textarea')).toBeNull()
  })

  it('falls back to a temporary textarea when Clipboard API access fails', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('clipboard blocked'))
    const execCommand = vi.fn().mockReturnValue(true)
    Object.defineProperty(window.navigator, 'clipboard', {
      configurable: true,
      value: { writeText }
    })
    Object.defineProperty(document, 'execCommand', {
      configurable: true,
      value: execCommand
    })

    await expect(writeClipboardText('fallback-link')).resolves.toBe(true)
    expect(execCommand).toHaveBeenCalledWith('copy')
    expect(document.querySelector('textarea')).toBeNull()
  })

  it('returns false for empty text without touching the clipboard', async () => {
    const writeText = vi.fn()
    Object.defineProperty(window.navigator, 'clipboard', {
      configurable: true,
      value: { writeText }
    })

    await expect(writeClipboardText('')).resolves.toBe(false)
    expect(writeText).not.toHaveBeenCalled()
  })

  it('returns false and removes the fallback textarea when copy is rejected', async () => {
    Object.defineProperty(window.navigator, 'clipboard', {
      configurable: true,
      value: undefined
    })
    Object.defineProperty(document, 'execCommand', {
      configurable: true,
      value: vi.fn(() => {
        throw new Error('copy unavailable')
      })
    })

    await expect(writeClipboardText('share-link')).resolves.toBe(false)
    expect(document.querySelector('textarea')).toBeNull()
  })
})
