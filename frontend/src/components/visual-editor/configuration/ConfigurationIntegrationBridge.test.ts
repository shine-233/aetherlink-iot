import { describe, expect, it } from 'vitest'
import type { ConfigurationIntegrationBridge } from './ConfigurationIntegrationBridge'
import { configurationIntegrationBridge } from './ConfigurationIntegrationBridge'

const createComponentId = (name: string) => `configuration-bridge-test-${name}`

describe('ConfigurationIntegrationBridge', () => {
  it('distinguishes an uninitialized component from an initialized default configuration', () => {
    const componentId = createComponentId('initialize')

    expect(configurationIntegrationBridge.getConfiguration(componentId)).toBeNull()
    expect(configurationIntegrationBridge.initializeConfiguration(componentId, {
      base: { title: 'Local widget' },
      customize: { metric: 'temperature' }
    })).toEqual({
      base: { title: 'Local widget' },
      component: {},
      dataSource: null,
      interaction: {},
      metadata: {},
      customize: { metric: 'temperature' }
    })
  })

  it('keeps existing initialization and merges section updates', () => {
    const componentId = createComponentId('merge')

    configurationIntegrationBridge.initializeConfiguration(componentId, {
      base: { title: 'Original' },
      customize: { metric: 'temperature' }
    })
    expect(configurationIntegrationBridge.initializeConfiguration(componentId, {
      base: { title: 'Ignored reinitialization' }
    })).toMatchObject({ base: { title: 'Original' } })

    const result = configurationIntegrationBridge.updateConfiguration(componentId, 'customize', {
      unit: '°C'
    })

    expect(result).toEqual({ success: true, persisted: false, scope: 'runtime' })
    expect(configurationIntegrationBridge.getConfiguration(componentId)?.customize).toEqual({
      metric: 'temperature',
      unit: '°C'
    })
  })

  it('replaces a configuration locally without claiming durable persistence', () => {
    const componentId = createComponentId('replace')

    const result = configurationIntegrationBridge.setConfiguration(componentId, {
      component: { properties: { color: 'blue' } }
    })

    expect(result).toEqual({ success: true, persisted: false, scope: 'runtime' })
    expect(configurationIntegrationBridge.getConfiguration(componentId)).toEqual({
      base: {},
      component: { properties: { color: 'blue' } },
      dataSource: null,
      interaction: {},
      metadata: {},
      customize: {}
    })
  })

  it('preserves the exported class contract for compatibility adapters', () => {
    const bridge: ConfigurationIntegrationBridge = configurationIntegrationBridge
    expect(bridge).toBe(configurationIntegrationBridge)
  })
})
