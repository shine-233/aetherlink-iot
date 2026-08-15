/**
 * 文件用途：验证 可视化编辑器状态测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useUnifiedEditorStore } from '../unified-editor'
import { resetConfigurationService, useConfigurationService } from '../configuration-service'

const hoisted = vi.hoisted(() => ({
  clearComponentCache: vi.fn(),
  executeComponent: vi.fn()
}))

vi.mock('@/core/data-architecture/SimpleDataBridge', () => ({
  simpleDataBridge: {
    clearComponentCache: hoisted.clearComponentCache,
    executeComponent: hoisted.executeComponent
  }
}))

const flushSideEffects = async () => {
  await Promise.resolve()
  await Promise.resolve()
}

describe('ConfigurationService', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T12:00:00.000Z'))
    vi.clearAllMocks()
    localStorage.clear()
    setActivePinia(createPinia())
    resetConfigurationService()
    hoisted.executeComponent.mockResolvedValue({
      success: true,
      data: { value: 26 },
      timestamp: 1782542400000
    })
  })

  it('initializes and merges widget configuration sections without losing existing data source config', () => {
    const service = useConfigurationService()

    service.initializeConfiguration('widget-1', {
      base: { title: 'Initial', opacity: 0.8 },
      component: { properties: { color: 'red' } },
      dataSource: {
        type: 'static',
        config: { data: { value: 1 } },
        bindings: { main: { rawData: '{"value":1}' } }
      },
      interaction: { click: { type: 'open' } },
      metadata: { owner: 'qa' }
    })

    const merged = service.initializeConfiguration('widget-1', {
      base: { visible: false },
      component: { style: { width: 320 } },
      interaction: { hover: { type: 'highlight' } },
      metadata: { version: '2.0.0' }
    })

    expect(merged).toMatchObject({
      base: { title: 'Initial', opacity: 0.8, visible: false },
      component: { properties: { color: 'red' }, style: { width: 320 } },
      dataSource: {
        type: 'static',
        config: { data: { value: 1 } },
        bindings: { main: { rawData: '{"value":1}' } }
      },
      interaction: {
        click: { type: 'open' },
        hover: { type: 'highlight' }
      }
    })
  })

  it('validates configuration and data source requirements before mutating store state', () => {
    const service = useConfigurationService()
    const store = useUnifiedEditorStore()

    expect(() => service.setConfiguration('widget-1', { base: { opacity: 2 } })).toThrow('透明度必须在0-1之间')
    expect(store.baseConfigs.has('widget-1')).toBe(false)

    expect(() =>
      service.setDataSourceConfig('widget-1', {
        type: 'api',
        config: {},
        bindings: {}
      })
    ).toThrow('API数据源必须提供URL')

    expect(() =>
      service.setDataSourceConfig('widget-1', {
        type: 'websocket',
        config: {},
        bindings: {}
      })
    ).toThrow('WebSocket数据源必须提供URL')

    expect(() =>
      service.setDataSourceConfig('widget-1', {
        type: 'device',
        config: {},
        bindings: {}
      })
    ).toThrow('设备数据源必须提供设备ID')

    expect(store.dataSourceConfigs.has('widget-1')).toBe(false)
  })

  it('emits configuration and runtime events while static data source side effects clear cache and set runtime data', async () => {
    const service = useConfigurationService()
    const events: any[] = []
    const unsubscribe = service.onConfigurationChange(event => events.push(event))

    service.setDataSourceConfig('widget-1', {
      type: 'static',
      config: { data: { value: 26, unit: 'C' } },
      bindings: {}
    })
    await flushSideEffects()

    expect(events).toEqual([
      expect.objectContaining({
        widgetId: 'widget-1',
        section: 'dataSource',
        newValue: {
          type: 'static',
          config: { data: { value: 26, unit: 'C' } },
          bindings: {}
        },
        timestamp: new Date('2026-06-27T12:00:00.000Z')
      })
    ])
    expect(hoisted.clearComponentCache).toHaveBeenCalledWith('widget-1')
    expect(service.getRuntimeData('widget-1')).toEqual({ value: 26, unit: 'C' })

    unsubscribe()
    service.updateConfigurationSection('widget-1', 'component', { properties: { color: 'blue' } })
    expect(events).toHaveLength(1)
  })

  it('refreshes runtime data from api, websocket, and multi-source configurations and records execution errors', async () => {
    const service = useConfigurationService()

    await service.refreshRuntimeData('widget-1', {
      type: 'api',
      config: {
        sourceId: 'telemetry',
        url: 'https://api.example.test/telemetry',
        method: 'POST',
        headers: { Authorization: 'Bearer token' },
        timeout: 3000,
        params: [{ key: 'deviceId', value: 'device-1' }],
        data: { limit: 10 },
        filterPath: '$.data',
        processScript: 'return data'
      },
      bindings: {}
    })

    expect(hoisted.executeComponent).toHaveBeenCalledWith({
      componentId: 'widget-1',
      dataSources: [
        {
          id: 'telemetry',
          type: 'http',
          config: {
            url: 'https://api.example.test/telemetry',
            method: 'POST',
            headers: { Authorization: 'Bearer token' },
            timeout: 3000,
            params: [{ key: 'deviceId', value: 'device-1' }],
            body: { limit: 10 }
          },
          filterPath: '$.data',
          processScript: 'return data'
        }
      ]
    })
    expect(service.getRuntimeData('widget-1')).toEqual({ value: 26 })

    hoisted.executeComponent.mockResolvedValueOnce({
      success: false,
      error: 'network timeout',
      timestamp: 1782542401000
    })
    await service.refreshRuntimeData('widget-1', {
      type: 'websocket',
      config: {
        sourceId: 'socket',
        url: 'wss://api.example.test/ws',
        filterPath: '$.message'
      },
      bindings: {}
    })

    expect(service.getRuntimeData('widget-1')).toEqual({
      __error: true,
      message: 'network timeout',
      timestamp: 1782542401000
    })

    await service.refreshRuntimeData('widget-2', {
      type: 'static',
      config: {},
      bindings: {},
      dataSources: [{ sourceId: 'main', dataItems: [] }]
    } as any)

    expect(hoisted.executeComponent).toHaveBeenLastCalledWith({
      componentId: 'widget-2',
      dataSources: [{ sourceId: 'main', dataItems: [] }]
    })
  })

  it('persists, migrates, loads, and rejects invalid saved configuration from localStorage', async () => {
    const service = useConfigurationService()

    service.initializeConfiguration('widget-1', {
      base: { title: 'Persisted' },
      component: { properties: { color: 'green' } }
    })
    await service.saveConfiguration('widget-1')
    expect(JSON.parse(localStorage.getItem('widget_config_widget-1') || '{}')).toMatchObject({
      base: { title: 'Persisted' },
      component: { properties: { color: 'green' } }
    })

    service.registerMigration({
      fromVersion: '1.0.0',
      toVersion: '2.0.0',
      migrate: config => ({
        ...config,
        base: { ...(config.base || {}), title: 'Migrated' },
        metadata: { ...(config.metadata || {}), version: '2.0.0' }
      })
    })
    localStorage.setItem(
      'widget_config_older-format',
      JSON.stringify({
        base: { title: 'Old' },
        metadata: { version: '1.0.0' }
      })
    )

    expect(await service.loadConfiguration('older-format')).toMatchObject({
      base: { title: 'Migrated' },
      metadata: { version: '2.0.0' }
    })

    localStorage.setItem('widget_config_bad', JSON.stringify({ base: { opacity: -1 } }))
    expect(await service.loadConfiguration('bad')).toBeNull()

    localStorage.setItem('widget_config_broken', '{bad json')
    expect(await service.loadConfiguration('broken')).toBeNull()
  })
})
