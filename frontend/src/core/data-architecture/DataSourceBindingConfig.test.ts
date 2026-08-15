/**
 * 文件用途: Data Source Binding Config 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { describe, expect, it, vi } from 'vitest'

import {
  dataSourceBindingConfig,
  DataSourceBindingConfig,
  installDataSourceBindingConfigDebugGlobal
} from './DataSourceBindingConfig'

const createComponentConfig = () => ({
  base: {
    deviceId: 'device-001',
    metricsList: ['temperature', 'humidity']
  },
  component: {
    startTime: new Date('2026-06-27T00:00:00.000Z'),
    endTime: new Date('2026-06-27T01:00:00.000Z'),
    dataType: 'telemetry',
    refreshInterval: '45',
    filterCondition: 'status=online',
    tenantId: 'tenant-a'
  }
})

describe('DataSourceBindingConfig', () => {
  it('builds HTTP params from default visualization data bindings', () => {
    const config = new DataSourceBindingConfig()

    expect(config.buildHttpParams(createComponentConfig())).toEqual({
      deviceId: 'device-001',
      metrics: 'temperature,humidity',
      startTime: '2026-06-27T00:00:00.000Z',
      endTime: '2026-06-27T01:00:00.000Z',
      dataType: 'telemetry',
      refreshInterval: 45,
      filter: 'status=online'
    })
  })

  it('honors strict, loose, disabled, and custom auto-bind modes', () => {
    const config = new DataSourceBindingConfig()
    const componentConfig = createComponentConfig()

    expect(
      config.buildAutoBindParams(componentConfig, {
        enabled: true,
        mode: 'strict',
        includeProperties: ['base.deviceId', 'component.refreshInterval']
      })
    ).toEqual({
      deviceId: 'device-001',
      refreshInterval: 45
    })

    expect(
      config.buildAutoBindParams(componentConfig, {
        enabled: true,
        mode: 'loose',
        excludeProperties: ['base.metricsList', 'component.filterCondition']
      })
    ).toEqual({
      deviceId: 'device-001',
      startTime: '2026-06-27T00:00:00.000Z',
      endTime: '2026-06-27T01:00:00.000Z',
      dataType: 'telemetry',
      refreshInterval: 45
    })

    expect(
      config.buildAutoBindParams(componentConfig, {
        enabled: true,
        mode: 'custom',
        customRules: [
          {
            propertyPath: 'component.tenantId',
            paramName: 'tenant_id',
            transform: (value) => String(value).toUpperCase()
          }
        ]
      })
    ).toEqual({ tenant_id: 'TENANT-A' })

    expect(config.buildAutoBindParams(componentConfig, { enabled: false, mode: 'strict' })).toMatchObject({
      deviceId: 'device-001',
      metrics: 'temperature,humidity'
    })
  })

  it('falls back to the original value when a binding transform fails', () => {
    const config = new DataSourceBindingConfig()
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => undefined)

    config.registerBindingRule({
      propertyPath: 'component.tenantId',
      paramName: 'tenant_id',
      transform: () => {
        throw new Error('transform failed')
      }
    })

    expect(config.buildHttpParams(createComponentConfig())).toMatchObject({
      tenant_id: 'tenant-a'
    })
    expect(warnSpy).toHaveBeenCalledTimes(1)

    warnSpy.mockRestore()
  })

  it('applies dynamic rules, component-specific rules, and trigger gates', () => {
    const config = new DataSourceBindingConfig()

    config.addCustomBindingRule({
      propertyPath: 'component.tenantId',
      paramName: 'tenantId'
    })
    config.addCustomTriggerRule({
      propertyPath: 'component.tenantId',
      enabled: true,
      debounceMs: 50
    })
    config.setComponentConfig('rdi-card', {
      componentType: 'rdi-card',
      additionalBindings: [{ propertyPath: 'component.filterCondition', paramName: 'rdi_filter' }],
      additionalTriggers: [{ propertyPath: 'component.filterCondition', enabled: true, debounceMs: 10 }]
    })

    expect(config.getBindingRule('component.tenantId')?.paramName).toBe('tenantId')
    expect(config.shouldTriggerDataSource('component.tenantId')).toBe(true)
    expect(config.shouldTriggerDataSource('component.refreshInterval')).toBe(false)
    expect(config.getBindingRule('component.filterCondition', 'rdi-card')?.paramName).toBe('filter')
    expect(config.getAllBindingRules('rdi-card').some((rule) => rule.paramName === 'rdi_filter')).toBe(true)
    expect(config.getTriggerRule('component.filterCondition', 'rdi-card')?.debounceMs).toBe(250)

    expect(config.removeCustomBindingRule('component.tenantId')).toBe(true)
    expect(config.removeCustomTriggerRule('component.tenantId')).toBe(true)
    expect(config.getBindingRule('component.tenantId')).toBeUndefined()
    expect(config.shouldTriggerDataSource('component.tenantId')).toBe(false)
  })

  it('reports debug information without touching private implementation-only fields', () => {
    const config = new DataSourceBindingConfig()

    const debugInfo = config.getDebugInfo('rdi-card')

    expect(debugInfo.baseBindingRules).toBe(7)
    expect(debugInfo.baseTriggerRules).toBe(7)
    expect(debugInfo.currentBindingRules).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ propertyPath: 'base.deviceId', paramName: 'deviceId', required: true }),
        expect.objectContaining({ propertyPath: 'base.metricsList', paramName: 'metrics' })
      ])
    )
    expect(debugInfo.currentTriggerRules).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ propertyPath: 'base.deviceId', enabled: true, debounceMs: 100 })
      ])
    )
  })

  it('keeps default v1-v2 binding rules as retained runtime contract until migrated through rule APIs', () => {
    const config = new DataSourceBindingConfig()

    expect(config.buildHttpParams(createComponentConfig())).toMatchObject({
      deviceId: 'device-001',
      metrics: 'temperature,humidity'
    })

    expect(config.removeBindingRule('base.deviceId')).toBe(true)
    expect(config.buildHttpParams(createComponentConfig())).not.toHaveProperty('deviceId')

    config.registerBindingRule({
      propertyPath: 'base.deviceId',
      paramName: 'device_id'
    })

    expect(config.buildHttpParams(createComponentConfig())).toMatchObject({
      device_id: 'device-001'
    })
  })

  it('clears only base rules while preserving custom and component extensions', () => {
    const config = new DataSourceBindingConfig()

    config.addCustomBindingRule({
      propertyPath: 'component.tenantId',
      paramName: 'tenantId'
    })
    config.addCustomTriggerRule({
      propertyPath: 'component.tenantId',
      enabled: true
    })
    config.setComponentConfig('rdi-card', {
      componentType: 'rdi-card',
      additionalBindings: [{ propertyPath: 'component.cardId', paramName: 'cardId' }],
      additionalTriggers: [{ propertyPath: 'component.cardId', enabled: true }]
    })

    config.clearAllRules()

    expect(config.getDebugInfo('rdi-card')).toMatchObject({
      baseBindingRules: 0,
      baseTriggerRules: 0,
      customBindingRules: 1,
      customTriggerRules: 1
    })
    expect(config.getBindingRule('base.deviceId')).toBeUndefined()
    expect(config.getBindingRule('component.tenantId')?.paramName).toBe('tenantId')
    expect(config.getTriggerRule('component.tenantId')?.enabled).toBe(true)
    expect(config.getBindingRule('component.cardId', 'rdi-card')?.paramName).toBe('cardId')
    expect(config.getTriggerRule('component.cardId', 'rdi-card')?.enabled).toBe(true)
    expect(config.getComponentConfig('rdi-card')?.componentType).toBe('rdi-card')
  })

  it('installs and reversibly cleans up the explicit debug global', () => {
    const emptyTarget: Record<string, any> = {}
    const removeNewValue = installDataSourceBindingConfigDebugGlobal(emptyTarget)

    expect(emptyTarget.__dataSourceBindingConfig).toBe(dataSourceBindingConfig)
    removeNewValue()
    removeNewValue()
    expect(emptyTarget).not.toHaveProperty('__dataSourceBindingConfig')

    const previousValue = { host: true }
    const targetWithPreviousValue: Record<string, any> = {
      __dataSourceBindingConfig: previousValue
    }
    const restorePreviousValue = installDataSourceBindingConfigDebugGlobal(targetWithPreviousValue)

    expect(targetWithPreviousValue.__dataSourceBindingConfig).toBe(dataSourceBindingConfig)
    restorePreviousValue()
    expect(targetWithPreviousValue.__dataSourceBindingConfig).toBe(previousValue)

    const hostReplacement = { host: 'replacement' }
    const targetOverwrittenByHost: Record<string, any> = {}
    const preserveHostReplacement = installDataSourceBindingConfigDebugGlobal(targetOverwrittenByHost)
    targetOverwrittenByHost.__dataSourceBindingConfig = hostReplacement
    preserveHostReplacement()

    expect(targetOverwrittenByHost.__dataSourceBindingConfig).toBe(hostReplacement)
  })
})
