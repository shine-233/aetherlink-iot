/**
 * 文件用途: ConfigToSimpleDataAdapter 的转换边界测试。
 * 核心逻辑: 验证历史配置归一、本地静态数据转换和不支持类型的显式阻断。
 * 关键注意事项: 适配器必须保留旧配置契约，但不能把未知类型透传给运行时执行器。
 */

import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  batchConvertConfigs,
  convertToSimpleDataRequirement,
  extractComponentType,
  shouldConvertConfig
} from './ConfigToSimpleDataAdapter'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('ConfigToSimpleDataAdapter', () => {
  it('converts a simple object into a local static data source', () => {
    expect(convertToSimpleDataRequirement('card-1', { value: 42 })).toEqual({
      componentId: 'card-1',
      dataSources: [
        {
          id: 'main',
          type: 'static',
          config: { data: { value: 42 } }
        }
      ]
    })
  })

  it('parses JSON rawData from legacy binding maps', () => {
    expect(
      convertToSimpleDataRequirement('card-2', {
        dataSourceBindings: {
          primary: { rawData: '{"temperature":21.5}' }
        }
      })
    ).toEqual({
      componentId: 'card-2',
      dataSources: [
        {
          id: 'primary',
          type: 'static',
          config: { data: { temperature: 21.5 } }
        }
      ]
    })
  })

  it('normalizes the persisted api alias to the runtime http type', () => {
    const result = convertToSimpleDataRequirement('card-3', {
      rawDataList: [
        {
          id: 'remote',
          type: 'api',
          config: { url: '/api/devices', method: 'GET' }
        },
        {
          id: 'disabled',
          type: 'static',
          enabled: false,
          config: { data: 'ignored' }
        }
      ]
    })

    expect(result?.dataSources).toEqual([
      {
        id: 'remote',
        type: 'http',
        config: { url: '/api/devices', method: 'GET' },
        filterPath: undefined,
        processScript: undefined
      }
    ])
  })

  it('blocks unknown data-source types instead of passing them to runtime', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const result = convertToSimpleDataRequirement('card-4', {
      rawDataList: [
        { id: 'unsupported', type: 'mqtt', config: { topic: 'devices' } },
        { id: 'local', type: 'static', config: { data: [1, 2, 3] } }
      ]
    })

    expect(result?.dataSources).toEqual([
      {
        id: 'local',
        type: 'static',
        config: { data: [1, 2, 3] },
        filterPath: undefined,
        processScript: undefined
      }
    ])
    expect(errorSpy).toHaveBeenCalledWith(
      '[ConfigAdapter] UNSUPPORTED_DATA_SOURCE_TYPE: unsupported (mqtt)'
    )
  })

  it('returns null for malformed raw JSON and reports the parse failure', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)

    expect(
      convertToSimpleDataRequirement('card-5', {
        dataSourceBindings: { primary: { rawData: '{broken' } }
      })
    ).toBeNull()
    expect(errorSpy).toHaveBeenCalled()
  })

  it('detects convertible shapes and extracts component metadata safely', () => {
    expect(shouldConvertConfig({ rawDataList: [] })).toBe(true)
    expect(shouldConvertConfig({ value: 1 })).toBe(true)
    expect(shouldConvertConfig({ type: 'static' })).toBe(false)
    expect(shouldConvertConfig(null)).toBe(false)
    expect(extractComponentType({ metadata: { componentType: 'rdi-card' } })).toBe('rdi-card')
    expect(extractComponentType(undefined)).toBe('unknown')
  })

  it('batch converts only eligible configurations with valid data sources', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined)

    expect(
      batchConvertConfigs({
        local: { status: 'ok' },
        ignored: { type: 'static', metadata: { componentType: 'existing' } },
        blocked: { rawDataList: [{ id: 'external', type: 'mqtt' }] }
      })
    ).toEqual({
      local: {
        componentId: 'local',
        dataSources: [
          {
            id: 'main',
            type: 'static',
            config: { data: { status: 'ok' } }
          }
        ]
      }
    })
  })
})
