import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { mockWriteClipboardText } = vi.hoisted(() => ({ mockWriteClipboardText: vi.fn() }))

vi.mock('@/utils/clipboard', () => ({ writeClipboardText: mockWriteClipboardText }))
vi.mock('@/locales', () => ({ $t: (key: string) => `t:${key}` }))

import { compactValueText, copyTextWithFeedback, downloadJsonWithFeedback } from '../detail-feedback'

describe('detail-feedback', () => {
  const message = { success: vi.fn(), error: vi.fn(), warning: vi.fn() }

  beforeEach(() => {
    vi.clearAllMocks()
    ;(window as any).$message = message
    window.URL.createObjectURL = vi.fn(() => 'blob:mock')
    window.URL.revokeObjectURL = vi.fn()
  })

  afterEach(() => {
    delete (window as any).$message
  })

  it('copies text and reports success or failure', async () => {
    mockWriteClipboardText.mockResolvedValueOnce(true)
    await expect(copyTextWithFeedback('abc')).resolves.toBe(true)
    expect(mockWriteClipboardText).toHaveBeenCalledWith('abc')
    expect(message.success).toHaveBeenCalledWith('t:theme.configOperation.copySuccess')

    mockWriteClipboardText.mockResolvedValueOnce(false)
    await expect(copyTextWithFeedback('abc')).resolves.toBe(false)
    expect(message.error).toHaveBeenCalledWith('t:common.copyFailed')
  })

  it('downloads JSON via an anchor and revokes the object URL', () => {
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const ok = downloadJsonWithFeedback(() => ({ a: 1 }), {
      fileName: 'bundle.json',
      successKey: 'ok',
      failureKey: 'fail'
    })
    expect(ok).toBe(true)
    expect(click).toHaveBeenCalledTimes(1)
    expect(window.URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock')
    expect(message.success).toHaveBeenCalledWith('t:ok')
    expect(document.querySelector('a[download]')).toBeNull()
    click.mockRestore()
  })

  it('reports failure with the configured level when payload construction throws', () => {
    const ok = downloadJsonWithFeedback(
      () => {
        throw new Error('boom')
      },
      { fileName: 'x.json', successKey: 'ok', failureKey: 'fail', failureLevel: 'warning' }
    )
    expect(ok).toBe(false)
    expect(message.warning).toHaveBeenCalledWith('t:fail')
    expect(message.success).not.toHaveBeenCalled()
  })

  it('compacts values for single-line display', () => {
    expect(compactValueText(undefined)).toBe('--')
    expect(compactValueText('')).toBe('--')
    expect(compactValueText('abc')).toBe('abc')
    expect(compactValueText({ v: 1 })).toBe('{"v":1}')
    expect(compactValueText('x'.repeat(130))).toBe(`${'x'.repeat(120)}...`)
    expect(compactValueText('abcdef', 3)).toBe('abc...')
  })
})
