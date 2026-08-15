/**
 * 文件用途: Data Item Fetcher 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { DataItemFetcher, type DataItem, type HttpDataItemConfig } from './DataItemFetcher'

const { requestMock, scriptEngineMock, editorStoreMock, configurationBridgeMock } = vi.hoisted(() => ({
  requestMock: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn()
  },
  scriptEngineMock: {
    execute: vi.fn()
  },
  editorStoreMock: {
    nodes: [] as Array<{ id: string; properties: Record<string, any> }>
  },
  configurationBridgeMock: {
    getConfiguration: vi.fn()
  }
}))

vi.mock('@/service/request', () => ({
  request: requestMock
}))

vi.mock('@/core/script-engine', () => ({
  defaultScriptEngine: scriptEngineMock
}))

vi.mock('@/components/visual-editor/store/editor', () => ({
  useEditorStore: () => editorStoreMock
}))

vi.mock('@/components/visual-editor/configuration/ConfigurationIntegrationBridge', () => ({
  configurationIntegrationBridge: configurationBridgeMock
}))

const httpParam = (overrides: Partial<HttpDataItemConfig['params'][number]> = {}) => ({
  key: 'q',
  value: 'value',
  enabled: true,
  isDynamic: false,
  dataType: 'string' as const,
  variableName: '',
  description: 'query param',
  paramType: 'query' as const,
  ...overrides
})

const httpItem = (config: Partial<HttpDataItemConfig>): DataItem => ({
  type: 'http',
  config: {
    url: '/api/devices',
    method: 'GET',
    ...config
  } as HttpDataItemConfig
})

describe('DataItemFetcher', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllTimers()
    vi.clearAllMocks()
    editorStoreMock.nodes = []
    configurationBridgeMock.getConfiguration.mockReturnValue(null)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('parses JSON data sources and reports malformed JSON before returning an empty object', async () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const fetcher = new DataItemFetcher()

    await expect(fetcher.fetchData({ type: 'json', config: { jsonString: '{"temperature":26}' } })).resolves.toEqual({
      temperature: 26
    })
    await expect(fetcher.fetchData({ type: 'json', config: { jsonString: '{bad json' } })).resolves.toEqual({})
    expect(errorSpy).toHaveBeenCalledWith(
      '[DataItemFetcher] JSON data source parse failed:',
      expect.objectContaining({
        error: expect.any(String)
      })
    )
    errorSpy.mockRestore()
  })

  it('returns an explicit unsupported result for WebSocket data sources', async () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    const fetcher = new DataItemFetcher()

    await expect(
      fetcher.fetchData({
        type: 'websocket',
        config: { url: 'wss://example.test/telemetry', protocols: ['json'], reconnectInterval: 1000 }
      })
    ).resolves.toEqual({
      success: false,
      unsupported: true,
      error: {
        code: 'UNSUPPORTED_DATA_SOURCE',
        message: 'WebSocket data sources are not supported by DataItemFetcher fetchData.',
        type: 'websocket'
      }
    })

    expect(errorSpy).toHaveBeenCalledWith(
      '[DataItemFetcher] Unsupported data source:',
      expect.objectContaining({
        type: 'websocket',
        url: 'wss://example.test/telemetry'
      })
    )
    errorSpy.mockRestore()
  })

  it('builds GET requests with path parameters, query parameters, headers, and timeout', async () => {
    requestMock.get.mockResolvedValue({ data: [{ value: 26 }] })
    const fetcher = new DataItemFetcher()

    const result = await fetcher.fetchData(
      httpItem({
        url: '/api/devices/{device_id}/telemetry',
        timeout: 3000,
        headers: { Authorization: 'Bearer token' },
        pathParams: [httpParam({ key: 'device_id', value: 'dev-1', paramType: 'path' })],
        params: [
          httpParam({ key: 'limit', value: '10', dataType: 'number' }),
          httpParam({ key: 'disabled', value: 'ignored', enabled: false })
        ]
      })
    )

    expect(result).toEqual({ data: [{ value: 26 }] })
    expect(requestMock.get).toHaveBeenCalledWith('/api/devices/dev-1/telemetry', {
      timeout: 3000,
      headers: { Authorization: 'Bearer token' },
      params: { limit: 10 }
    })
  })

  it('maps persisted pathParameter aliases with empty keys to the first URL placeholder', async () => {
    requestMock.get.mockResolvedValue({ data: [{ value: 27 }] })
    const fetcher = new DataItemFetcher()

    await fetcher.fetchData(
      httpItem({
        url: '/api/devices/{device_id}/telemetry',
        pathParameter: httpParam({ key: '', value: 'dev-legacy', paramType: 'path' })
      })
    )

    expect(requestMock.get).toHaveBeenCalledWith('/api/devices/dev-legacy/telemetry', { timeout: 10000 })
  })

  it('maps the persisted parameters schema alias into appended path, query, and header values', async () => {
    requestMock.put.mockResolvedValue({ ok: true })
    const fetcher = new DataItemFetcher()

    await fetcher.fetchData(
      httpItem({
        url: '/api/device',
        method: 'PUT',
        body: '{"enabled":true}',
        parameters: [
          httpParam({ key: 'device_id', value: 'dev-2', paramType: 'path' }),
          httpParam({ key: 'page', value: '2', dataType: 'number', paramType: 'query' }),
          httpParam({ key: 'X-Trace', value: 'trace-1', paramType: 'header' })
        ]
      })
    )

    expect(requestMock.put).toHaveBeenCalledWith(
      '/api/device/dev-2',
      { enabled: true },
      {
        timeout: 10000,
        headers: { 'X-Trace': 'trace-1' },
        params: { page: 2 }
      }
    )
  })

  it('replaces URL placeholders from legacy unified path parameters before appending path segments', async () => {
    requestMock.put.mockResolvedValue({ ok: true })
    const fetcher = new DataItemFetcher()

    await fetcher.fetchData(
      httpItem({
        url: '/api/devices/{device_id}/telemetry',
        method: 'PUT',
        parameters: [httpParam({ key: 'device_id', value: 'dev-legacy-path', paramType: 'path' })]
      })
    )

    expect(requestMock.put).toHaveBeenCalledWith('/api/devices/dev-legacy-path/telemetry', undefined, {
      timeout: 10000
    })
  })

  it('keeps current query params while still honoring legacy unified path and header params in mixed configs', async () => {
    requestMock.put.mockResolvedValue({ ok: true })
    const fetcher = new DataItemFetcher()

    await fetcher.fetchData(
      httpItem({
        url: '/api/device',
        method: 'PUT',
        body: '{"enabled":true}',
        params: [httpParam({ key: 'page', value: '2', dataType: 'number', paramType: 'query' })],
        parameters: [
          httpParam({ key: 'device_id', value: 'dev-mixed', paramType: 'path' }),
          httpParam({ key: 'page', value: '99', dataType: 'number', paramType: 'query' }),
          httpParam({ key: 'X-Trace', value: 'trace-mixed', paramType: 'header' })
        ]
      })
    )

    expect(requestMock.put).toHaveBeenCalledWith(
      '/api/device/dev-mixed',
      { enabled: true },
      {
        timeout: 10000,
        headers: { 'X-Trace': 'trace-mixed' },
        params: { page: 2 }
      }
    )
  })

  it('keeps ignored legacy query and header aliases out of the request cache key', async () => {
    requestMock.get.mockResolvedValue({ ok: true })
    const fetcher = new DataItemFetcher()

    await Promise.all([
      fetcher.fetchData(
        httpItem({
          url: '/api/device',
          headers: { 'X-Trace': 'trace-current' },
          params: [httpParam({ key: 'page', value: '2', dataType: 'number', paramType: 'query' })],
          parameters: [
            httpParam({ key: 'page', value: '99', dataType: 'number', paramType: 'query' }),
            httpParam({ key: 'X-Trace', value: 'trace-legacy-1', paramType: 'header' })
          ]
        })
      ),
      fetcher.fetchData(
        httpItem({
          url: '/api/device',
          headers: { 'X-Trace': 'trace-current' },
          params: [httpParam({ key: 'page', value: '2', dataType: 'number', paramType: 'query' })],
          parameters: [
            httpParam({ key: 'page', value: '100', dataType: 'number', paramType: 'query' }),
            httpParam({ key: 'X-Trace', value: 'trace-legacy-2', paramType: 'header' })
          ]
        })
      )
    ])

    expect(requestMock.get).toHaveBeenCalledTimes(1)
    expect(requestMock.get).toHaveBeenCalledWith('/api/device', {
      timeout: 10000,
      headers: { 'X-Trace': 'trace-current' },
      params: { page: 2 }
    })
  })

  it('resolves current-component bindings through the configuration bridge before sending HTTP requests', async () => {
    configurationBridgeMock.getConfiguration.mockReturnValue({
      base: { deviceId: 'bridge-device' },
      component: { title: 'RDI card' }
    })
    requestMock.get.mockResolvedValue({ ok: true })
    const fetcher = new DataItemFetcher()
    fetcher.setCurrentComponentId('card-current')

    await fetcher.fetchData(
      httpItem({
        url: '/api/devices/{device_id}/snapshot',
        pathParams: [
          httpParam({
            key: 'device_id',
            value: '__CURRENT_COMPONENT__.base.deviceId',
            valueMode: 'component',
            selectedTemplate: 'component-property-binding',
            isDynamic: true,
            paramType: 'path'
          })
        ]
      })
    )

    expect(configurationBridgeMock.getConfiguration).toHaveBeenCalledWith('card-current')
    expect(requestMock.get).toHaveBeenCalledWith('/api/devices/bridge-device/snapshot', { timeout: 10000 })
  })

  it('falls back from empty component customize values to base configuration values', async () => {
    configurationBridgeMock.getConfiguration.mockReturnValue({
      base: { metric: 'temperature' },
      component: { metric: null }
    })
    requestMock.get.mockResolvedValue({ ok: true })
    const fetcher = new DataItemFetcher()
    fetcher.setCurrentComponentId('card-current')

    await fetcher.fetchData(
      httpItem({
        params: [
          httpParam({
            key: 'metric',
            value: '__CURRENT_COMPONENT__.customize.metric',
            valueMode: 'component',
            selectedTemplate: 'component-property-binding',
            isDynamic: true
          })
        ]
      })
    )

    expect(requestMock.get).toHaveBeenCalledWith('/api/devices', {
      timeout: 10000,
      params: { metric: 'temperature' }
    })
  })

  it('returns invalid component binding defaults directly without numeric coercion', async () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    requestMock.get.mockResolvedValue({ ok: true })
    const fetcher = new DataItemFetcher()

    await fetcher.fetchData(
      httpItem({
        params: [
          httpParam({
            key: 'limit',
            value: 'invalid-binding',
            defaultValue: '',
            dataType: 'number',
            valueMode: 'component',
            selectedTemplate: 'component-property-binding',
            isDynamic: true
          })
        ]
      })
    )

    expect(requestMock.get).toHaveBeenCalledWith('/api/devices', {
      timeout: 10000,
      params: { limit: '' }
    })
    errorSpy.mockRestore()
  })

  it('recovers damaged component binding paths from variable names', async () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    configurationBridgeMock.getConfiguration.mockReturnValue({
      base: { deviceId: 'recovered-device' },
      component: {}
    })
    requestMock.get.mockResolvedValue({ ok: true })
    const fetcher = new DataItemFetcher()

    await fetcher.fetchData(
      httpItem({
        url: '/api/devices/{device_id}',
        pathParams: [
          httpParam({
            key: 'device_id',
            value: '123',
            variableName: 'chartA_deviceId',
            valueMode: 'component',
            selectedTemplate: 'component-property-binding',
            isDynamic: true,
            paramType: 'path'
          })
        ]
      })
    )

    expect(configurationBridgeMock.getConfiguration).toHaveBeenCalledWith('chartA')
    expect(requestMock.get).toHaveBeenCalledWith('/api/devices/recovered-device', { timeout: 10000 })
    expect(errorSpy).toHaveBeenCalledWith(
      expect.stringContaining('Damaged binding path detected'),
      expect.objectContaining({
        key: 'device_id',
        bindingPath: '123',
        variableName: 'chartA_deviceId'
      })
    )
    errorSpy.mockRestore()
  })

  it('falls back to the visual-editor store when bridge configuration is missing', async () => {
    editorStoreMock.nodes = [
      {
        id: 'card-store',
        properties: {
          customize: {
            metric: 'temperature'
          }
        }
      }
    ]
    requestMock.get.mockResolvedValue({ ok: true })
    const fetcher = new DataItemFetcher()

    await fetcher.fetchData(
      httpItem({
        params: [
          httpParam({
            key: 'metric',
            value: 'card-store.customize.metric',
            valueMode: 'component',
            selectedTemplate: 'component-property-binding',
            isDynamic: true
          })
        ]
      })
    )

    expect(requestMock.get).toHaveBeenCalledWith('/api/devices', {
      timeout: 10000,
      params: { metric: 'temperature' }
    })
  })

  it('deduplicates concurrent identical HTTP requests and clears the entry when the request settles', async () => {
    requestMock.get.mockResolvedValue({ data: 'same-response' })
    const fetcher = new DataItemFetcher()
    const item = httpItem({ url: '/api/slow', params: [httpParam({ key: 'k', value: 'v' })] })

    const [first, second] = await Promise.all([fetcher.fetchData(item), fetcher.fetchData(item)])

    expect(first).toEqual({ data: 'same-response' })
    expect(second).toEqual({ data: 'same-response' })
    expect(requestMock.get).toHaveBeenCalledTimes(1)

    await fetcher.fetchData(item)

    expect(requestMock.get).toHaveBeenCalledTimes(2)
  })

  it('deduplicates the raw HTTP request while applying each caller post-response script independently', async () => {
    requestMock.get.mockResolvedValue({ raw: 'shared-response' })
    scriptEngineMock.execute
      .mockResolvedValueOnce({ success: true, data: { view: 'temperature' } })
      .mockResolvedValueOnce({ success: true, data: { view: 'humidity' } })
    const fetcher = new DataItemFetcher()

    const [temperatureResult, humidityResult] = await Promise.all([
      fetcher.fetchData(
        httpItem({
          url: '/api/shared',
          postResponseScript: 'return { view: "temperature" }'
        })
      ),
      fetcher.fetchData(
        httpItem({
          url: '/api/shared',
          postResponseScript: 'return { view: "humidity" }'
        })
      )
    ])

    expect(requestMock.get).toHaveBeenCalledTimes(1)
    expect(scriptEngineMock.execute).toHaveBeenCalledTimes(2)
    expect(scriptEngineMock.execute).toHaveBeenNthCalledWith(1, 'return { view: "temperature" }', {
      response: { raw: 'shared-response' }
    })
    expect(scriptEngineMock.execute).toHaveBeenNthCalledWith(2, 'return { view: "humidity" }', {
      response: { raw: 'shared-response' }
    })
    expect(temperatureResult).toEqual({ view: 'temperature' })
    expect(humidityResult).toEqual({ view: 'humidity' })
  })

  it('deduplicates mixed current path params by the effective request instead of the compatibility mirror', async () => {
    requestMock.get.mockResolvedValue({ data: 'same-effective-request' })
    const fetcher = new DataItemFetcher()

    const currentPathParams = [httpParam({ key: 'device_id', value: 'dev-current', paramType: 'path' })]

    const [first, second] = await Promise.all([
      fetcher.fetchData(
        httpItem({
          url: '/api/devices/{device_id}',
          pathParams: currentPathParams,
          pathParameter: httpParam({ key: 'device_id', value: 'stale-mirror-a', paramType: 'path' })
        })
      ),
      fetcher.fetchData(
        httpItem({
          url: '/api/devices/{device_id}',
          pathParams: currentPathParams,
          pathParameter: httpParam({ key: 'device_id', value: 'stale-mirror-b', paramType: 'path' })
        })
      )
    ])

    expect(first).toEqual({ data: 'same-effective-request' })
    expect(second).toEqual({ data: 'same-effective-request' })
    expect(requestMock.get).toHaveBeenCalledTimes(1)
    expect(requestMock.get).toHaveBeenCalledWith('/api/devices/dev-current', { timeout: 10000 })
  })

  it('applies pre-request and post-response scripts around HTTP requests', async () => {
    scriptEngineMock.execute
      .mockResolvedValueOnce({
        success: true,
        data: { body: '{"patched":true}', headers: { 'X-From-Script': 'yes' } }
      })
      .mockResolvedValueOnce({ success: true, data: { transformed: true } })
    requestMock.post.mockResolvedValue({ raw: true })
    const fetcher = new DataItemFetcher()

    const result = await fetcher.fetchData(
      httpItem({
        url: '/api/scripted',
        method: 'POST',
        body: '{"patched":false}',
        preRequestScript: 'config.body = JSON.stringify({ patched: true })',
        postResponseScript: 'return { transformed: true }'
      })
    )

    expect(requestMock.post).toHaveBeenCalledWith(
      '/api/scripted',
      { patched: true },
      { timeout: 10000, headers: { 'X-From-Script': 'yes' } }
    )
    expect(result).toEqual({ transformed: true })
  })

  it('deduplicates HTTP requests by the config produced by the pre-request script', async () => {
    scriptEngineMock.execute
      .mockResolvedValueOnce({ success: true, data: { body: '{"tenant":"a"}' } })
      .mockResolvedValueOnce({ success: true, data: { body: '{"tenant":"b"}' } })
    requestMock.post.mockResolvedValue({ ok: true })
    const fetcher = new DataItemFetcher()
    const item = httpItem({
      url: '/api/scripted-cache',
      method: 'POST',
      body: '{"tenant":"initial"}',
      preRequestScript: 'config.body = JSON.stringify({ tenant })'
    })

    await Promise.all([fetcher.fetchData(item), fetcher.fetchData(item)])

    expect(requestMock.post).toHaveBeenCalledTimes(2)
    expect(requestMock.post).toHaveBeenNthCalledWith(1, '/api/scripted-cache', { tenant: 'a' }, { timeout: 10000 })
    expect(requestMock.post).toHaveBeenNthCalledWith(2, '/api/scripted-cache', { tenant: 'b' }, { timeout: 10000 })
  })

  it('preserves successful script data, including falsy values, and reports failures', async () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    scriptEngineMock.execute
      .mockResolvedValueOnce({ success: true, data: { computed: 42 } })
      .mockResolvedValueOnce({ success: true, data: 0 })
      .mockResolvedValueOnce({ success: true, data: false })
      .mockResolvedValueOnce({ success: true, data: '' })
      .mockResolvedValueOnce({ success: false, error: 'blocked' })
    const fetcher = new DataItemFetcher()

    await expect(fetcher.fetchData({ type: 'script', config: { script: 'return 42' } })).resolves.toEqual({
      computed: 42
    })
    await expect(fetcher.fetchData({ type: 'script', config: { script: 'return 0' } })).resolves.toBe(0)
    await expect(fetcher.fetchData({ type: 'script', config: { script: 'return false' } })).resolves.toBe(false)
    await expect(fetcher.fetchData({ type: 'script', config: { script: 'return ""' } })).resolves.toBe('')
    await expect(fetcher.fetchData({ type: 'script', config: { script: 'while(true){}' } })).resolves.toEqual({})
    expect(errorSpy).toHaveBeenCalledWith('[DataItemFetcher] Script data source failed:', {
      error: 'blocked'
    })
    errorSpy.mockRestore()
  })
})
