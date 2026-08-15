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
import {
  createAddNodeAction,
  createSetRuntimeDataAction,
  createUpdateConfigAction,
  resetDataFlowManager,
  useDataFlowManager
} from '../data-flow-manager'

const hoisted = vi.hoisted(() => ({
  clearComponentCache: vi.fn(),
  executeComponent: vi.fn(),
  loggerError: vi.fn()
}))

vi.mock('@/core/data-architecture/SimpleDataBridge', () => ({
  simpleDataBridge: {
    clearComponentCache: hoisted.clearComponentCache,
    executeComponent: hoisted.executeComponent
  }
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({
    error: hoisted.loggerError,
    debug: vi.fn(),
    warn: vi.fn(),
    info: vi.fn()
  })
}))

const node = (id = 'widget-1') => ({
  id,
  type: 'card2-line',
  componentType: 'card2-line',
  x: 0,
  y: 0,
  w: 2,
  h: 2,
  properties: { title: 'Initial' },
  metadata: {}
})

describe('DataFlowManager', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T13:00:00.000Z'))
    vi.clearAllMocks()
    localStorage.clear()
    setActivePinia(createPinia())
    resetConfigurationService()
    resetDataFlowManager()
    hoisted.executeComponent.mockResolvedValue({
      success: true,
      data: { value: 26 },
      timestamp: 1782546000000
    })
  })

  it('adds nodes, prevents duplicate ids, emits view updates, and reports validation errors', async () => {
    const manager = useDataFlowManager()
    const store = useUnifiedEditorStore()
    const updates: any[] = []
    const errors: any[] = []
    manager.onDataFlowUpdate(action => updates.push(action))
    manager.onError((action, error) => errors.push({ action, error }))

    await manager.handleUserAction(createAddNodeAction(node()))

    expect(store.nodes).toEqual([expect.objectContaining({ id: 'widget-1' })])
    expect(updates).toEqual([expect.objectContaining({ type: 'ADD_NODE' })])

    await expect(manager.handleUserAction(createAddNodeAction(node()))).rejects.toThrow('节点ID已存在: widget-1')

    expect(store.nodes).toHaveLength(1)
    expect(errors[0]).toMatchObject({
      action: expect.objectContaining({ type: 'ADD_NODE' }),
      error: expect.any(Error)
    })
  })

  it('updates node properties and mirrors them into component configuration without blocking node updates', async () => {
    const manager = useDataFlowManager()
    const store = useUnifiedEditorStore()
    const configService = useConfigurationService()

    store.addNode(node())
    await manager.handleUserAction({
      type: 'UPDATE_NODE',
      targetId: 'widget-1',
      data: {
        x: 42,
        properties: {
          title: 'Updated title',
          color: 'blue'
        }
      }
    })

    expect(store.nodes[0]).toMatchObject({
      x: 42,
      properties: {
        title: 'Updated title',
        color: 'blue'
      }
    })
    expect(configService.getConfigurationSection('widget-1', 'component')).toEqual({
      title: 'Updated title',
      color: 'blue'
    })
  })

  it('handles configuration updates, autosaves them, refreshes data source runtime data, and updates Card2 bindings once', async () => {
    const manager = useDataFlowManager()
    const store = useUnifiedEditorStore()
    const configService = useConfigurationService()

    store.addNode(node())
    store.registerCard2Component({
      type: 'card2-line',
      name: 'Line',
      description: '',
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
          name: 'Main',
          description: 'main source',
          supportedTypes: ['api'],
          required: true
        }
      ]
    } as any)

    await manager.handleUserAction(
      createUpdateConfigAction('widget-1', 'dataSource', {
        type: 'api',
        config: {
          url: 'https://api.example.test/telemetry',
          sourceId: 'main'
        },
        bindings: {
          main: {
            url: 'https://api.example.test/telemetry'
          }
        }
      })
    )

    await Promise.resolve()
    await Promise.resolve()

    expect(JSON.parse(localStorage.getItem('widget_config_widget-1') || '{}')).toMatchObject({
      dataSource: {
        type: 'api',
        config: {
          url: 'https://api.example.test/telemetry',
          sourceId: 'main'
        }
      }
    })
    expect(hoisted.executeComponent).toHaveBeenCalledWith({
      componentId: 'widget-1',
      dataSources: [
        {
          id: 'main',
          type: 'http',
          config: {
            url: 'https://api.example.test/telemetry',
            method: 'GET',
            headers: undefined,
            timeout: undefined,
            params: undefined,
            body: undefined
          },
          filterPath: undefined,
          processScript: undefined
        }
      ]
    })
    expect(configService.getRuntimeData('widget-1')).toEqual({ value: 26 })
    expect(store.dataBindings.get('widget-1')?.requirement.main.config).toEqual({
      url: 'https://api.example.test/telemetry'
    })
  })

  it('rolls runtime data back to its previous value when a configuration action fails validation', async () => {
    const manager = useDataFlowManager()
    const store = useUnifiedEditorStore()
    const configService = useConfigurationService()
    const errors: any[] = []
    manager.onError((action, error) => errors.push({ action, error }))

    store.addNode(node())
    configService.setRuntimeData('widget-1', { value: 'previous' })

    await expect(
      manager.handleUserAction({
        type: 'UPDATE_CONFIGURATION',
        targetId: 'widget-1',
        data: {
          section: 'not-a-section',
          config: {}
        }
      } as any)
    ).rejects.toThrow('无效的配置section: not-a-section')

    expect(configService.getRuntimeData('widget-1')).toEqual({ value: 'previous' })
    expect(errors[0]).toMatchObject({
      action: expect.objectContaining({ type: 'UPDATE_CONFIGURATION' }),
      error: expect.any(Error)
    })
  })

  it('processes batch actions with loading flags and supports runtime data action creators', async () => {
    const manager = useDataFlowManager()
    const store = useUnifiedEditorStore()
    const configService = useConfigurationService()

    await manager.handleBatchActions([
      createAddNodeAction(node('widget-a')),
      createAddNodeAction(node('widget-b')),
      {
        type: 'SELECT_NODES',
        data: ['widget-a', 'widget-b']
      },
      createSetRuntimeDataAction('widget-a', { value: 1 })
    ])

    expect(store.isLoading).toBe(false)
    expect(store.nodes.map(item => item.id)).toEqual(['widget-a', 'widget-b'])
    expect(store.selectedIds).toEqual(['widget-a', 'widget-b'])
    expect(configService.getRuntimeData('widget-a')).toEqual({ value: 1 })
  })

  it('allows custom side effects and isolates their failures from the main state update', async () => {
    const manager = useDataFlowManager()
    const store = useUnifiedEditorStore()

    manager.registerSideEffect({
      name: 'ExplodingAuditHook',
      condition: action => action.type === 'ADD_NODE',
      execute: () => {
        throw new Error('audit hook failed')
      }
    })

    await manager.handleUserAction(createAddNodeAction(node()))

    expect(store.nodes).toHaveLength(1)
    expect(hoisted.loggerError).toHaveBeenCalledWith(
      expect.stringContaining('ExplodingAuditHook'),
      expect.objectContaining({
        actionType: 'ADD_NODE',
        error: 'audit hook failed'
      })
    )
  })
})
