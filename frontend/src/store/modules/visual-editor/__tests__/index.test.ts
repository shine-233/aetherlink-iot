/**
 * 文件用途：验证 可视化编辑器状态测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  MigrationHelper,
  resetCard2Adapter,
  resetConfigurationService,
  resetDataFlowManager,
  resetUnifiedVisualEditorSystem,
  useUnifiedVisualEditorSystem
} from '..'

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

const oldStorePayload = {
  nodes: [
    {
      id: 'old-widget-1',
      type: 'card2-line',
      componentType: 'card2-line',
      x: 10,
      y: 20,
      w: 3,
      h: 2,
      properties: { title: 'Old Line' },
      metadata: {}
    }
  ],
  selectedIds: ['old-widget-1'],
  configurations: {
    'old-widget-1': {
      base: { title: 'Migrated chart', opacity: 0.9 },
      component: { properties: { color: 'green' }, style: { width: 320 } },
      dataSource: {
        type: 'static',
        config: { data: { value: 26 } },
        bindings: {}
      },
      interaction: { click: { type: 'open' }, custom: {} },
      metadata: { source: 'old_editor_data' }
    }
  }
}

describe('visual-editor migration entrypoint', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    resetUnifiedVisualEditorSystem()
    resetConfigurationService()
    resetDataFlowManager()
    resetCard2Adapter()
    setActivePinia(createPinia())
    hoisted.executeComponent.mockResolvedValue({
      success: true,
      data: { value: 26 },
      timestamp: 1782542400000
    })
  })

  it('keeps old_editor_data as a manual persisted migration contract', async () => {
    expect(MigrationHelper.needsMigration()).toBe(false)

    localStorage.setItem('old_editor_data', JSON.stringify(oldStorePayload))
    expect(MigrationHelper.needsMigration()).toBe(true)

    const system = useUnifiedVisualEditorSystem()
    await system.initialize()

    MigrationHelper.migrateFromOldStore(JSON.parse(localStorage.getItem('old_editor_data') || '{}'))

    expect(system.store?.nodes).toEqual([expect.objectContaining({ id: 'old-widget-1' })])
    expect(system.store?.selectedIds).toEqual(['old-widget-1'])
    expect(system.configService?.getConfiguration('old-widget-1')).toMatchObject({
      base: { title: 'Migrated chart', opacity: 0.9 },
      component: { properties: { color: 'green' }, style: { width: 320 } },
      dataSource: { type: 'static', config: { data: { value: 26 } } },
      interaction: { click: { type: 'open' } }
    })
  })
})
