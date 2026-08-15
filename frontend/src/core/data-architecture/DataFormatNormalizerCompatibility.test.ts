/**
 * 文件用途: DataFormatNormalizerCompatibility 的 focused 回归测试。
 * 核心逻辑: 锁定旧配置、rawData 数据项和 import/export 载荷的兼容归一化行为。
 * 关键注意事项: 测试只断言可观察兼容契约，不绑定 UI 组件实现。
 * 重构建议: 后续新增旧配置样本时，优先补充这里而不是扩大组件挂载测试。
 */

import { describe, expect, it } from 'vitest'

import { DataFormatNormalizer, type StandardDataItem } from './DataFormatNormalizer'
import {
  isImportExportFormat,
  isSimpleConfigEditorFormat,
  isStandardDataItem,
  normalizePersistedDataItem,
  normalizePersistedDataSourceType,
  normalizePersistedMergeStrategy,
  normalizePersistedSourceId
} from './DataFormatNormalizerCompatibility'

describe('DataFormatNormalizerCompatibility', () => {
  it('detects persisted editor and import/export configuration shapes', () => {
    expect(
      isSimpleConfigEditorFormat({
        dataSources: [{ id: 'legacy-source', dataItems: [] }]
      })
    ).toBe(true)
    expect(
      isSimpleConfigEditorFormat({
        dataSources: [{ _id: 'internal-only', dataItems: [] }]
      })
    ).toBe(false)

    expect(
      isImportExportFormat({
        dataSourceConfig: { dataItems: [] }
      })
    ).toBe(true)
  })

  it('keeps already-standard data items unchanged', () => {
    const standardItem: StandardDataItem = {
      item: { type: 'json', config: { value: 1 } },
      processing: { filterPath: '$.value' }
    }

    expect(isStandardDataItem(standardItem)).toBe(true)
    expect(normalizePersistedDataItem(standardItem)).toBe(standardItem)
  })

  it('normalizes persisted aliases and rejects unsupported source types', () => {
    expect(normalizePersistedDataSourceType('api')).toBe('http')
    expect(normalizePersistedDataSourceType('file')).toBe('file')
    expect(() => normalizePersistedDataSourceType('mqtt')).toThrow('UNSUPPORTED_DATA_SOURCE_TYPE:mqtt')

    const legacyWrappedItem = {
      item: { type: 'api', config: { url: '/api/devices' } },
      processing: { filterPath: '$.data' }
    }
    expect(isStandardDataItem(legacyWrappedItem)).toBe(false)
    expect(normalizePersistedDataItem(legacyWrappedItem)).toEqual({
      item: { type: 'http', config: { url: '/api/devices' } },
      processing: { filterPath: '$.data', customScript: undefined, defaultValue: undefined }
    })

    expect(
      isStandardDataItem({
        item: { type: 'mqtt', config: { topic: 'telemetry' } },
        processing: { filterPath: '$' }
      })
    ).toBe(false)
  })

  it('wraps old rawData items without dropping persisted row identifiers', () => {
    const rawItem = {
      _id: 'raw-row-1',
      type: 'json',
      rawData: '{"temperature":21}',
      filterPath: '$.temperature',
      processScript: 'return data.temperature',
      defaultValue: 0
    }

    const normalized = normalizePersistedDataItem(rawItem)

    expect(normalized).toMatchObject({
      item: {
        type: 'json',
        config: {
          _id: 'raw-row-1',
          rawData: '{"temperature":21}'
        }
      },
      processing: {
        filterPath: '$.temperature',
        customScript: 'return data.temperature',
        defaultValue: 0
      }
    })
    expect(normalized.item.config).toBe(rawItem)
  })

  it('preserves source identifier and merge strategy compatibility rules', () => {
    expect(normalizePersistedSourceId({ sourceId: 'current-source', id: 'legacy-source' }, 'fallback')).toBe(
      'current-source'
    )
    expect(normalizePersistedSourceId({ id: 'legacy-source' }, 'fallback')).toBe('legacy-source')
    expect(normalizePersistedSourceId({}, 'fallback')).toBe('fallback')

    expect(normalizePersistedMergeStrategy('array')).toEqual({ type: 'array' })
    expect(normalizePersistedMergeStrategy({ type: 'replace' })).toEqual({ type: 'replace' })
    expect(normalizePersistedMergeStrategy('invalid')).toEqual({ type: 'object' })
  })

  it('keeps rawData and _id through import/export normalization', () => {
    const result = DataFormatNormalizer.normalizeToStandard(
      {
        dataSourceConfig: {
          id: 'legacy-import-source',
          mergeStrategy: 'array',
          dataItems: [
            {
              _id: 'raw-row-2',
              type: 'json',
              rawData: '{"humidity":68}',
              filterPath: '$.humidity'
            }
          ]
        }
      },
      'imported-card'
    )

    expect(result.dataSources[0]).toMatchObject({
      sourceId: 'legacy-import-source',
      mergeStrategy: { type: 'array' },
      dataItems: [
        {
          item: {
            type: 'json',
            config: {
              _id: 'raw-row-2',
              rawData: '{"humidity":68}'
            }
          },
          processing: { filterPath: '$.humidity' }
        }
      ]
    })
  })
})
