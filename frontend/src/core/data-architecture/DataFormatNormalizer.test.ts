/**
 * Static regression coverage for the data-format normalizer compatibility and
 * standard-shape conversion paths.
 */

import { describe, expect, it, vi } from 'vitest'

import { DataFormatNormalizer, type StandardDataSourceConfig } from './DataFormatNormalizer'

describe('DataFormatNormalizer', () => {
  it('keeps standard data-source config unchanged', () => {
    const standard: StandardDataSourceConfig = {
      componentId: 'rdi-device-operations',
      createdAt: 100,
      updatedAt: 200,
      dataSources: [
        {
          sourceId: 'telemetry',
          mergeStrategy: { type: 'object' },
          dataItems: [
            {
              item: {
                type: 'http',
                config: { url: '/api/telemetry' }
              },
              processing: {
                filterPath: '$.data.temperature',
                defaultValue: null
              }
            }
          ]
        }
      ]
    }

    expect(DataFormatNormalizer.normalizeToStandard(standard, 'ignored')).toBe(standard)
  })

  it('normalizes persisted SimpleConfigurationEditor source schema data and wraps raw data items', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1710000000000)

    const result = DataFormatNormalizer.normalizeToStandard(
      {
        createdAt: 1700000000000,
        dataSources: [
          {
            sourceId: 'main',
            mergeStrategy: { type: 'array' },
            dataItems: [
              {
                type: 'json',
                config: { value: 42 },
                filterPath: '$.value',
                customScript: 'return data.value',
                defaultValue: 0
              },
              {
                item: { type: 'static', config: { ok: true } },
                processing: { filterPath: '$' }
              }
            ]
          }
        ]
      },
      'card-a'
    )

    expect(result).toMatchObject({
      componentId: 'card-a',
      createdAt: 1700000000000,
      updatedAt: 1710000000000,
      dataSources: [
        {
          sourceId: 'main',
          mergeStrategy: { type: 'array' },
          dataItems: [
            {
              item: { type: 'json', config: { value: 42 } },
              processing: {
                filterPath: '$.value',
                customScript: 'return data.value',
                defaultValue: 0
              }
            },
            {
              item: { type: 'static', config: { ok: true } },
              processing: { filterPath: '$' }
            }
          ]
        }
      ]
    })
  })

  it('normalizes import/export payloads that already contain standard data items', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1720000000000)

    const result = DataFormatNormalizer.normalizeToStandard(
      {
        dataSourceConfig: {
          id: 'persisted-source',
          mergeStrategy: 'array',
          dataItems: [
            {
              item: { type: 'http', config: { url: '/api/rdi/history' } },
              processing: { filterPath: '$.data.rows', customScript: 'return data.rows', defaultValue: [] }
            },
            {
              type: 'json',
              data: { ok: true },
              filterPath: '$.ok',
              processScript: 'return data.ok',
              defaultValue: false
            }
          ]
        }
      },
      'imported-card'
    )

    expect(result).toMatchObject({
      componentId: 'imported-card',
      createdAt: 1720000000000,
      updatedAt: 1720000000000,
      dataSources: [
        {
          sourceId: 'persisted-source',
          mergeStrategy: { type: 'array' },
          dataItems: [
            {
              item: { type: 'http', config: { url: '/api/rdi/history' } },
              processing: { filterPath: '$.data.rows', customScript: 'return data.rows', defaultValue: [] }
            },
            {
              item: { type: 'json', config: { ok: true } },
              processing: { filterPath: '$.ok', customScript: 'return data.ok', defaultValue: false }
            }
          ]
        }
      ]
    })
  })

  it('normalizes import/export, card executor, editor-manager, and generic object shapes', () => {
    const importExport = DataFormatNormalizer.normalizeToStandard(
      {
        dataSourceConfig: {
          mergeStrategy: { type: 'replace' },
          dataItems: [{ type: 'http', config: { url: '/api/devices' } }]
        }
      },
      'imported-card'
    )

    expect(importExport.dataSources[0]).toMatchObject({
      sourceId: 'main',
      mergeStrategy: { type: 'replace' },
      dataItems: [{ item: { type: 'http', config: { url: '/api/devices' } } }]
    })

    const cardExecutor = DataFormatNormalizer.normalizeToStandard(
      {
        telemetry: {
          type: 'json',
          data: { temperature: 26 },
          metadata: { source: 'mock' }
        }
      },
      'card-executor'
    )

    expect(cardExecutor.dataSources[0]).toMatchObject({
      sourceId: 'telemetry',
      dataItems: [
        {
          item: { type: 'json', config: { temperature: 26 } },
          processing: { filterPath: '$' }
        }
      ]
    })

    const editorManager = DataFormatNormalizer.normalizeToStandard(
      { type: 'file', config: { path: '/tmp/a.csv' }, filterPath: '$.rows', processScript: 'return rows' },
      'editor-card'
    )

    expect(editorManager.dataSources[0].dataItems[0]).toMatchObject({
      item: { type: 'file', config: { path: '/tmp/a.csv' } },
      processing: { filterPath: '$.rows', customScript: 'return rows' }
    })

    const generic = DataFormatNormalizer.normalizeToStandard({ loose: true }, 'generic-card')
    expect(generic.dataSources[0].dataItems[0]).toMatchObject({
      item: { type: 'static', config: { loose: true } },
      processing: { filterPath: '$' }
    })

    const cardExecutorAlias = DataFormatNormalizer.normalizeToStandard(
      {
        legacyApi: {
          type: 'api',
          data: { url: '/api/legacy' },
          metadata: { source: 'persisted' }
        }
      },
      'card-executor-alias'
    )
    expect(cardExecutorAlias.dataSources[0].dataItems[0].item.type).toBe('http')

    const editorManagerAlias = DataFormatNormalizer.normalizeToStandard(
      { type: 'api', config: { url: '/api/legacy' } },
      'editor-alias'
    )
    expect(editorManagerAlias.dataSources[0].dataItems[0].item.type).toBe('http')

    expect(() =>
      DataFormatNormalizer.normalizeToStandard(
        { type: 'mqtt', config: { topic: 'telemetry' } },
        'external-source'
      )
    ).toThrow('UNSUPPORTED_DATA_SOURCE_TYPE:mqtt')
  })

  it('converts standard config back to persisted schema and downstream runtime formats', () => {
    const standard: StandardDataSourceConfig = {
      componentId: 'rdi-card',
      createdAt: 1,
      updatedAt: 2,
      dataSources: [
        {
          sourceId: 'main',
          mergeStrategy: { type: 'object' },
          dataItems: [
            {
              item: { type: 'static', config: { a: 1 } },
              processing: { filterPath: '$.a' }
            },
            {
              item: { type: 'json', config: { b: 2 } },
              processing: { filterPath: '$.b' }
            }
          ]
        }
      ]
    }

    expect(DataFormatNormalizer.convertFromStandard(standard, 'simpleConfigEditor')).toMatchObject({
      dataSources: [{ sourceId: 'main', dataItems: standard.dataSources[0].dataItems }]
    })
    expect(DataFormatNormalizer.convertFromStandard(standard, 'importExport')).toEqual({
      dataSourceConfig: {
        dataItems: [
          { type: 'static', config: { a: 1 } },
          { type: 'json', config: { b: 2 } }
        ],
        mergeStrategy: { type: 'object' }
      }
    })
    expect(DataFormatNormalizer.convertFromStandard(standard, 'card2Executor')).toMatchObject({
      main_0: {
        type: 'static',
        data: { a: 1 },
        metadata: { sourceId: 'main', processing: { filterPath: '$.a' } }
      },
      main_1: {
        type: 'json',
        data: { b: 2 },
        metadata: { sourceId: 'main', processing: { filterPath: '$.b' } }
      }
    })
  })

  it('normalizes multiple component configs and validates standard format defects', () => {
    const results = DataFormatNormalizer.normalizeMultiple([
      { componentId: 'one', data: { type: 'static', config: { ok: true } } },
      { componentId: 'two', data: { loose: 'value' } }
    ])

    expect(results.map(item => item.componentId)).toEqual(['one', 'two'])
    expect(DataFormatNormalizer.validateStandardFormat(results[0]).valid).toBe(true)

    const invalid = DataFormatNormalizer.validateStandardFormat({
      componentId: '',
      createdAt: 1,
      updatedAt: 2,
      dataSources: [
        {
          sourceId: '',
          mergeStrategy: { type: 'object' },
          dataItems: [{ item: undefined, processing: undefined } as never]
        }
      ]
    })

    expect(invalid.valid).toBe(false)
    expect(invalid.errors).toEqual(
      expect.arrayContaining([
        'componentId is required',
        'dataSources[0].sourceId is required',
        'dataSources[0].dataItems[0] must include item and processing'
      ])
    )
  })
})
