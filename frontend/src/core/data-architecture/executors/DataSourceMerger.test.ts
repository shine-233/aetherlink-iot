import { beforeEach, describe, expect, it, vi } from 'vitest'

import { DataSourceMerger } from './DataSourceMerger'

const { scriptEngineMock } = vi.hoisted(() => ({
  scriptEngineMock: {
    execute: vi.fn()
  }
}))

vi.mock('@/core/script-engine', () => ({
  defaultScriptEngine: scriptEngineMock
}))

describe('DataSourceMerger', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('selects the configured data item', async () => {
    const merger = new DataSourceMerger()

    await expect(merger.mergeDataItems([{ id: 1 }, { id: 2 }], { type: 'select', selectedIndex: 1 })).resolves.toEqual({
      id: 2
    })
  })

  it('falls back to the first item when the selected index is out of bounds', async () => {
    const merger = new DataSourceMerger()

    await expect(merger.mergeDataItems(['first', 'second'], { type: 'select', selectedIndex: 5 })).resolves.toBe(
      'first'
    )
  })

  it.each([-1, 1.5, Number.NaN])('falls back to the first item for the invalid persisted index %s', async selectedIndex => {
    const merger = new DataSourceMerger()

    await expect(
      merger.mergeDataItems(['first', 'second'], { type: 'select', selectedIndex } as any)
    ).resolves.toBe('first')
  })

  it.each([0, false, ''])('preserves the selected falsy value %j', async selectedValue => {
    const merger = new DataSourceMerger()

    await expect(merger.mergeDataItems([selectedValue, 'fallback'], { type: 'select' })).resolves.toBe(selectedValue)
  })

  it('returns an empty object when no items are available', async () => {
    const merger = new DataSourceMerger()

    await expect(merger.mergeDataItems([], { type: 'select', selectedIndex: 0 })).resolves.toEqual({})
  })

  it.each([0, false, '', null])('preserves the successful script result %j', async scriptValue => {
    scriptEngineMock.execute.mockResolvedValue({ success: true, data: scriptValue })
    const merger = new DataSourceMerger()

    await expect(merger.mergeDataItems([{ value: 1 }], { type: 'script', script: 'return items[0]' })).resolves.toBe(
      scriptValue
    )
  })

  it('returns an empty object when script execution fails', async () => {
    scriptEngineMock.execute.mockResolvedValue({ success: false, error: 'blocked' })
    const merger = new DataSourceMerger()

    await expect(merger.mergeDataItems([{ value: 1 }], { type: 'script', script: 'return items[0]' })).resolves.toEqual(
      {}
    )
  })

  it('validates every supported public merge strategy', () => {
    const merger = new DataSourceMerger()

    expect(merger.validateMergeStrategy({ type: 'object' })).toBe(true)
    expect(merger.validateMergeStrategy({ type: 'array' })).toBe(true)
    expect(merger.validateMergeStrategy({ type: 'select', selectedIndex: 0 })).toBe(true)
    expect(merger.validateMergeStrategy({ type: 'script', script: 'return items' })).toBe(true)
    expect(merger.validateMergeStrategy({ type: 'script', script: '' })).toBe(false)
  })
})
