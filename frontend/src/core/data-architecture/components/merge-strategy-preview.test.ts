import { beforeEach, describe, expect, it, vi } from 'vitest'

import { previewMergeStrategy } from './merge-strategy-preview'

const { scriptEngineMock } = vi.hoisted(() => ({
  scriptEngineMock: {
    execute: vi.fn()
  }
}))

vi.mock('@/core/script-engine', () => ({
  defaultScriptEngine: scriptEngineMock
}))

describe('previewMergeStrategy', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('previews object, array, and condition strategies locally', async () => {
    const items = [{ temperature: 25 }, null, { online: true }]

    await expect(previewMergeStrategy(items, { type: 'object' })).resolves.toEqual({
      success: true,
      data: { temperature: 25, online: true }
    })
    await expect(previewMergeStrategy(items, { type: 'array' })).resolves.toEqual({ success: true, data: items })
    await expect(previewMergeStrategy(items, { type: 'condition' })).resolves.toEqual({
      success: true,
      data: { temperature: 25 }
    })
  })

  it.each([0, false, '', null])('preserves the successful script preview result %j', async scriptValue => {
    scriptEngineMock.execute.mockResolvedValue({ success: true, data: scriptValue })

    await expect(previewMergeStrategy([{ value: 1 }], { type: 'script', script: 'return items[0]' })).resolves.toEqual({
      success: true,
      data: scriptValue
    })
  })

  it('returns an explicit error for an empty script', async () => {
    await expect(previewMergeStrategy([], { type: 'script', script: '  ' })).resolves.toEqual({
      success: false,
      error: '请输入合并脚本后再预览'
    })
    expect(scriptEngineMock.execute).not.toHaveBeenCalled()
  })

  it('surfaces script-engine failures without host evaluation fallback', async () => {
    scriptEngineMock.execute.mockResolvedValue({ success: false, error: 'SCRIPT_NETWORK_EXTERNAL_BLOCKED' })

    await expect(previewMergeStrategy([], { type: 'script', script: 'return fetch("/")' })).resolves.toEqual({
      success: false,
      error: 'SCRIPT_NETWORK_EXTERNAL_BLOCKED'
    })
  })
})
