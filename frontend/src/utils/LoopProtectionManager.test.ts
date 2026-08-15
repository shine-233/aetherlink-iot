/*
 * 文件用途：验证循环保护管理器的调用跟踪、黑名单、递归深度和包装清理行为。
 * 核心逻辑：通过异步/同步方法模拟高频调用和递归溢出，断言保护状态可恢复。
 * 关键注意事项：测试应保持时间窗口稳定，避免依赖真实长时间等待。
 * 重构建议：如配置项增加，需要补充阈值边界测试。
 */
import { afterEach, describe, expect, it, vi } from 'vitest'

const loadLoopProtection = async () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-06-27T00:00:00.000Z'))

  const mod = await import('./LoopProtectionManager')
  mod.loopProtectionManager.reset()
  mod.loopProtectionManager.updateConfig({
    maxDepth: 10,
    timeWindow: 1000,
    maxCallsInWindow: 50,
    enableDebug: false
  })
  return mod
}

afterEach(async () => {
  const { loopProtectionManager } = await import('./LoopProtectionManager')
  loopProtectionManager.reset()
  vi.clearAllTimers()
  vi.useRealTimers()
})

describe('LoopProtectionManager', () => {
  it('tracks active calls and clears them when the protected call ends', async () => {
    const { loopProtectionManager } = await loadLoopProtection()

    const callId = loopProtectionManager.markCallStart('refreshData', 'card-a', 'unit-test')

    expect(callId).toContain('refreshData:card-a')
    expect(loopProtectionManager.getPerformanceStats()).toMatchObject({
      activeCallsCount: 1,
      totalTrackedFunctions: 1,
      blacklistedFunctionsCount: 0
    })

    loopProtectionManager.markCallEnd(callId, 'refreshData', 'card-a')

    expect(loopProtectionManager.getPerformanceStats()).toMatchObject({
      activeCallsCount: 0,
      totalTrackedFunctions: 1
    })
  })

  it('blacklists a component call when frequency exceeds the configured time window', async () => {
    const { loopProtectionManager } = await loadLoopProtection()
    loopProtectionManager.updateConfig({ maxCallsInWindow: 2, timeWindow: 1000 })

    expect(loopProtectionManager.shouldAllowCall('executeDataSource', 'card-a')).toBe(true)
    expect(loopProtectionManager.shouldAllowCall('executeDataSource', 'card-a')).toBe(true)
    expect(loopProtectionManager.shouldAllowCall('executeDataSource', 'card-a')).toBe(false)
    expect(loopProtectionManager.shouldAllowCall('executeDataSource', 'card-a')).toBe(false)

    expect(loopProtectionManager.getBlacklistedFunctions()).toContain('executeDataSource:card-a')
    expect(loopProtectionManager.getPerformanceStats()).toMatchObject({
      totalLoopsDetected: 1,
      totalCallsBlocked: 1,
      blacklistedFunctionsCount: 1
    })
  })

  it('blocks recursive depth overflow before recording another active call', async () => {
    const { loopProtectionManager } = await loadLoopProtection()
    loopProtectionManager.updateConfig({ maxDepth: 1 })

    const firstCall = loopProtectionManager.markCallStart('syncConfig', 'card-b', 'unit-test')
    const recursiveCall = loopProtectionManager.markCallStart('syncConfig', 'card-b', 'unit-test')

    expect(firstCall).not.toBe('')
    expect(recursiveCall).toBe('')
    expect(loopProtectionManager.getBlacklistedFunctions()).toContain('syncConfig:card-b')
    expect(loopProtectionManager.getPerformanceStats()).toMatchObject({
      activeCallsCount: 1,
      blacklistedFunctionsCount: 1
    })
  })

  it('resets history, active calls, blacklist, and counters', async () => {
    const { loopProtectionManager } = await loadLoopProtection()
    loopProtectionManager.updateConfig({ maxCallsInWindow: 1 })

    expect(loopProtectionManager.shouldAllowCall('reload', 'card-c')).toBe(true)
    expect(loopProtectionManager.shouldAllowCall('reload', 'card-c')).toBe(false)
    expect(loopProtectionManager.getBlacklistedFunctions()).toContain('reload:card-c')

    loopProtectionManager.reset()

    expect(loopProtectionManager.getBlacklistedFunctions()).toEqual([])
    expect(loopProtectionManager.getPerformanceStats()).toMatchObject({
      totalCallsBlocked: 0,
      totalLoopsDetected: 0,
      activeCallsCount: 0,
      totalTrackedFunctions: 0
    })
  })

  it('wraps synchronous and asynchronous methods with loop protection cleanup', async () => {
    const { loopProtection, loopProtectionManager } = await loadLoopProtection()

    class FixtureComponent {
      componentId = 'pump-status-card'

      runSync() {
        return 'sync-ok'
      }

      async runAsync() {
        return 'async-ok'
      }
    }

    for (const methodName of ['runSync', 'runAsync'] as const) {
      const descriptor = Object.getOwnPropertyDescriptor(FixtureComponent.prototype, methodName)!
      loopProtection(`FixtureComponent.${methodName}`)(FixtureComponent.prototype, methodName, descriptor)
      Object.defineProperty(FixtureComponent.prototype, methodName, descriptor)
    }

    const component = new FixtureComponent()

    expect(component.runSync()).toBe('sync-ok')
    expect(await component.runAsync()).toBe('async-ok')
    expect(loopProtectionManager.getPerformanceStats().activeCallsCount).toBe(0)
  })
})
