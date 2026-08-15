/**
 * 文件用途: Simple Data Bridge 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { describe, expect, it, vi } from 'vitest'

import { SimpleDataBridge, type ComponentDataRequirement } from './SimpleDataBridge'
import type { DataSourceConfiguration, ExecutionResult } from './executors/MultiLayerExecutorChain'

const createWarehouse = () => {
  const completeData = new Map<string, Record<string, any>>()

  return {
    getComponentData: vi.fn((componentId: string) => completeData.get(componentId) ?? null),
    storeComponentData: vi.fn((componentId: string, sourceId: string, data: Record<string, any>) => {
      if (sourceId === 'complete') completeData.set(componentId, data)
    }),
    clearComponentCache: vi.fn((componentId: string) => completeData.delete(componentId)),
    clearAllCache: vi.fn(() => completeData.clear()),
    destroy: vi.fn(),
    setCacheExpiry: vi.fn(),
    getPerformanceMetrics: vi.fn(() => ({})),
    getStorageStats: vi.fn(() => ({
      totalComponents: completeData.size,
      totalDataSources: 0,
      memoryUsageMB: 0
    }))
  } as any
}

const createSuccessfulExecutor = (componentData: Record<string, any>) => ({
  executeDataProcessingChain: vi.fn(
    async (_config: DataSourceConfiguration): Promise<ExecutionResult> => ({
      success: true,
      componentData,
      executionTime: 1,
      timestamp: 1000
    })
  )
})

describe('SimpleDataBridge', () => {
  it('uses an injected snapshot reader before executing the data source chain', async () => {
    const warehouse = createWarehouse()
    const executorChain = createSuccessfulExecutor({
      'snapshot-source': {
        data: { value: 42 },
        type: 'json',
        lastUpdated: 1000
      }
    })
    const snapshotReader = vi.fn(() => ({
      dataSource: {
        dataSources: [
          {
            sourceId: 'snapshot-source',
            dataItems: [
              {
                item: { type: 'json', config: { jsonContent: '{"value":42}' } },
                processing: { filterPath: '$', defaultValue: {} }
              }
            ],
            mergeStrategy: 'object'
          }
        ]
      }
    }))
    const bridge = new SimpleDataBridge({
      warehouse,
      executorChain,
      snapshotReader,
      now: () => 1000,
      random: () => 0.5
    })

    const result = await bridge.executeComponent({
      componentId: 'component-1',
      dataSources: [{ id: 'stale-source', type: 'json', config: { data: { stale: true } } }]
    })

    expect(snapshotReader).toHaveBeenCalledWith('component-1')
    expect(executorChain.executeDataProcessingChain).toHaveBeenCalledTimes(1)
    expect(executorChain.executeDataProcessingChain.mock.calls[0][0]).toMatchObject({
      componentId: 'component-1',
      dataSources: [{ sourceId: 'snapshot-source' }],
      configHash: expect.any(String)
    })
    expect(result).toEqual({
      success: true,
      data: { 'snapshot-source': { value: 42 } },
      timestamp: 1000
    })
    expect(warehouse.storeComponentData).toHaveBeenCalledWith(
      'component-1',
      'complete',
      { 'snapshot-source': { value: 42 } },
      'multi-source'
    )
  })

  it('logs snapshot reader failures and falls back to the supplied requirement', async () => {
    const warehouse = createWarehouse()
    const executorChain = createSuccessfulExecutor({ 'fallback-source': { ok: true } })
    const logError = vi.fn()
    const requirement: ComponentDataRequirement = {
      componentId: 'component-2',
      dataSources: [{ id: 'fallback-source', type: 'json', config: { data: { ok: true } } }]
    }
    const bridge = new SimpleDataBridge({
      warehouse,
      executorChain,
      snapshotReader: vi.fn(() => {
        throw 'snapshot unavailable'
      }),
      logError,
      now: () => 2000,
      random: () => 0.25
    })

    const result = await bridge.executeComponent(requirement)

    expect(logError).toHaveBeenCalledWith(
      expect.stringContaining('[SimpleDataBridge] [component-2-2000-9]'),
      'snapshot unavailable'
    )
    expect(executorChain.executeDataProcessingChain.mock.calls[0][0]).toMatchObject({
      componentId: 'component-2',
      dataSources: [{ sourceId: 'fallback-source' }]
    })
    expect(result).toEqual({
      success: true,
      data: { 'fallback-source': { ok: true } },
      timestamp: 2000,
      metadata: {
        configurationSnapshot: {
          status: 'degraded',
          errorCode: 'CONFIGURATION_SNAPSHOT_UNAVAILABLE'
        }
      }
    })
  })

  it('executes supplied requirements without a snapshot host adapter', async () => {
    const warehouse = createWarehouse()
    const executorChain = createSuccessfulExecutor({ local: { value: 7 } })
    const bridge = new SimpleDataBridge({ warehouse, executorChain, now: () => 3000, random: () => 0 })

    const result = await bridge.executeComponent({
      componentId: 'component-local',
      dataSources: [{ id: 'local', type: 'static', config: { data: { value: 7 } } }]
    })

    expect(executorChain.executeDataProcessingChain.mock.calls[0][0]).toMatchObject({
      componentId: 'component-local',
      dataSources: [{ sourceId: 'local' }]
    })
    expect(result).toEqual({ success: true, data: { local: { value: 7 } }, timestamp: 3000 })
  })

  it('propagates an external-blocked failure without caching or notifying it', async () => {
    const warehouse = createWarehouse()
    const executorChain = {
      executeDataProcessingChain: vi.fn(async (): Promise<ExecutionResult> => ({
        success: false,
        componentData: {
          stream: {
            type: 'websocket',
            data: null,
            lastUpdated: 3000,
            metadata: { success: false, errorCode: 'WS_EXTERNAL_BLOCKED' }
          }
        },
        error: 'WebSocket数据源需要外部订阅适配器',
        errorCode: 'WS_EXTERNAL_BLOCKED',
        executionTime: 1,
        timestamp: 3000
      }))
    }
    const bridge = new SimpleDataBridge({ warehouse, executorChain, now: () => 3000, random: () => 0 })
    const callback = vi.fn()
    bridge.onDataUpdate(callback)

    const result = await bridge.executeComponent({
      componentId: 'component-stream',
      dataSources: [
        { id: 'stream', type: 'websocket', config: { wsUrl: 'wss://example.test/telemetry' } }
      ]
    })

    expect(result).toMatchObject({ success: false, errorCode: 'WS_EXTERNAL_BLOCKED' })
    expect(warehouse.storeComponentData).not.toHaveBeenCalled()
    expect(callback).not.toHaveBeenCalled()
  })

  it('does not destroy an injected warehouse by default', () => {
    const warehouse = createWarehouse()
    const bridge = new SimpleDataBridge({ warehouse })

    bridge.destroy()

    expect(warehouse.destroy).not.toHaveBeenCalled()
  })

  it('destroys an explicitly owned warehouse', () => {
    const warehouse = createWarehouse()
    const bridge = new SimpleDataBridge({ warehouse, destroyWarehouseOnDispose: true })

    bridge.destroy()

    expect(warehouse.destroy).toHaveBeenCalledOnce()
  })
})
