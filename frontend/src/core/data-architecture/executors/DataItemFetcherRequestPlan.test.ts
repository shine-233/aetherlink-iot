/**
 * 文件用途：DataItemFetcher request-plan helper 的 focused 单元测试。
 * 核心逻辑：验证已解析参数如何构建 URL、query、headers、body 和缓存 key。
 * 关键注意事项：这些测试不 mock request wrapper，避免把网络执行细节绑进纯函数测试。
 * 重构建议：新增 legacy/raw 兼容规则时先补这里，再补 DataItemFetcher 行为测试。
 */
import { describe, expect, it } from 'vitest'

import type { HttpParameter } from '@/core/data-architecture/types/http-config'

import type { HttpDataItemConfig } from './DataItemFetcher'
import {
  buildHttpRequestPlan,
  collectHttpRequestParameterInputs,
  type ResolvedHttpParameter
} from './DataItemFetcherRequestPlan'

const httpParam = (overrides: Partial<HttpParameter> = {}): HttpParameter =>
  ({
    key: 'q',
    value: 'value',
    enabled: true,
    isDynamic: false,
    dataType: 'string',
    variableName: '',
    description: 'request param',
    paramType: 'query',
    ...overrides
  }) as HttpParameter

const httpConfig = (overrides: Partial<HttpDataItemConfig>): HttpDataItemConfig =>
  ({
    url: '/api/devices',
    method: 'GET',
    ...overrides
  }) as HttpDataItemConfig

const resolveInputsByValue = (config: HttpDataItemConfig): ResolvedHttpParameter[] =>
  collectHttpRequestParameterInputs(config).map(input => ({
    ...input,
    resolvedValue: input.param.value
  }))

