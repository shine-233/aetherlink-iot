import { beforeEach, describe, expect, it, vi } from 'vitest'

import { DataItemProcessor } from './DataItemProcessor'

const { scriptEngineMock } = vi.hoisted(() => ({
  scriptEngineMock: {
    execute: vi.fn()
  }
}))

vi.mock('@/core/script-engine', () => ({
  defaultScriptEngine: scriptEngineMock
}))

describe('DataItemProcessor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it.each([0, false, ''])('preserves the falsy default value %j for missing input', async defaultValue => {
    const processor = new DataItemProcessor()

    await expect(processor.processData(null, { filterPath: '$', defaultValue })).resolves.toBe(defaultValue)
  })

  it.each([0, false, ''])('preserves the falsy default value %j when a filter resolves to null', async defaultValue => {
    const processor = new DataItemProcessor()

    await expect(
      processor.processData({ device: null }, { filterPath: '$.device.temperature', defaultValue })
    ).resolves.toBe(defaultValue)
  })

  it.each([0, false, ''])('preserves the falsy script result %j', async scriptValue => {
    scriptEngineMock.execute.mockResolvedValue({ success: true, data: scriptValue })
    const processor = new DataItemProcessor()

    await expect(
      processor.processData({ temperature: 26 }, { filterPath: '$', customScript: 'return data' })
    ).resolves.toBe(scriptValue)
  })

  it('falls back to an empty object only when no default value is configured', async () => {
    const processor = new DataItemProcessor()

    await expect(processor.processData(undefined, { filterPath: '$' })).resolves.toEqual({})
  })

  it('filters nested properties and consecutive array indexes with the supported JSONPath subset', async () => {
    const processor = new DataItemProcessor()

    await expect(
      processor.processData({ items: [{ name: 'sensor-a' }] }, { filterPath: '$.items[0].name' })
    ).resolves.toBe('sensor-a')
    await expect(
      processor.processData([['first', 'second']], { filterPath: '$[0][1]' })
    ).resolves.toBe('second')
  })

  it('validates exactly the same JSONPath subset used at runtime', () => {
    const processor = new DataItemProcessor()

    for (const path of ['', '$', 'items[0]', '$[0]', '$.items[0].name', '$[0][1]']) {
      expect(processor.validateFilterPath(path)).toBe(true)
    }
    for (const path of ['$.items[]', '$.items[0]name', '$.items..name', '$.[0]']) {
      expect(processor.validateFilterPath(path)).toBe(false)
    }
  })

  it('uses the configured default value when the filter path is malformed', async () => {
    const processor = new DataItemProcessor()

    await expect(
      processor.processData({ items: [{ name: 'sensor-a' }] }, { filterPath: '$.items[]', defaultValue: 'invalid-path' })
    ).resolves.toBe('invalid-path')
  })
})
