/**
 * 文件用途: Config Event Bus 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { describe, expect, it, vi } from 'vitest'

import { ConfigEventBus, type ConfigChangeEvent } from './ConfigEventBus'

const createEvent = (overrides: Partial<ConfigChangeEvent> = {}): ConfigChangeEvent => ({
  componentId: 'rdi-card',
  componentType: 'rdi-device-operations',
  section: 'dataSource',
  oldConfig: { url: '/old' },
  newConfig: { url: '/new' },
  timestamp: 1710000000000,
  source: 'user',
  context: {
    changedFields: ['dataSource.url'],
    shouldTriggerExecution: true
  },
  ...overrides
})

describe('ConfigEventBus', () => {
  it('dispatches generic and section-specific handlers for one config change', async () => {
    const bus = new ConfigEventBus()
    const genericHandler = vi.fn()
    const dataSourceHandler = vi.fn()

    bus.onConfigChange('config-changed', genericHandler)
    bus.onConfigChange('data-source-changed', dataSourceHandler)

    await bus.emitConfigChange(createEvent())

    expect(genericHandler).toHaveBeenCalledWith(expect.objectContaining({ section: 'dataSource' }))
    expect(dataSourceHandler).toHaveBeenCalledWith(expect.objectContaining({ section: 'dataSource' }))
    expect(bus.getStatistics()).toMatchObject({
      eventsEmitted: 1,
      eventsFiltered: 0,
      handlersExecuted: 2,
      errors: 0
    })
  })

  it('unregisters handlers and removes empty handler buckets', async () => {
    const bus = new ConfigEventBus()
    const handler = vi.fn()
    const off = bus.onConfigChange('component-props-changed', handler)

    off()
    await bus.emitConfigChange(createEvent({ section: 'component' }))

    expect(handler).toHaveBeenCalledTimes(0)
    expect(bus.getStatistics()).toMatchObject({ eventsEmitted: 1, handlersExecuted: 0 })
  })

  it('applies priority filters and blocks events that do not pass', async () => {
    const bus = new ConfigEventBus()
    const order: string[] = []
    const handler = vi.fn()

    bus.onConfigChange('config-changed', handler)
    bus.addEventFilter({
      name: 'second',
      priority: 5,
      condition: () => {
        order.push('second')
        return true
      }
    })
    bus.addEventFilter({
      name: 'first',
      priority: 10,
      condition: () => {
        order.push('first')
        return false
      }
    })

    await bus.emitConfigChange(createEvent())

    expect(order).toEqual(['first'])
    expect(handler).toHaveBeenCalledTimes(0)
    expect(bus.getStatistics()).toMatchObject({
      eventsEmitted: 1,
      eventsFiltered: 1,
      handlersExecuted: 0
    })
  })

  it('lets events pass when a filter throws and isolates handler errors', async () => {
    const bus = new ConfigEventBus()
    const healthyHandler = vi.fn()

    bus.addEventFilter({
      name: 'broken-filter',
      condition: () => {
        throw new Error('filter failed')
      }
    })
    bus.onConfigChange('config-changed', () => {
      throw new Error('handler failed')
    })
    bus.onConfigChange('config-changed', healthyHandler)

    await bus.emitConfigChange(createEvent())

    expect(healthyHandler).toHaveBeenCalledTimes(1)
    expect(bus.getStatistics()).toMatchObject({
      eventsEmitted: 1,
      eventsFiltered: 0,
      handlersExecuted: 2,
      errors: 1
    })
  })

  it('removes filters by name and clears handlers, filters, and statistics', async () => {
    const bus = new ConfigEventBus()
    const handler = vi.fn()

    bus.onConfigChange('config-changed', handler)
    bus.addEventFilter({ name: 'blocker', condition: () => false })
    bus.removeEventFilter('blocker')

    await bus.emitConfigChange(createEvent())
    expect(handler).toHaveBeenCalledTimes(1)

    bus.clear()
    await bus.emitConfigChange(createEvent())

    expect(handler).toHaveBeenCalledTimes(1)
    expect(bus.getStatistics()).toMatchObject({
      eventsEmitted: 1,
      eventsFiltered: 0,
      handlersExecuted: 0,
      errors: 0
    })
  })
})
