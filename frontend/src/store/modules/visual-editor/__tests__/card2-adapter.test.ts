/**
 * 文件用途：验证 可视化编辑器状态测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useUnifiedEditorStore } from '../unified-editor'
import { resetConfigurationService } from '../configuration-service'
import { resetDataFlowManager } from '../data-flow-manager'
import { resetCard2Adapter, useCard2Adapter, type ComponentDefinition } from '../card2-adapter'

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

const flushAsyncInit = async () => {
  await (vi as any).dynamicImportSettled?.()
  await Promise.resolve()
  await Promise.resolve()
}

const definition = (overrides: Partial<ComponentDefinition> = {}): ComponentDefinition =>
  ({
    type: 'card2-line',
    name: 'Line Card',
    description: 'Line telemetry card',
    version: '1.0.0',
    component: { name: 'LineCard' },
    category: 'chart',
    mainCategory: 'chart',
    subCategory: 'line',
    icon: 'line',
    author: 'AetherLink',
    permission: 'public',
    tags: ['telemetry'],
    config: {
      width: 360,
      height: 240,
      style: { color: 'red' },
      properties: { title: 'Telemetry' }
    },
    dataSources: [
      {
        key: 'main',
        name: 'Main Source',
        description: 'main telemetry',
        supportedTypes: [],
        required: true,
        fieldMappings: {
          value: {
            targetField: 'value',
            type: 'number',
            required: true,
            description: 'metric value',
            defaultValue: 0
          }
        }
      },
      {
        key: 'status',
        name: 'Status Source',
        description: 'device status',
        supportedTypes: ['api'],
        required: false
      }
    ],
    ...overrides
  }) as ComponentDefinition

const node = (overrides: Record<string, any> = {}) => ({
  id: 'widget-1',
  type: 'card2-line',
  componentType: 'card2-line',
  x: 0,
  y: 0,
  w: 3,
  h: 2,
  metadata: {},
  ...overrides
})

describe('Card2VisualEditorAdapter', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T14:00:00.000Z'))
    vi.clearAllMocks()
    setActivePinia(createPinia())
    resetConfigurationService()
    resetDataFlowManager()
    resetCard2Adapter()
    hoisted.executeComponent.mockResolvedValue({ success: true, data: { value: 26 } })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('registers Card2 definitions as visual editor widgets with normalized layout, properties, and data sources', async () => {
    const adapter = useCard2Adapter()
    const store = useUnifiedEditorStore()

    adapter.registerCard2Component(definition())
    await flushAsyncInit()

    expect(store.widgets.get('card2-line')).toMatchObject({
      type: 'card2-line',
      name: 'Line Card',
      defaultLayout: {
        canvas: { width: 360, height: 240, x: 0, y: 0 },
        grid: { w: 3, h: 2, x: 0, y: 0, minW: 1, minH: 1 },
        gridstack: { w: 3, h: 2, x: 0, y: 0, minW: 1, minH: 1 },
        gridLayoutPlus: { w: 4, h: 3, x: 0, y: 0 }
      },
      defaultProperties: {
        properties: { title: 'Telemetry' },
        style: { width: 360, height: 240, color: 'red' },
        events: {}
      },
      dataSources: [
        expect.objectContaining({
          key: 'main',
          supportedTypes: ['static'],
          fieldMappings: expect.objectContaining({
            value: expect.objectContaining({ defaultValue: 0 })
          })
        }),
        expect.objectContaining({
          key: 'status',
          supportedTypes: ['api'],
          fieldMappings: {}
        })
      ],
      metadata: expect.objectContaining({
        source: 'card2',
        isCard2Component: true,
        hasDataSources: true
      })
    })
    expect(store.card2Components.get('card2-line')).toMatchObject({ type: 'card2-line' })
    expect(adapter.getCard2ComponentCount()).toBe(1)
  })

  it('initializes default component and data source configuration when a Card2 component is added to the canvas', () => {
    const adapter = useCard2Adapter()
    const store = useUnifiedEditorStore()

    adapter.registerCard2Component(definition())
    adapter.onComponentAdded('widget-1', 'card2-line')

    expect(store.componentConfigs.get('widget-1')).toEqual({
      properties: { title: 'Telemetry' },
      style: { width: 360, height: 240, color: 'red' },
      events: {}
    })
    expect(store.dataSourceConfigs.get('widget-1')).toEqual({
      type: 'static',
      config: {},
      bindings: {
        main: { rawData: '0' }
      }
    })
  })

  it('creates, updates, and destroys reactive data bindings from node type and metadata definitions', async () => {
    const adapter = useCard2Adapter()
    const store = useUnifiedEditorStore()

    store.addNode(node())
    store.registerCard2Component(definition())
    await flushAsyncInit()

    const binding = await adapter.createDataBinding('widget-1', {
      type: 'api',
      config: {},
      bindings: {
        main: { url: '/api/main' },
        status: { url: '/api/status' }
      }
    })

    expect(binding).toMatchObject({
      id: 'widget-1_binding',
      componentId: 'widget-1',
      isActive: true,
      requirement: {
        main: {
          type: 'object',
          required: true,
          description: 'main telemetry',
          mapping: {
            value: expect.objectContaining({ targetField: 'value' })
          },
          defaultValue: 0,
          config: { url: '/api/main' }
        },
        status: {
          type: 'object',
          required: false,
          description: 'device status',
          mapping: {},
          defaultValue: null,
          config: { url: '/api/status' }
        }
      }
    })
    expect(store.dataBindings.get('widget-1')).toEqual(binding)

    await adapter.updateDataBinding('widget-1', {
      type: 'api',
      config: {},
      bindings: {
        status: { url: '/api/status-v2' }
      }
    })
    expect(store.dataBindings.get('widget-1')?.requirement).toEqual({
      status: expect.objectContaining({
        config: { url: '/api/status-v2' }
      })
    })

    adapter.onComponentRemoved('widget-1')
    expect(store.dataBindings.has('widget-1')).toBe(false)

    store.addNode(
      node({
        id: 'metadata-widget',
        type: 'unknown-card',
        componentType: 'unknown-card',
        metadata: {
          card2Definition: definition({ type: 'metadata-card' })
        }
      })
    )
    const metadataBinding = await adapter.createDataBinding('metadata-widget', {
      type: 'static',
      config: {},
      bindings: {
        main: { rawData: '{"value":88}' }
      }
    })

    expect(metadataBinding?.requirement.main.config).toEqual({ rawData: '{"value":88}' })
  })

  it('returns null when Card2 system or component definitions are unavailable and delegates runtime data updates', async () => {
    const adapter = useCard2Adapter()
    const store = useUnifiedEditorStore()

    expect(await adapter.createDataBinding('missing-widget', { type: 'static', config: {}, bindings: {} })).toBeNull()

    adapter.handleRuntimeDataUpdate('widget-runtime', { value: 55 })
    await flushAsyncInit()

    expect(store.getRuntimeData('widget-runtime')).toEqual({ value: 55 })
  })

  it('keeps the retired Card2 runtime lookup safe while returning null for unavailable components', async () => {
    const adapter = useCard2Adapter()
    await flushAsyncInit()

    expect(await adapter.getComponent('card2-line')).toBeNull()
    expect(adapter.getComponentDefinition('card2-line')).toBeNull()
    expect(await adapter.getComponent('missing-card')).toBeNull()
    expect(adapter.getComponentDefinition('missing-card')).toBeNull()
  })
})
