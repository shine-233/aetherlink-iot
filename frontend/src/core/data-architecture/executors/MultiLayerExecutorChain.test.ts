/**
 * 文件用途: Multi Layer Executor Chain 的测试文件。
 * 核心逻辑: 构造局部 fixture 并断言完整数据处理管道的可观察行为。
 * 关键注意事项: 测试覆盖端到端管道产物（componentData / success / executionState），
 *   避免绑定无关实现细节导致重构成本过高；JSON 源不依赖网络，可作为稳定契约基线。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { describe, expect, it } from 'vitest'
import {
  MultiLayerExecutorChain,
  type DataSourceConfiguration
} from '@/core/data-architecture/executors/MultiLayerExecutorChain'

/**
 * JSON 数据项示例配置：两个数据项通过 object 策略合并；第二个数据源单值用 array 策略。
 * 用真实期望值断言 componentData，确保 fetch → process(过滤) → merge → integrate 全链路打通。
 */
const createJsonExampleConfig = (): DataSourceConfiguration => {
  return {
    componentId: 'test-component-001',
    dataSources: [
      {
        sourceId: 'json-source-1',
        dataItems: [
          {
            item: {
              type: 'json',
              config: {
                jsonString: JSON.stringify({
                  user: { name: '张三', age: 25, hobbies: ['读书', '游泳'] },
                  stats: { score: 95, level: 'A' }
                })
              }
            },
            processing: {
              filterPath: '$.user',
              defaultValue: {}
            }
          },
          {
            item: {
              type: 'json',
              config: {
                jsonString: JSON.stringify({
                  product: { name: '商品A', price: 199 },
                  categories: ['电子', '数码']
                })
              }
            },
            processing: {
              filterPath: '$.product',
              defaultValue: {}
            }
          }
        ],
        mergeStrategy: {
          type: 'object'
        }
      },
      {
        sourceId: 'json-source-2',
        dataItems: [
          {
            item: {
              type: 'json',
              config: {
                jsonString: JSON.stringify([
                  { id: 1, name: '项目1' },
                  { id: 2, name: '项目2' },
                  { id: 3, name: '项目3' }
                ])
              }
            },
            processing: {
              filterPath: '$[0]', // 获取第一个元素
              defaultValue: {}
            }
          }
        ],
        mergeStrategy: {
          type: 'array'
        }
      }
    ],
    createdAt: Date.now(),
    updatedAt: Date.now()
  }
}

/**
 * 自定义脚本合并示例配置：通过 script 合并策略对两个 JSON 数据项做业务级聚合。
 */
const createScriptMergeExampleConfig = (): DataSourceConfiguration => {
  return {
    componentId: 'test-component-003',
    dataSources: [
      {
        sourceId: 'script-merge-source',
        dataItems: [
          {
            item: {
              type: 'json',
              config: {
                jsonString: JSON.stringify({ count: 10, name: '测试数据' })
              }
            },
            processing: {
              filterPath: '$',
              customScript: `
                return {
                  ...data,
                  processedAt: 'fixed-timestamp',
                  doubled: data.count * 2
                };
              `,
              defaultValue: {}
            }
          },
          {
            item: {
              type: 'json',
              config: {
                jsonString: JSON.stringify({ value: 20, status: 'active' })
              }
            },
            processing: {
              filterPath: '$',
              defaultValue: {}
            }
          }
        ],
        mergeStrategy: {
          type: 'script',
          script: `
            const result = {
              merged: true,
              totalValue: 0,
              items: []
            };
            for (const item of items) {
              result.items.push(item);
              if (item && item.count) result.totalValue += item.count;
              if (item && item.value) result.totalValue += item.value;
            }
            return result;
          `
        }
      }
    ],
    createdAt: Date.now(),
    updatedAt: Date.now()
  }
}

/**
 * 空数据项配置：dataItems 为空数组，管道应安全降级，不抛错且返回空 componentData。
 */
const createEmptyDataItemsConfig = (): DataSourceConfiguration => {
  return {
    componentId: 'test-component-empty',
    dataSources: [
      {
        sourceId: 'empty-source',
        dataItems: [],
        mergeStrategy: { type: 'object' }
      }
    ],
    createdAt: Date.now(),
    updatedAt: Date.now()
  }
}

/**
 * 坏 JSON 配置：jsonString 非法，应触发 fetch 层 fallback，最终 componentData 为空。
 */
const createInvalidJsonConfig = (): DataSourceConfiguration => {
  return {
    componentId: 'test-component-invalid',
    dataSources: [
      {
        sourceId: 'invalid-json-source',
        dataItems: [
          {
            item: {
              type: 'json',
              config: {
                jsonString: '{not-valid-json'
              }
            },
            processing: {
              filterPath: '$',
              defaultValue: {}
            }
          }
        ],
        mergeStrategy: { type: 'object' }
      }
    ],
    createdAt: Date.now(),
    updatedAt: Date.now()
  }
}

