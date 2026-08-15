/**
 * 文件用途: Visual Editor Bridge 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  disposeVisualEditorBridge,
  getVisualEditorBridge,
  VisualEditorBridge
} from './VisualEditorBridge'

const { simpleDataBridgeMock, bindingConfigMock, loggerMock } = vi.hoisted(() => ({
  simpleDataBridgeMock: {
    executeComponent: vi.fn(),
    getComponentData: vi.fn(),
    clearComponentCache: vi.fn()
  },
  bindingConfigMock: {
    buildAutoBindParams: vi.fn()
  },
  loggerMock: {
    debug: vi.fn(),
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn()
  }
}))

vi.mock('@/core/data-architecture/SimpleDataBridge', () => ({
  simpleDataBridge: simpleDataBridgeMock
}))

vi.mock('@/core/data-architecture/DataSourceBindingConfig', () => ({
  dataSourceBindingConfig: bindingConfigMock
}))

vi.mock('@/components/visual-editor/store/editor', () => ({
  useEditorStore: vi.fn()
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => loggerMock
}))

describe('VisualEditorBridge', () => {
  beforeEach(() => {
    disposeVisualEditorBridge()
    vi.clearAllMocks()
    simpleDataBridgeMock.executeComponent.mockResolvedValue({ success: true, data: { main: { value: 26 } } })
    bindingConfigMock.buildAutoBindParams.mockReturnValue({ device_id: 'auto-device' })
  })

  it('converts standard data-source configuration and notifies subscribers', async () => {
    const bridge = new VisualEditorBridge()
    const updateCallback = vi.fn()
    bridge.onDataUpdate(updateCallback)

    const result = await bridge.updateComponentExecutor('card-a', 'rdi-card', {
      base: { deviceId: 'device-a', metricsList: ['temperature'] },
      dataSource: {
        dataSources: [
          {
            sourceId: 'main',
            dataItems: [
              {
                item: { type: 'json', config: { jsonString: '{"temperature":26}' } },
                processing: { filterPath: '$.temperature', customScript: 'return data' }
              },
              {
                item: { type: 'script', config: { script: 'return { ok: true }' } },
                processing: {}
              }
            ],
            mergeStrategy: { type: 'array' }
          }
        ]
      }
    })

    expect(result).toEqual({ success: true, data: { main: { value: 26 } } })
    expect(simpleDataBridgeMock.executeComponent).toHaveBeenCalledWith({
      componentId: 'card-a',
      componentType: 'rdi-card',
      enabled: true,
      dataSources: [
        {
          sourceId: 'main',
          dataItems: [
            {
              item: {
                type: 'json',
                config: { jsonString: '{"temperature":26}', jsonContent: '{"temperature":26}' }
              },
              processing: { filterPath: '$.temperature', customScript: 'return data', defaultValue: {} }
            },
            {
              item: {
                type: 'script',
                config: { script: 'return { ok: true }', scriptContent: 'return { ok: true }' }
              },
              processing: { filterPath: '$', customScript: undefined, defaultValue: {} }
            }
          ],
          mergeStrategy: { type: 'array' }
        }
      ]
    })
    expect(updateCallback).toHaveBeenCalledWith('card-a', { main: { value: 26 } })
  })

  it('normalizes rawDataList aliases and skips disabled or unsupported sources', async () => {
    const bridge = new VisualEditorBridge()

    await bridge.updateComponentExecutor('card-raw', 'widget', {
      rawDataList: [
        { type: 'http', config: { url: '/api/live' }, filterPath: '$.data', processScript: 'return data' },
        { id: 'legacy-api', type: 'api', config: { url: '/api/legacy' } },
        { id: 'external', type: 'mqtt', config: { topic: 'telemetry' } },
        { type: 'json', enabled: false, config: { jsonString: '{}' } }
      ]
    })

    expect(simpleDataBridgeMock.executeComponent).toHaveBeenCalledWith({
      componentId: 'card-raw',
      componentType: 'widget',
      enabled: true,
      dataSources: [
        {
          id: 'dataSource1',
          type: 'http',
          config: { url: '/api/live' },
          filterPath: '$.data',
          processScript: 'return data'
        },
        {
          id: 'legacy-api',
          type: 'http',
          config: { url: '/api/legacy' },
          filterPath: undefined,
          processScript: undefined
        }
      ]
    })
    expect(loggerMock.error).toHaveBeenCalledWith('[VisualEditorBridge] UNSUPPORTED_DATA_SOURCE_TYPE', {
      sourceId: 'external',
      type: 'mqtt'
    })
  })

  it('injects base configuration into older-format dataSourceN entries', async () => {
    const bridge = new VisualEditorBridge()

    await bridge.updateComponentExecutor('card-classic', 'rdi-card', {
      base: { deviceId: 'base-device', metricsList: ['temp', 'hum'] },
      dataSource: {
        dataSource1: {
          type: 'api',
          config: {
            url: '/api/telemetry',
            params: { metric: 'temperature' }
          },
          filterPath: '$.data'
        },
        dataSource2: {
          type: 'json',
          enabled: false,
          config: { jsonString: '{}' }
        }
      }
    })

    expect(simpleDataBridgeMock.executeComponent).toHaveBeenCalledWith({
      componentId: 'card-classic',
      componentType: 'rdi-card',
      enabled: true,
      dataSources: [
        {
          id: 'dataSource1',
          type: 'http',
          config: {
            url: '/api/telemetry',
            params: { metric: 'temperature' },
            deviceId: 'base-device',
            metricsList: ['temp', 'hum']
          },
          filterPath: '$.data',
          processScript: undefined
        }
      ]
    })
  })

  it('logs unsupported property reads instead of silently resolving binding placeholders to undefined', async () => {
    const bridge = new VisualEditorBridge()

    await bridge.updateComponentExecutor('card-bindings', 'rdi-card', {
      base: {},
      dataSource: {
        dataSource1: {
          type: 'http',
          config: {
            url: '/api/telemetry',
            params: {
              device_id: 'card-bindings.base.deviceId',
              metric: 'card-bindings.component.metric'
            }
          }
        }
      }
    })

    expect(loggerMock.error).toHaveBeenCalledWith(
      '[VisualEditorBridge] Unsupported property binding read:',
      expect.objectContaining({
        scope: 'base',
        componentId: 'card-bindings',
        propertyName: 'deviceId'
      })
    )
    expect(loggerMock.error).toHaveBeenCalledWith(
      '[VisualEditorBridge] Unsupported property binding read:',
      expect.objectContaining({
        scope: 'component',
        componentId: 'card-bindings',
        propertyName: 'metric'
      })
    )
    expect(simpleDataBridgeMock.executeComponent).toHaveBeenCalledWith({
      componentId: 'card-bindings',
      componentType: 'rdi-card',
      enabled: true,
      dataSources: [
        {
          id: 'dataSource1',
          type: 'http',
          config: {
            url: '/api/telemetry',
            params: {
              device_id: 'card-bindings.base.deviceId',
              metric: 'card-bindings.component.metric'
            }
          },
          filterPath: undefined,
          processScript: undefined
        }
      ]
    })
  })

  it('keeps compatibility whitelist bindings as persisted aliases to component bindings', async () => {
    const bridge = new VisualEditorBridge()

    await bridge.updateComponentExecutor('card-whitelist', 'rdi-card', {
      base: {},
      dataSource: {
        dataSource1: {
          type: 'http',
          config: {
            url: '/api/telemetry',
            params: {
              metric: 'card-whitelist.whitelist.metric'
            }
          }
        }
      }
    })

    expect(loggerMock.error).toHaveBeenCalledWith(
      '[VisualEditorBridge] Unsupported property binding read:',
      expect.objectContaining({
        scope: 'component',
        componentId: 'card-whitelist',
        propertyName: 'metric'
      })
    )
    expect(simpleDataBridgeMock.executeComponent).toHaveBeenCalledWith({
      componentId: 'card-whitelist',
      componentType: 'rdi-card',
      enabled: true,
      dataSources: [
        {
          id: 'dataSource1',
          type: 'http',
          config: {
            url: '/api/telemetry',
            params: {
              metric: 'card-whitelist.whitelist.metric'
            }
          },
          filterPath: undefined,
          processScript: undefined
        }
      ]
    })
  })

  it('applies auto-bind parameters to single HTTP data sources', async () => {
    const bridge = new VisualEditorBridge()

    await bridge.updateComponentExecutor('card-auto', 'rdi-card', {
      base: { deviceId: 'base-device' },
      dataSource: {
        type: 'http',
        autoBind: { enabled: true, mode: 'strict' },
        config: {
          url: '/api/devices/{device_id}',
          params: { existing: 'keep' }
        }
      }
    })

    expect(bindingConfigMock.buildAutoBindParams).toHaveBeenCalledWith(
      expect.objectContaining({
        base: { deviceId: 'base-device' },
        componentType: 'widget'
      }),
      { enabled: true, mode: 'strict' },
      undefined
    )
    expect(simpleDataBridgeMock.executeComponent).toHaveBeenCalledWith({
      componentId: 'card-auto',
      componentType: 'rdi-card',
      enabled: true,
      dataSources: [
        {
          id: 'dataSource1',
          type: 'http',
          config: {
            url: '/api/devices/{device_id}',
            params: { existing: 'keep', device_id: 'auto-device' }
          },
          filterPath: undefined,
          processScript: undefined
        }
      ]
    })
  })

  it('wraps data-source-bindings entries into separate executable sources', async () => {
    const bridge = new VisualEditorBridge()

    await bridge.updateComponentExecutor('card-bindings', 'widget', {
      type: 'data-source-bindings',
      dataSource1: { deviceId: 'dev-a' },
      dataSource2: { deviceId: 'dev-b' }
    })

    expect(simpleDataBridgeMock.executeComponent).toHaveBeenCalledWith({
      componentId: 'card-bindings',
      componentType: 'widget',
      enabled: true,
      dataSources: [
        {
          id: 'dataSource1',
          type: 'data-source-bindings',
          config: { dataSourceBindings: { dataSource1: { deviceId: 'dev-a' } } },
          filterPath: undefined,
          processScript: undefined
        },
        {
          id: 'dataSource2',
          type: 'data-source-bindings',
          config: { dataSourceBindings: { dataSource2: { deviceId: 'dev-b' } } },
          filterPath: undefined,
          processScript: undefined
        }
      ]
    })
  })

  it('allows data-update subscriptions to unsubscribe and isolates failing callbacks', async () => {
    const bridge = new VisualEditorBridge()
    const failingCallback = vi.fn(() => {
      throw new Error('subscriber failed')
    })
    const activeCallback = vi.fn()
    const removedCallback = vi.fn()
    const unsubscribe = bridge.onDataUpdate(removedCallback)

    bridge.onDataUpdate(failingCallback)
    bridge.onDataUpdate(activeCallback)
    unsubscribe()
    unsubscribe()

    await bridge.updateComponentExecutor('card-callbacks', 'widget', { type: 'json', config: { jsonString: '{}' } })

    expect(removedCallback).toHaveBeenCalledTimes(0)
    expect(failingCallback).toHaveBeenCalledWith('card-callbacks', { main: { value: 26 } })
    expect(activeCallback).toHaveBeenCalledWith('card-callbacks', { main: { value: 26 } })

    bridge.dispose()
    await bridge.updateComponentExecutor('card-after-dispose', 'widget', { type: 'json', config: { jsonString: '{}' } })

    expect(failingCallback).toHaveBeenCalledTimes(1)
    expect(activeCallback).toHaveBeenCalledTimes(1)
  })

  it('reuses and explicitly releases the current port bridge instance', async () => {
    const firstBridge = getVisualEditorBridge()
    const callback = vi.fn()
    firstBridge.onDataUpdate(callback)

    expect(getVisualEditorBridge()).toBe(firstBridge)

    disposeVisualEditorBridge()
    disposeVisualEditorBridge()

    const replacementBridge = getVisualEditorBridge()
    expect(replacementBridge).not.toBe(firstBridge)

    await firstBridge.updateComponentExecutor('card-old-instance', 'widget', {
      type: 'json',
      config: { jsonString: '{}' }
    })
    expect(callback).not.toHaveBeenCalled()

    const replacementCallback = vi.fn()
    replacementBridge.onDataUpdate(replacementCallback)
    await replacementBridge.updateComponentExecutor('card-new-instance', 'widget', {
      type: 'json',
      config: { jsonString: '{}' }
    })
    expect(replacementCallback).toHaveBeenCalledWith('card-new-instance', { main: { value: 26 } })
  })

  it('proxies component data and cache clearing through SimpleDataBridge', () => {
    simpleDataBridgeMock.getComponentData.mockReturnValue({ main: { value: 26 } })
    const bridge = new VisualEditorBridge()

    expect(bridge.getComponentData('card-cache')).toEqual({ main: { value: 26 } })
    bridge.clearComponentCache('card-cache')

    expect(simpleDataBridgeMock.getComponentData).toHaveBeenCalledWith('card-cache')
    expect(simpleDataBridgeMock.clearComponentCache).toHaveBeenCalledWith('card-cache')
  })
})
