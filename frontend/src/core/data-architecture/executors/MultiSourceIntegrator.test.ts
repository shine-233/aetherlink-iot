import { afterEach, describe, expect, it, vi } from 'vitest'

import { MultiSourceIntegrator, type DataSourceResult } from './MultiSourceIntegrator'

describe('MultiSourceIntegrator', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('integrates successful and failed sources with one consistent timestamp', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-05T12:34:56.789Z'))
    const integrator = new MultiSourceIntegrator()

    const result = await integrator.integrateDataSources(
      [
        { sourceId: 'sensor', type: 'json', data: { value: 26 }, success: true },
        { sourceId: 'external', type: 'websocket', data: { stale: true }, success: false, error: 'WS_EXTERNAL_BLOCKED' }
      ],
      'card-1'
    )

    expect(result.sensor).toEqual({
      type: 'json',
      data: { value: 26 },
      lastUpdated: Date.now(),
      metadata: {
        componentId: 'card-1',
        success: true,
        error: undefined,
        processedAt: '2026-08-05T12:34:56.789Z'
      }
    })
    expect(result.external).toEqual({
      type: 'websocket',
      data: null,
      lastUpdated: Date.now(),
      metadata: {
        componentId: 'card-1',
        success: false,
        error: 'WS_EXTERNAL_BLOCKED',
        processedAt: '2026-08-05T12:34:56.789Z'
      }
    })
  })

  it('does not expose the successful source payload by reference', async () => {
    const integrator = new MultiSourceIntegrator()
    const payload = { nested: { value: 1 } }

    const result = await integrator.integrateDataSources(
      [{ sourceId: 'sensor', type: 'json', data: payload, success: true }],
      'card-1'
    )

    result.sensor.data.nested.value = 2

    expect(payload).toEqual({ nested: { value: 1 } })
  })

  it.each(['', '__proto__', 'prototype', 'constructor'])('skips the unsafe source id %j', async sourceId => {
    const integrator = new MultiSourceIntegrator()
    const source: DataSourceResult = { sourceId, type: 'json', data: { polluted: true }, success: true }

    const result = await integrator.integrateDataSources([source], 'card-1')

    expect(result).toEqual({})
    expect(Object.getPrototypeOf(result)).toBe(Object.prototype)
    expect(integrator.validateDataSourceResult(source)).toBe(false)
  })

  it('merges only newer safe updates without exposing the update entry by reference', () => {
    const integrator = new MultiSourceIntegrator()
    const existing = {
      sensor: { type: 'json', data: { value: 1 }, lastUpdated: 10 }
    }
    const newerUpdate = { type: 'json', data: { value: 2 }, lastUpdated: 20 }
    const updates = {
      sensor: newerUpdate,
      stale: { type: 'json', data: { value: 3 }, lastUpdated: 5 }
    }

    const result = integrator.mergeComponentData(existing, updates)
    result.sensor.data.value = 99

    expect(newerUpdate.data).toEqual({ value: 2 })
    expect(existing.sensor.data).toEqual({ value: 1 })
    expect(result.stale).toEqual(updates.stale)
  })

  it.each(['__proto__', 'prototype', 'constructor'])('skips the unsafe incremental source id %s', sourceId => {
    const integrator = new MultiSourceIntegrator()
    const updates = Object.create(null)
    updates[sourceId] = { type: 'json', data: { polluted: true }, lastUpdated: 20 }

    const result = integrator.mergeComponentData({}, updates)

    expect(result).toEqual({})
    expect(Object.getPrototypeOf(result)).toBe(Object.prototype)
  })

  it('cleans expired and unsafe entries without exposing retained data by reference', () => {
    vi.useFakeTimers()
    vi.setSystemTime(1_000)
    const integrator = new MultiSourceIntegrator()
    const componentData = Object.create(null)
    componentData.current = { type: 'json', data: { value: 1 }, lastUpdated: 900 }
    componentData.expired = { type: 'json', data: { value: 2 }, lastUpdated: 100 }
    componentData.prototype = { type: 'json', data: { polluted: true }, lastUpdated: 900 }

    const result = integrator.cleanupExpiredData(componentData, 200)
    result.current.data.value = 99

    expect(result).toEqual({ current: { type: 'json', data: { value: 99 }, lastUpdated: 900 } })
    expect(componentData.current.data).toEqual({ value: 1 })
  })

  it('creates an isolated Visual Editor payload and skips unsafe source ids', () => {
    const integrator = new MultiSourceIntegrator()
    const componentData = Object.create(null)
    componentData.sensor = { type: 'json', data: { value: 1 }, lastUpdated: 10 }
    componentData.constructor = { type: 'json', data: { polluted: true }, lastUpdated: 10 }

    const result = integrator.toVisualEditorFormat(componentData)
    result.sensor.value = 99

    expect(result).toEqual({ sensor: { value: 99 } })
    expect(componentData.sensor.data).toEqual({ value: 1 })
  })

  it('serializes safe Card 2.1 sources independently and skips circular or unsafe entries', () => {
    const integrator = new MultiSourceIntegrator()
    const circular: Record<string, any> = {}
    circular.self = circular
    const componentData = Object.create(null)
    componentData.sensor = { type: 'json', data: { value: 1 }, lastUpdated: 10 }
    componentData.circular = { type: 'json', data: circular, lastUpdated: 10 }
    componentData.__proto__ = { type: 'json', data: { polluted: true }, lastUpdated: 10 }

    expect(integrator.toCard21Format(componentData)).toEqual({
      rawDataSources: {
        dataSourceBindings: {
          sensor: { rawData: '{"value":1}' }
        }
      }
    })
  })
})