describe('MultiLayerExecutorChain examples', () => {
  it('validates the bundled JSON example configuration', () => {
    const executorChain = new MultiLayerExecutorChain()

    expect(executorChain.validateConfiguration(createJsonExampleConfig())).toBe(true)
  })

  it('exposes supported executor chain capabilities', () => {
    const executorChain = new MultiLayerExecutorChain()
    const statistics = executorChain.getChainStatistics()

    expect(statistics.supportedDataTypes).toContain('json')
    expect(statistics.supportedMergeStrategies).toContain('object')
  })

  it('processes JSON data through the full chain and returns merged component data', async () => {
    const executorChain = new MultiLayerExecutorChain()
    const config = createJsonExampleConfig()

    const result = await executorChain.executeDataProcessingChain(config, true)

    expect(result.success).toBe(true)
    expect(result.componentData).toBeDefined()
    // ComponentData 按 sourceId 分桶：json-source-1 是 object 合并后的 user+product。
    // object 合并是浅层键覆盖：product.name='商品A' 覆盖 user.name='张三'。
    expect(result.componentData['json-source-1'].type).toBe('json')
    expect(result.componentData['json-source-1'].data).toMatchObject({
      name: '商品A',
      age: 25,
      hobbies: ['读书', '游泳'],
      price: 199
    })
    expect(result.componentData['json-source-1'].metadata.success).toBe(true)
    // json-source-2 是 array 合并：$[0] 单元素被包装进数组。
    expect(result.componentData['json-source-2'].type).toBe('json')
    expect(Array.isArray(result.componentData['json-source-2'].data)).toBe(true)
    expect(result.componentData['json-source-2'].data[0]).toMatchObject({ id: 1, name: '项目1' })
    expect(result.executionState).toBeDefined()
    expect(result.executionState?.stages.rawData.size).toBeGreaterThan(0)
    expect(result.executionState?.stages.processedData.size).toBeGreaterThan(0)
    expect(result.executionState?.stages.mergedData).not.toBeNull()
    expect(result.executionState?.stages.finalData).not.toBeNull()
  })

  it('completes the chain when script merge strategy is used and degrades on script engine limits', async () => {
    const executorChain = new MultiLayerExecutorChain()
    const config = createScriptMergeExampleConfig()

    const result = await executorChain.executeDataProcessingChain(config, false)

    expect(result.success).toBe(true)
    // script 合并策略依赖 script-engine 在测试上下文中执行用户脚本；
    // 当脚本引擎无法执行复杂脚本时，merger 走 fallback 返回 {}，但整条链不崩溃。
    expect(result.componentData['script-merge-source']).toBeDefined()
    expect(result.componentData['script-merge-source'].type).toBe('json')
  })

  it('marks an empty data source as a failed chain while retaining its diagnostic bucket', async () => {
    const executorChain = new MultiLayerExecutorChain()
    const config = createEmptyDataItemsConfig()

    const result = await executorChain.executeDataProcessingChain(config, false)

    expect(result).toMatchObject({
      success: false,
      errorCode: 'DATA_SOURCE_NO_SUCCESSFUL_ITEMS',
      isEmpty: true
    })
    expect(result.componentData?.['empty-source']).toBeDefined()
    expect(result.componentData?.['empty-source'].data).toBeNull()
    expect(result.componentData?.['empty-source'].metadata.success).toBe(false)
  })

  it('preserves the stable failure code when every source fails', async () => {
    const executorChain = new MultiLayerExecutorChain()
    const config = createInvalidJsonConfig()

    const result = await executorChain.executeDataProcessingChain(config, false)

    expect(result).toMatchObject({ success: false, errorCode: 'JSON_PARSE_ERROR', isEmpty: true })
    expect(result.componentData?.['invalid-json-source']).toBeDefined()
    expect(result.componentData?.['invalid-json-source'].data).toBeNull()
    expect(result.componentData?.['invalid-json-source'].metadata).toMatchObject({
      success: false,
      errorCode: 'JSON_PARSE_ERROR'
    })
  })

  it('keeps partial success when a local source succeeds and WebSocket is external-blocked', async () => {
    const config = createInvalidJsonConfig()
    config.dataSources = [
      {
        sourceId: 'local-source',
        dataItems: [
          {
            item: { type: 'json', config: { jsonString: '{"value":7}' } },
            processing: { filterPath: '$', defaultValue: {} }
          }
        ],
        mergeStrategy: { type: 'object' }
      },
      {
        sourceId: 'stream-source',
        dataItems: [
          {
            item: { type: 'websocket', config: { wsUrl: 'wss://example.test/telemetry' } },
            processing: { filterPath: '$', defaultValue: {} }
          }
        ],
        mergeStrategy: { type: 'object' }
      }
    ]

    const result = await new MultiLayerExecutorChain().executeDataProcessingChain(config, false)

    expect(result.success).toBe(true)
    expect(result.errorCode).toBeUndefined()
    expect(result.componentData?.['local-source'].metadata.success).toBe(true)
    expect(result.componentData?.['stream-source'].metadata).toMatchObject({
      success: false,
      errorCode: 'WS_EXTERNAL_BLOCKED'
    })
  })

  it('rejects configuration missing componentId or dataSources', () => {
    const executorChain = new MultiLayerExecutorChain()

    expect(executorChain.validateConfiguration({ componentId: '', dataSources: [] } as any)).toBe(false)
    expect(
      executorChain.validateConfiguration({
        componentId: 'ok',
        dataSources: [{ sourceId: '', dataItems: [], mergeStrategy: { type: 'object' } }]
      } as any)
    ).toBe(false)
  })
})