describe('DataItemFetcherRequestPlan', () => {
  it('collects only request-affecting parameter inputs in runtime resolution order', () => {
    const config = httpConfig({
      pathParams: [
        httpParam({ key: 'disabled_path', value: 'ignored', enabled: false, paramType: 'path' }),
        httpParam({ key: 'device_id', value: 'dev-1', paramType: 'path' })
      ],
      pathParameter: httpParam({ key: 'device_id', value: 'stale-path-alias', paramType: 'path' }),
      params: [
        httpParam({ key: 'limit', value: '10' }),
        httpParam({ key: '', value: 'ignored-empty-key' }),
        httpParam({ key: 'disabled', value: 'ignored-disabled', enabled: false })
      ],
      parameters: [
        httpParam({ key: 'page', value: '2', paramType: 'query' }),
        httpParam({ key: 'X-Trace', value: 'trace-1', paramType: 'header' })
      ]
    })

    expect(
      collectHttpRequestParameterInputs(config).map(input => ({
        source: input.source,
        key: input.param.key,
        index: input.index
      }))
    ).toEqual([
      { source: 'pathParams', key: 'device_id', index: 1 },
      { source: 'params', key: 'limit', index: 0 },
      { source: 'parameters', key: 'page', index: 0 },
      { source: 'parameters', key: 'X-Trace', index: 1 }
    ])
  })

  it('builds URL, query params, headers, parsed body, and key material from current schema values', () => {
    const config = httpConfig({
      url: '/api/devices/{device_id}/telemetry',
      method: 'POST',
      timeout: 3000,
      headers: { Authorization: 'Bearer token' },
      body: '{"enabled":true}',
      pathParams: [httpParam({ key: 'device_id', value: 'dev-1', paramType: 'path' })],
      params: [httpParam({ key: 'limit', value: 10, dataType: 'number' })]
    })

    const plan = buildHttpRequestPlan(config, resolveInputsByValue(config))

    expect(plan).toEqual({
      method: 'POST',
      finalUrl: '/api/devices/dev-1/telemetry',
      requestConfig: {
        timeout: 3000,
        headers: { Authorization: 'Bearer token' },
        params: { limit: 10 }
      },
      requestBody: { enabled: true },
      keyMaterial:
        '{"body":{"enabled":true},"headers":{"Authorization":"Bearer token"},"method":"POST","params":{"limit":10},"timeout":3000,"url":"/api/devices/dev-1/telemetry"}'
    })
  })

  it('preserves legacy parameters compatibility without letting ignored aliases affect the cache key', () => {
    const baseConfig = httpConfig({
      url: '/api/device',
      method: 'PUT',
      body: '{not-json',
      headers: { 'X-Trace': 'trace-current' },
      params: [httpParam({ key: 'page', value: 2, dataType: 'number', paramType: 'query' })],
      parameters: [
        httpParam({ key: 'device_id', value: 'dev-legacy', paramType: 'path' }),
        httpParam({ key: 'page', value: 99, dataType: 'number', paramType: 'query' }),
        httpParam({ key: 'X-Trace', value: 'trace-legacy-a', paramType: 'header' }),
        httpParam({ key: 'X-Legacy', value: 'legacy-header', paramType: 'header' })
      ]
    })
    const changedIgnoredAliases = httpConfig({
      ...baseConfig,
      parameters: [
        httpParam({ key: 'device_id', value: 'dev-legacy', paramType: 'path' }),
        httpParam({ key: 'page', value: 100, dataType: 'number', paramType: 'query' }),
        httpParam({ key: 'X-Trace', value: 'trace-legacy-b', paramType: 'header' }),
        httpParam({ key: 'X-Legacy', value: 'legacy-header', paramType: 'header' })
      ]
    })

    const firstPlan = buildHttpRequestPlan(baseConfig, resolveInputsByValue(baseConfig))
    const secondPlan = buildHttpRequestPlan(changedIgnoredAliases, resolveInputsByValue(changedIgnoredAliases))

    expect(firstPlan.finalUrl).toBe('/api/device/dev-legacy')
    expect(firstPlan.requestConfig).toEqual({
      timeout: 10000,
      headers: { 'X-Trace': 'trace-current', 'X-Legacy': 'legacy-header' },
      params: { page: 2 }
    })
    expect(firstPlan.requestBody).toBe('{not-json')
    expect(secondPlan.keyMaterial).toBe(firstPlan.keyMaterial)
  })

  it('uses the persisted pathParameter alias when current pathParams are absent', () => {
    const config = httpConfig({
      url: '/api/devices/{device_id}/telemetry',
      pathParameter: httpParam({ key: '', value: 'dev-legacy', paramType: 'path' })
    })

    const plan = buildHttpRequestPlan(config, resolveInputsByValue(config))

    expect(plan.finalUrl).toBe('/api/devices/dev-legacy/telemetry')
    expect(plan.requestConfig).toEqual({ timeout: 10000 })
  })

  it.each(['__proto__', 'prototype', 'constructor'])('skips the unsafe current query key %s', key => {
    const config = httpConfig({
      params: [
        httpParam({ key: 'safe', value: 'value' }),
        httpParam({ key, value: 'polluted' })
      ]
    })

    const plan = buildHttpRequestPlan(config, resolveInputsByValue(config))

    expect(plan.requestConfig.params).toEqual({ safe: 'value' })
    expect(Object.getPrototypeOf(plan.requestConfig.params!)).toBe(Object.prototype)
  })

  it.each(['__proto__', 'prototype', 'constructor'])('skips the unsafe compatibility header key %s', key => {
    const config = httpConfig({
      parameters: [
        httpParam({ key: 'X-Safe', value: 'value', paramType: 'header' }),
        httpParam({ key, value: 'polluted', paramType: 'header' })
      ]
    })

    const plan = buildHttpRequestPlan(config, resolveInputsByValue(config))

    expect(plan.requestConfig.headers).toEqual({ 'X-Safe': 'value' })
    expect(Object.getPrototypeOf(plan.requestConfig.headers!)).toBe(Object.prototype)
  })
})
