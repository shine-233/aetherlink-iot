import { describe, expect, it } from 'vitest'

import { ConfigurationAdapter } from './ConfigurationAdapter'
import type { DataSourceConfiguration } from '../executors/MultiLayerExecutorChain'

function createHttpConfiguration(
  body: unknown,
  scripts: { preRequestScript?: string; postResponseScript?: string } = {}
): DataSourceConfiguration {
  return {
    componentId: 'component-1',
    dataSources: [
      {
        sourceId: 'http-source',
        dataItems: [
          {
            item: {
              type: 'http',
              config: {
                url: 'https://example.test/data',
                method: 'POST',
                body,
                ...scripts
              }
            },
            processing: {}
          }
        ],
        mergeStrategy: { type: 'object' }
      }
    ],
    createdAt: 1,
    updatedAt: 1
  }
}

describe('ConfigurationAdapter HTTP body conversion', () => {
  it.each([0, false, ''])('preserves the falsy body %j through a v1/v2 round trip', body => {
    const adapter = new ConfigurationAdapter()
    const original = createHttpConfiguration(body)

    const upgraded = adapter.upgradeV1ToV2(original)
    const upgradedConfig = upgraded.dataSources[0].dataItems[0].item.config as {
      body?: { type: string; content: unknown }
    }
    expect(upgradedConfig.body).toEqual({ type: 'json', content: body })

    const downgraded = adapter.downgradeV2ToV1(upgraded)
    expect(downgraded.dataSources[0].dataItems[0].item.config.body).toBe(body)
  })

  it.each([null, undefined])('treats the absent body %j as undefined', body => {
    const adapter = new ConfigurationAdapter()
    const upgraded = adapter.upgradeV1ToV2(createHttpConfiguration(body))
    const upgradedConfig = upgraded.dataSources[0].dataItems[0].item.config as {
      body?: { type: string; content: unknown }
    }

    expect(upgradedConfig.body).toBeUndefined()
  })

  it('preserves request and response scripts through a v1/v2 round trip', () => {
    const adapter = new ConfigurationAdapter()
    const original = createHttpConfiguration(undefined, {
      preRequestScript: 'config.headers.Authorization = token',
      postResponseScript: 'return response.data'
    })

    const upgraded = adapter.upgradeV1ToV2(original)
    const upgradedConfig = upgraded.dataSources[0].dataItems[0].item.config as {
      preRequestScript?: string
      responseScript?: string
    }
    expect(upgradedConfig.preRequestScript).toBe('config.headers.Authorization = token')
    expect(upgradedConfig.responseScript).toBe('return response.data')

    const downgraded = adapter.downgradeV2ToV1(upgraded)
    const downgradedConfig = downgraded.dataSources[0].dataItems[0].item.config
    expect(downgradedConfig.preRequestScript).toBe('config.headers.Authorization = token')
    expect(downgradedConfig.postResponseScript).toBe('return response.data')
  })

  it('blocks HTTP parameter migration instead of silently dropping the configuration', () => {
    const adapter = new ConfigurationAdapter()
    const original = createHttpConfiguration(undefined)
    const httpConfig = original.dataSources[0].dataItems[0].item.config
    httpConfig.params = [
      {
        key: 'deviceId',
        value: 0,
        enabled: true,
        isDynamic: false,
        dataType: 'number',
        variableName: 'var_device_id',
        description: 'Device ID',
        paramType: 'query'
      }
    ]

    expect(() => adapter.upgradeV1ToV2(original)).toThrow('UNSUPPORTED_HTTP_PARAMETER_MIGRATION')

    const result = adapter.adaptToVersion(original, 'v2.0')
    expect(result.success).toBe(false)
    expect(result.data).toBeUndefined()
    expect(result.errors).toEqual(['UNSUPPORTED_HTTP_PARAMETER_MIGRATION'])
  })

  it('reports an unsupported plugin type instead of creating a fake v1 item', () => {
    const adapter = new ConfigurationAdapter()
    const pluginConfig = {
      componentId: 'component-1',
      version: '2.0.0',
      dataSources: [
        {
          sourceId: 'plugin-source',
          dataItems: [
            {
              item: { type: 'plugin-stream', id: 'plugin-1', config: { endpoint: 'external' } },
              processing: {}
            }
          ],
          mergeStrategy: { type: 'object' as const }
        }
      ],
      dynamicParams: [],
      enhancedFeatures: {},
      createdAt: 1,
      updatedAt: 1
    }

    expect(() => adapter.downgradeV2ToV1(pluginConfig as any)).toThrow(
      'UNSUPPORTED_DATA_ITEM_TYPE:plugin-stream'
    )

    const result = adapter.adaptToVersion(pluginConfig, 'v1.0')
    expect(result.success).toBe(false)
    expect(result.data).toBeUndefined()
    expect(result.errors).toEqual(['UNSUPPORTED_DATA_ITEM_TYPE:plugin-stream'])
  })

  it('returns an isolated snapshot when the source already matches the target version', () => {
    const adapter = new ConfigurationAdapter()
    const original = {
      version: '2.0.0',
      componentId: 'component-1',
      nested: { value: 1 }
    }

    const result = adapter.adaptToVersion(original, 'v2.0')
    expect(result.success).toBe(true)

    result.data.nested.value = 2
    expect(original.nested).toEqual({ value: 1 })
  })

  it.each(['__proto__', 'prototype', 'constructor'])('filters the unsafe HTTP header key %s in both directions', key => {
    const adapter = new ConfigurationAdapter()
    const original = createHttpConfiguration(undefined)
    const headers = Object.create(null)
    headers['X-Safe'] = 'value'
    headers[key] = 'polluted'
    original.dataSources[0].dataItems[0].item.config.headers = headers

    const upgraded = adapter.upgradeV1ToV2(original)
    const upgradedConfig = upgraded.dataSources[0].dataItems[0].item.config as any
    expect(upgradedConfig.headers).toEqual([
      { key: 'X-Safe', value: 'value', enabled: true, isDynamic: false }
    ])

    upgradedConfig.headers.push({ key, value: 'polluted', enabled: true, isDynamic: false })
    const downgraded = adapter.downgradeV2ToV1(upgraded)
    expect(downgraded.dataSources[0].dataItems[0].item.config.headers).toEqual({ 'X-Safe': 'value' })
  })
})
