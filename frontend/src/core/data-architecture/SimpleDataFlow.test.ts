/**
 * 文件用途: Simple Data Flow 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  installSimpleDataFlowDebugGlobal,
  simpleDataFlow,
  SimpleDataFlow
} from './SimpleDataFlow'

const { bridgeMock, getVisualEditorBridgeMock, simpleDataBridgeMock, bindingConfigMock, loggerMock } = vi.hoisted(
  () => ({
    bridgeMock: {
      updateComponentExecutor: vi.fn()
    },
    getVisualEditorBridgeMock: vi.fn(),
    simpleDataBridgeMock: {
      clearComponentCache: vi.fn()
    },
    bindingConfigMock: {
      shouldTriggerDataSource: vi.fn(),
      buildHttpParams: vi.fn(),
      buildAutoBindParams: vi.fn(),
      getComponentConfig: vi.fn(),
      getAllTriggerRules: vi.fn(),
      addCustomTriggerRule: vi.fn(),
      addCustomBindingRule: vi.fn(),
      setComponentConfig: vi.fn(),
      getDebugInfo: vi.fn()
    },
    loggerMock: {
      debug: vi.fn(),
      warn: vi.fn(),
      error: vi.fn(),
      info: vi.fn()
    }
  })
)

vi.mock('./VisualEditorBridge', () => ({
  getVisualEditorBridge: getVisualEditorBridgeMock
}))

vi.mock('./SimpleDataBridge', () => ({
  simpleDataBridge: simpleDataBridgeMock
}))

vi.mock('./DataSourceBindingConfig', () => ({
  dataSourceBindingConfig: bindingConfigMock
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => loggerMock
}))

const flow = () => SimpleDataFlow.getInstance()

const registerDataComponent = (componentId: string) => {
  flow().registerComponent(componentId, {
    componentType: 'rdi-card',
    base: { deviceId: 'old-device' },
    component: { title: 'RDI' },
    dataSource: {
      dataSources: [
        {
          sourceId: 'main',
          dataItems: [],
          mergeStrategy: { type: 'object' }
        }
      ]
    },
    interaction: { responses: [] }
  })
}

describe('SimpleDataFlow', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T02:00:00.000Z'))
    vi.clearAllMocks()
    bridgeMock.updateComponentExecutor.mockResolvedValue({ success: true })
    getVisualEditorBridgeMock.mockReturnValue(bridgeMock)
    bindingConfigMock.shouldTriggerDataSource.mockReturnValue(false)
    bindingConfigMock.buildHttpParams.mockReturnValue({ device_id: 'built-device' })
    bindingConfigMock.buildAutoBindParams.mockReturnValue({ auto_device_id: 'auto-device' })
    bindingConfigMock.getComponentConfig.mockReturnValue(null)
    bindingConfigMock.getAllTriggerRules.mockReturnValue([
      { propertyPath: 'base.deviceId' },
      { propertyPath: 'component.metricsList' }
    ])
    bindingConfigMock.getDebugInfo.mockReturnValue({ rules: 2 })
  })

  afterEach(() => {
    ;['card-flow', 'card-debounce', 'card-manual', 'card-empty', 'card-unregister', 'card-watch'].forEach(componentId =>
      flow().unregisterComponent(componentId)
    )
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  it('debounces whitelisted property changes into one data-source execution', async () => {
    bindingConfigMock.shouldTriggerDataSource.mockImplementation(propertyPath => propertyPath === 'base.deviceId')
    const watcher = vi.fn()
    const unsubscribe = flow().addPropertyWatcher('base.deviceId', watcher)
    registerDataComponent('card-flow')

    flow().updateComponentConfig('card-flow', 'base', { deviceId: 'new-device' })

    expect(watcher).toHaveBeenCalledWith(
      expect.objectContaining({
        componentId: 'card-flow',
        propertyPath: 'base.deviceId',
        oldValue: 'old-device',
        newValue: 'new-device',
        timestamp: Date.now()
      })
    )
    expect(bridgeMock.updateComponentExecutor).toHaveBeenCalledTimes(0)

    await vi.advanceTimersByTimeAsync(100)

    expect(simpleDataBridgeMock.clearComponentCache).toHaveBeenCalledWith('card-flow')
    expect(bindingConfigMock.buildHttpParams).toHaveBeenCalledWith(
      expect.objectContaining({
        base: { deviceId: 'new-device' }
      }),
      'rdi-card'
    )
    expect(bridgeMock.updateComponentExecutor).toHaveBeenCalledWith(
      'card-flow',
      'rdi-card',
      expect.objectContaining({
        base: { deviceId: 'new-device' },
        component: { title: 'RDI' },
        interaction: { responses: [] },
        _httpParams: { device_id: 'built-device' }
      })
    )

    unsubscribe()
  })

  it('coalesces rapid changes for the same component into the latest execution', async () => {
    bindingConfigMock.shouldTriggerDataSource.mockReturnValue(true)
    registerDataComponent('card-debounce')

    flow().updateComponentConfig('card-debounce', 'base', { deviceId: 'device-1' })
    flow().updateComponentConfig('card-debounce', 'base', { deviceId: 'device-2' })

    await vi.advanceTimersByTimeAsync(99)
    expect(bridgeMock.updateComponentExecutor).toHaveBeenCalledTimes(0)

    await vi.advanceTimersByTimeAsync(1)
    expect(bridgeMock.updateComponentExecutor).toHaveBeenCalledTimes(1)
    expect(bridgeMock.updateComponentExecutor).toHaveBeenCalledWith(
      'card-debounce',
      'rdi-card',
      expect.objectContaining({
        base: { deviceId: 'device-2' }
      })
    )
  })

  it('does not execute when the changed value is unchanged or not whitelisted', async () => {
    registerDataComponent('card-flow')

    flow().updateComponentConfig('card-flow', 'base', { deviceId: 'old-device' })
    flow().updateComponentConfig('card-flow', 'component', { title: 'New title' })

    await vi.advanceTimersByTimeAsync(150)
    expect(bridgeMock.updateComponentExecutor).toHaveBeenCalledTimes(0)
  })

  it('manual trigger executes registered data-source configs and skips missing or empty configs', async () => {
    await flow().triggerDataSource('missing-card', 'manual-check')
    expect(bridgeMock.updateComponentExecutor).toHaveBeenCalledTimes(0)

    flow().registerComponent('card-empty', { componentType: 'widget', base: {} })
    await flow().triggerDataSource('card-empty', 'manual-check')
    expect(bridgeMock.updateComponentExecutor).toHaveBeenCalledTimes(0)

    registerDataComponent('card-manual')
    await flow().triggerDataSource('card-manual', 'manual-check')

    expect(bridgeMock.updateComponentExecutor).toHaveBeenCalledWith(
      'card-manual',
      'rdi-card',
      expect.objectContaining({
        _httpParams: { device_id: 'built-device' }
      })
    )
  })

  it('unregistering a component cancels pending debounce execution', async () => {
    bindingConfigMock.shouldTriggerDataSource.mockReturnValue(true)
    registerDataComponent('card-unregister')

    flow().updateComponentConfig('card-unregister', 'base', { deviceId: 'new-device' })
    flow().unregisterComponent('card-unregister')

    await vi.advanceTimersByTimeAsync(150)
    expect(bridgeMock.updateComponentExecutor).toHaveBeenCalledTimes(0)
  })

  it('property watchers can unsubscribe and a failing watcher does not block others', () => {
    const failingWatcher = vi.fn(() => {
      throw new Error('watcher failed')
    })
    const activeWatcher = vi.fn()
    const removedWatcher = vi.fn()

    const unsubscribeRemoved = flow().addPropertyWatcher('base.deviceId', removedWatcher)
    flow().addPropertyWatcher('base.deviceId', failingWatcher)
    const unsubscribeActive = flow().addPropertyWatcher('base.deviceId', activeWatcher)
    unsubscribeRemoved()
    registerDataComponent('card-watch')

    flow().updateComponentConfig('card-watch', 'base', { deviceId: 'watch-device' })

    expect(removedWatcher).toHaveBeenCalledTimes(0)
    expect(failingWatcher).toHaveBeenCalledTimes(1)
    expect(activeWatcher).toHaveBeenCalledTimes(1)

    unsubscribeActive()
  })

  it('proxies trigger, binding, component, and debug configuration operations', () => {
    flow().addTriggerProperty('base.productId', false, 250)
    flow().addBindingRule('base.deviceId', 'device_id', value => String(value), true)
    flow().setComponentBindingConfig('rdi-card', { triggerRules: [], bindingRules: [] } as any)

    expect(flow().getTriggerWhitelist('rdi-card')).toEqual(['base.deviceId', 'component.metricsList'])
    expect(flow().getBindingDebugInfo('rdi-card')).toEqual({ rules: 2 })
    expect(bindingConfigMock.addCustomTriggerRule).toHaveBeenCalledWith({
      propertyPath: 'base.productId',
      enabled: false,
      debounceMs: 250,
      description: '动态添加的触发规则: base.productId'
    })
    expect(bindingConfigMock.addCustomBindingRule).toHaveBeenCalledWith(
      expect.objectContaining({
        propertyPath: 'base.deviceId',
        paramName: 'device_id',
        required: true
      })
    )
    expect(bindingConfigMock.setComponentConfig).toHaveBeenCalledWith('rdi-card', {
      triggerRules: [],
      bindingRules: []
    })
    expect(bindingConfigMock.getDebugInfo).toHaveBeenCalledWith('rdi-card')
  })

  it('installs and removes the debug instance only when explicitly requested', () => {
    const target = {} as unknown as typeof globalThis
    const debugTarget = target as typeof globalThis & Record<string, unknown>
    const cleanup = installSimpleDataFlowDebugGlobal(target)

    expect(debugTarget.__simpleDataFlow).toBe(simpleDataFlow)

    cleanup()
    expect(Object.prototype.hasOwnProperty.call(debugTarget, '__simpleDataFlow')).toBe(false)
  })

  it('restores an existing debug instance during cleanup', () => {
    const previousValue = { owner: 'host' }
    const target = { __simpleDataFlow: previousValue } as unknown as typeof globalThis
    const debugTarget = target as typeof globalThis & Record<string, unknown>
    const cleanup = installSimpleDataFlowDebugGlobal(target)

    expect(debugTarget.__simpleDataFlow).toBe(simpleDataFlow)
    cleanup()
    expect(debugTarget.__simpleDataFlow).toBe(previousValue)
  })

  it('does not overwrite a host value assigned after debug installation', () => {
    const target = {} as unknown as typeof globalThis
    const debugTarget = target as typeof globalThis & Record<string, unknown>
    const cleanup = installSimpleDataFlowDebugGlobal(target)
    const hostValue = { owner: 'host-after-install' }

    debugTarget.__simpleDataFlow = hostValue
    cleanup()

    expect(debugTarget.__simpleDataFlow).toBe(hostValue)
  })
})
