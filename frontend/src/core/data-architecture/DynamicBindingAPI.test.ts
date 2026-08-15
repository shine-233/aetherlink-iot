/**
 * 文件用途: Dynamic Binding API 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { beforeEach, describe, expect, it } from 'vitest'

import { DynamicBindingAPI, installDynamicBindingDebugGlobal } from './DynamicBindingAPI'

describe('DynamicBindingAPI', () => {
  beforeEach(() => {
    delete (globalThis as any).__dynamicBindingAPI
    DynamicBindingAPI.clearAllDefaultRules()
  })

  it('does not mutate globalThis on module import', () => {
    expect((globalThis as any).__dynamicBindingAPI).toBeUndefined()
  })

  it('installs and removes the debug global explicitly', () => {
    const cleanup = installDynamicBindingDebugGlobal()

    expect((globalThis as any).__dynamicBindingAPI).toBe(DynamicBindingAPI)

    cleanup()
    expect((globalThis as any).__dynamicBindingAPI).toBeUndefined()
  })

  it('restores an existing debug global during cleanup', () => {
    const previousValue = { owner: 'host' }
    ;(globalThis as any).__dynamicBindingAPI = previousValue

    const cleanup = installDynamicBindingDebugGlobal()
    expect((globalThis as any).__dynamicBindingAPI).toBe(DynamicBindingAPI)

    cleanup()
    expect((globalThis as any).__dynamicBindingAPI).toBe(previousValue)
  })

  it('adds, lists, and removes fully custom binding and trigger rules', () => {
    DynamicBindingAPI.addCustomBinding({
      propertyPath: 'component.tenantId',
      paramName: 'tenant_id',
      required: true,
      transform: value => String(value).toUpperCase(),
      description: 'tenant binding'
    })
    DynamicBindingAPI.addCustomTrigger({
      propertyPath: 'component.tenantId',
      debounceMs: 0,
      description: 'tenant trigger'
    })

    const binding = DynamicBindingAPI.getCurrentBindingRules().find(rule => rule.propertyPath === 'component.tenantId')
    const trigger = DynamicBindingAPI.getCurrentTriggerRules().find(rule => rule.propertyPath === 'component.tenantId')

    expect(binding).toMatchObject({
      paramName: 'tenant_id',
      required: true,
      description: 'tenant binding'
    })
    expect(binding?.transform?.('tenant-a')).toBe('TENANT-A')
    expect(trigger).toMatchObject({
      enabled: true,
      debounceMs: 0,
      description: 'tenant trigger'
    })

    expect(DynamicBindingAPI.removeBinding('component.tenantId')).toBe(true)
    expect(DynamicBindingAPI.removeTrigger('component.tenantId')).toBe(true)
    expect(DynamicBindingAPI.getCurrentBindingRules().some(rule => rule.propertyPath === 'component.tenantId')).toBe(false)
    expect(DynamicBindingAPI.getCurrentTriggerRules().some(rule => rule.propertyPath === 'component.tenantId')).toBe(false)
  })

  it('registers component-specific binding and trigger rules without leaking them to other component types', () => {
    DynamicBindingAPI.configureCustomComponent('unit-rdi-device-operations', {
      bindings: [
        {
          propertyPath: 'component.metric',
          paramName: 'metric_name',
          required: true
        }
      ],
      triggers: [
        {
          propertyPath: 'component.metric',
          debounceMs: 0
        }
      ],
      autoBind: {
        enabled: true,
        mode: 'custom',
        customRules: []
      }
    })

    expect(DynamicBindingAPI.getCurrentBindingRules().some(rule => rule.paramName === 'metric_name')).toBe(false)
    expect(DynamicBindingAPI.getCurrentBindingRules('unit-rdi-device-operations')).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          propertyPath: 'component.metric',
          paramName: 'metric_name',
          required: true
        })
      ])
    )
    expect(DynamicBindingAPI.getCurrentTriggerRules('unit-rdi-device-operations')).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          propertyPath: 'component.metric',
          enabled: true,
          debounceMs: 0
        })
      ])
    )
  })

  it('applies built-in templates as removable runtime rules', () => {
    DynamicBindingAPI.applyTemplate('iot-device')

    const bindings = DynamicBindingAPI.getCurrentBindingRules()
    const sensorBinding = bindings.find(rule => rule.propertyPath === 'component.sensorIds')

    expect(bindings).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ propertyPath: 'base.deviceId', paramName: 'device_id', required: true }),
        expect.objectContaining({ propertyPath: 'base.deviceType', paramName: 'device_type' })
      ])
    )
    expect(sensorBinding?.transform?.(['temperature', 'humidity'])).toBe('temperature,humidity')
    expect(DynamicBindingAPI.getCurrentTriggerRules()).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ propertyPath: 'base.deviceId', enabled: true, debounceMs: 50 }),
        expect.objectContaining({ propertyPath: 'component.sensorIds', enabled: true, debounceMs: 200 })
      ])
    )

    DynamicBindingAPI.applyTemplate('custom')

    expect(DynamicBindingAPI.getCurrentBindingRules()).toEqual([])
    expect(DynamicBindingAPI.getCurrentTriggerRules()).toEqual([])
    expect(DynamicBindingAPI.getSystemStatus()).toMatchObject({
      totalBindingRules: 0,
      totalTriggerRules: 0,
      hasDefaultRules: false,
      isFullyCustomized: true
    })
  })

  it('removes device defaults through the same public API used by runtime configuration', () => {
    DynamicBindingAPI.applyTemplate('iot-device')
    DynamicBindingAPI.removeBinding('base.deviceId')
    DynamicBindingAPI.removeTrigger('base.deviceId')

    const remainingBindings = DynamicBindingAPI.getCurrentBindingRules()
    const remainingTriggers = DynamicBindingAPI.getCurrentTriggerRules()

    expect(remainingBindings.some(rule => rule.propertyPath === 'base.deviceId')).toBe(false)
    expect(remainingTriggers.some(rule => rule.propertyPath === 'base.deviceId')).toBe(false)
    expect(remainingBindings.length).toBeGreaterThan(0)
    expect(remainingTriggers.length).toBeGreaterThan(0)
    expect(DynamicBindingAPI.getSystemStatus()).toMatchObject({
      hasDefaultRules: true,
      isFullyCustomized: false
    })
  })
})
