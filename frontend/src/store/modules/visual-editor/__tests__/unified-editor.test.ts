/**
 * 文件用途：验证 可视化编辑器状态测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useUnifiedEditorStore, type ComponentDefinition } from '../unified-editor'

const node = (overrides: Record<string, any> = {}) => ({
  id: 'widget-1',
  type: 'card2-line',
  componentType: 'card2-line',
  x: 10,
  y: 20,
  w: 3,
  h: 2,
  properties: { title: 'Line' },
  metadata: {},
  ...overrides
})

const card2Definition = (overrides: Partial<ComponentDefinition> = {}): ComponentDefinition =>
  ({
    type: 'card2-line',
    name: 'Line Card',
    description: 'line chart',
    version: '1.0.0',
    component: {},
    category: 'chart',
    mainCategory: 'chart',
    subCategory: 'line',
    icon: 'line',
    author: 'AetherLink',
    permission: 'public',
    dataSources: [
      {
        key: 'main',
        name: 'Main telemetry',
        description: 'main source',
        supportedTypes: ['static', 'api'],
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
      }
    ],
    ...overrides
  }) as ComponentDefinition

describe('unified-editor store', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T11:00:00.000Z'))
    setActivePinia(createPinia())
  })

  it('keeps nodes, selected ids, configuration maps, and dirty state synchronized through node lifecycle', () => {
    const store = useUnifiedEditorStore()

    store.addNode(node())
    store.selectNodes(['widget-1'])
    store.setBaseConfiguration('widget-1', { title: 'Telemetry', opacity: 0.8 })
    store.setComponentConfiguration('widget-1', { properties: { color: 'red' }, style: { width: 320 } })
    store.setDataSourceConfiguration('widget-1', {
      type: 'static',
      config: { data: { value: 26 } },
      bindings: {}
    })
    store.setInteractionConfiguration('widget-1', { click: { type: 'navigate' }, custom: {} })
    store.setRuntimeData('widget-1', { value: 26 })

    expect(store.nodes).toHaveLength(1)
    expect(store.selectedNodes).toEqual([expect.objectContaining({ id: 'widget-1' })])
    expect(store.getFullConfiguration('widget-1')).toMatchObject({
      base: { title: 'Telemetry', opacity: 0.8 },
      component: { properties: { color: 'red' }, style: { width: 320 } },
      dataSource: { type: 'static', config: { data: { value: 26 } } },
      interaction: { click: { type: 'navigate' } },
      metadata: {
        id: 'widget-1',
        hasRuntimeData: true,
        configurationSections: {
          base: true,
          component: true,
          dataSource: true,
          interaction: true
        }
      }
    })
    expect(store.hasUnsavedChanges).toBe(true)

    store.markSaved()
    expect(store.hasUnsavedChanges).toBe(false)
    expect(store.lastSaved).toEqual(new Date('2026-06-27T11:00:00.000Z'))

    store.updateNode('widget-1', { x: 42, properties: { title: 'Updated' } } as any)
    expect(store.nodes[0]).toMatchObject({ x: 42, properties: { title: 'Updated' } })
    expect(store.hasUnsavedChanges).toBe(true)

    store.removeNode('widget-1')
    expect(store.nodes).toEqual([])
    expect(store.selectedIds).toEqual([])
    expect(store.baseConfigs.has('widget-1')).toBe(false)
    expect(store.componentConfigs.has('widget-1')).toBe(false)
    expect(store.dataSourceConfigs.has('widget-1')).toBe(false)
    expect(store.interactionConfigs.has('widget-1')).toBe(false)
    expect(store.runtimeData.has('widget-1')).toBe(false)
  })

  it('creates Card2 data bindings from registered definitions, node metadata, declared sources, and extra bindings', () => {
    const store = useUnifiedEditorStore()
    store.addNode(node())
    store.registerCard2Component(card2Definition())

    store.setDataSourceConfiguration('widget-1', {
      type: 'static',
      config: {},
      bindings: {
        main: { rawData: '{"value":26}' },
        extra: { rawData: '{"status":"online"}' }
      }
    })

    expect(store.dataBindings.get('widget-1')).toMatchObject({
      id: 'widget-1_binding',
      componentId: 'widget-1',
      isActive: true,
      requirement: {
        main: {
          type: 'object',
          required: true,
          description: 'main source',
          mapping: {
            value: expect.objectContaining({ targetField: 'value' })
          },
          config: { rawData: '{"value":26}' }
        },
        extra: {
          type: 'object',
          required: false,
          description: 'extra',
          mapping: {},
          config: { rawData: '{"status":"online"}' }
        }
      }
    })

    store.removeNode('widget-1')
    store.addNode(
      node({
        id: 'widget-from-metadata',
        type: 'unregistered',
        componentType: 'unregistered',
        metadata: {
          card2Definition: card2Definition({ type: 'metadata-card' })
        }
      })
    )
    store.setDataSourceConfiguration('widget-from-metadata', {
      type: 'static',
      config: {},
      bindings: {
        main: { rawData: '{"value":99}' }
      }
    })

    expect(store.dataBindings.get('widget-from-metadata')?.requirement.main.config).toEqual({
      rawData: '{"value":99}'
    })
  })

  it('registers widgets, updates viewport and mode, and clears all editor state back to defaults', () => {
    const store = useUnifiedEditorStore()

    store.registerWidgets([
      { type: 'card-a', name: 'A' } as any,
      { type: 'card-b', name: 'B' } as any
    ])
    store.updateViewport({ x: 100, zoom: 1.5 })
    store.setMode('preview')
    store.addNode(node())
    store.setRuntimeData('widget-1', { value: 1 })

    expect(store.allWidgets.map(widget => widget.type)).toEqual(['card-a', 'card-b'])
    expect(store.viewport).toEqual({ x: 100, y: 0, zoom: 1.5 })
    expect(store.mode).toBe('preview')

    store.resetViewport()
    expect(store.viewport).toEqual({ x: 0, y: 0, zoom: 1 })

    store.clearAll()
    expect(store.nodes).toEqual([])
    expect(store.allWidgets).toEqual([])
    expect(store.runtimeData.size).toBe(0)
    expect(store.mode).toBe('design')
    expect(store.hasUnsavedChanges).toBe(false)
    expect(store.lastSaved).toBeNull()
  })
})
