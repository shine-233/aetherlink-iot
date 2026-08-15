/**
 * 文件用途：验证数据仓库后台维护任务的显式生命周期。
 * 核心逻辑：隔离模块导入并统计定时器注册/清理，确保默认单例没有导入副作用。
 * 关键注意事项：测试只观察定时器契约，不等待真实时间，也不依赖外部服务。
 */
import { afterEach, describe, expect, it, vi } from 'vitest'

afterEach(() => {
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('EnhancedDataWarehouse maintenance lifecycle', () => {
  it('does not start timers when the module creates its default warehouse', async () => {
    vi.resetModules()
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval')

    const { dataWarehouse, EnhancedDataWarehouse } = await import('./DataWarehouse')

    expect(dataWarehouse).toBeInstanceOf(EnhancedDataWarehouse)
    expect(setIntervalSpy).not.toHaveBeenCalled()
  })

  it('keeps background maintenance enabled for explicitly created default instances', async () => {
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval')
    const clearIntervalSpy = vi.spyOn(globalThis, 'clearInterval')
    const { EnhancedDataWarehouse } = await import('./DataWarehouse')
    setIntervalSpy.mockClear()

    const warehouse = new EnhancedDataWarehouse()

    expect(setIntervalSpy).toHaveBeenCalledTimes(2)
    warehouse.destroy()
    expect(clearIntervalSpy).toHaveBeenCalledTimes(2)
  })

  it('supports local maintenance without starting background timers', async () => {
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval')
    const { EnhancedDataWarehouse } = await import('./DataWarehouse')
    setIntervalSpy.mockClear()
    const warehouse = new EnhancedDataWarehouse({ enableBackgroundMaintenance: false })

    warehouse.storeComponentData('component-1', 'source-1', { value: 1 }, 'static')
    expect(() => warehouse.performMaintenance()).not.toThrow()
    expect(warehouse.getComponentData('component-1')).toEqual({ 'source-1': { value: 1 } })
    expect(setIntervalSpy).not.toHaveBeenCalled()

    warehouse.destroy()
  })

  it('starts only cleanup when performance monitoring is disabled', async () => {
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval')
    const clearIntervalSpy = vi.spyOn(globalThis, 'clearInterval')
    const { EnhancedDataWarehouse } = await import('./DataWarehouse')
    setIntervalSpy.mockClear()

    const warehouse = new EnhancedDataWarehouse({ enablePerformanceMonitoring: false })

    expect(setIntervalSpy).toHaveBeenCalledOnce()
    warehouse.destroy()
    expect(clearIntervalSpy).toHaveBeenCalledOnce()
  })
})
