import { describe, expect, it } from 'vitest'

import { SimpleConfigGenerator } from './SimpleConfigGenerator'

describe('SimpleConfigGenerator', () => {
  it('generates a required static source using the Card2.1 key', () => {
    const generator = new SimpleConfigGenerator()
    const requirement = {
      componentId: 'component-1',
      componentName: 'Test component',
      dataSources: [
        {
          key: 'telemetry',
          name: 'Telemetry',
          description: 'Device telemetry',
          supportedTypes: ['static' as const],
          fieldMappings: {},
          required: true,
          structureType: 'object' as const,
          fields: [
            { name: 'temperature', type: 'number' as const, required: false, description: 'Temperature' }
          ]
        }
      ]
    }

    const config = generator.generateConfig(requirement, [
      { dataSourceId: 'telemetry', type: 'static', config: { data: { temperature: 25 } } }
    ])

    expect(config.dataSources).toEqual([
      {
        id: 'telemetry',
        type: 'static',
        config: { data: { temperature: 25 } },
        fieldMapping: { temperature: 'temperature' }
      }
    ])
    expect(config.triggers).toEqual([{ type: 'manual', config: {} }])
  })

  it('keeps supporting the legacy source id', () => {
    const generator = new SimpleConfigGenerator()
    const requirement = {
      componentId: 'component-1',
      componentName: 'Test component',
      dataSources: [
        {
          key: 'telemetry',
          id: 'legacy-telemetry',
          name: 'Telemetry',
          description: 'Device telemetry',
          supportedTypes: ['static' as const],
          fieldMappings: {},
          required: true
        }
      ]
    }

    expect(() =>
      generator.generateConfig(requirement, [
        { dataSourceId: 'legacy-telemetry', type: 'static', config: { data: {} } }
      ])
    ).not.toThrow()
  })

  it('creates a WebSocket trigger from the typed configuration', () => {
    const generator = new SimpleConfigGenerator()
    const requirement = {
      componentId: 'component-1',
      componentName: 'Test component',
      dataSources: [
        {
          key: 'live',
          name: 'Live data',
          description: 'Live data',
          supportedTypes: ['websocket' as const],
          fieldMappings: {}
        }
      ]
    }

    const input = {
      dataSourceId: 'live',
      type: 'websocket' as const,
      config: { url: 'wss://example.test', protocols: ['json'] }
    }
    const config = generator.generateConfig(requirement, [input])

    expect(config.triggers).toEqual([
      { type: 'websocket', config: { url: 'wss://example.test', protocols: ['json'] } }
    ])

    const sourceConfig = config.dataSources[0].config as typeof input.config
    const triggerConfig = config.triggers[0].config
    sourceConfig.protocols.push('source-only')
    triggerConfig.protocols?.push('trigger-only')

    expect(input.config.protocols).toEqual(['json'])
    expect(sourceConfig.protocols).toEqual(['json', 'source-only'])
    expect(triggerConfig.protocols).toEqual(['json', 'trigger-only'])
  })

  it('rejects a missing required source input', () => {
    const generator = new SimpleConfigGenerator()
    const requirement = {
      componentId: 'component-1',
      componentName: 'Test component',
      dataSources: [
        {
          key: 'required-source',
          name: 'Required source',
          description: 'Required source',
          supportedTypes: ['api' as const],
          fieldMappings: {},
          required: true
        }
      ]
    }

    expect(() =>
      generator.generateConfig(requirement, [
        { dataSourceId: 'other', type: 'api', config: { url: 'https://example.test', method: 'GET' } }
      ])
    ).toThrow('缺少必需的数据源配置: Required source')
  })

  it('extracts nested object properties and numeric array indexes', () => {
    const generator = new SimpleConfigGenerator()
    const sourceData = {
      device: {
        telemetry: [{ temperature: 25 }, { temperature: 27 }]
      }
    }

    expect(
      generator.previewMapping(sourceData, {
        firstTemperature: 'device.telemetry[0].temperature',
        secondTemperature: 'device.telemetry.1.temperature'
      })
    ).toEqual([
      {
        targetField: 'firstTemperature',
        sourcePath: 'device.telemetry[0].temperature',
        mappedValue: 25,
        success: true
      },
      {
        targetField: 'secondTemperature',
        sourcePath: 'device.telemetry.1.temperature',
        mappedValue: 27,
        success: true
      }
    ])
  })

  it.each([
    ['zero', 0],
    ['disabled', false],
    ['emptyText', '']
  ])('preserves the falsy mapped value at %s', (sourcePath, mappedValue) => {
    const generator = new SimpleConfigGenerator()

    expect(generator.previewMapping({ [sourcePath]: mappedValue }, { output: sourcePath })).toEqual([
      {
        targetField: 'output',
        sourcePath,
        mappedValue,
        success: true
      }
    ])
  })

  it.each(['device.getValue()', 'items[0 + 1]', 'device.__proto__.polluted', 'device.constructor.prototype']) (
    'rejects the executable or prototype path %s',
    sourcePath => {
      const generator = new SimpleConfigGenerator()
      const [result] = generator.previewMapping({ device: {} }, { output: sourcePath })

      expect(result).toEqual({
        targetField: 'output',
        sourcePath,
        mappedValue: null,
        success: false,
        error: `无法解析路径: ${sourcePath}`
      })
    }
  )

  it('returns undefined for a valid path whose value is absent', () => {
    const generator = new SimpleConfigGenerator()

    expect(generator.previewMapping({ device: {} }, { output: 'device.temperature' })).toEqual([
      {
        targetField: 'output',
        sourcePath: 'device.temperature',
        mappedValue: undefined,
        success: true
      }
    ])
  })
})
