import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { useEntityRelationDataLoader } from '../useEntityRelationDataLoader'

const mockListEntityRelations = vi.fn()
const mockTelemetryDataCurrent = vi.fn()

vi.mock('@/service/api/entity-relation', () => ({
  listEntityRelations: (...args: any[]) => mockListEntityRelations(...args)
}))

vi.mock('@/service/api/device-telemetry-twin-api', () => ({
  telemetryDataCurrent: (...args: any[]) => mockTelemetryDataCurrent(...args)
}))

describe('useEntityRelationDataLoader', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns empty fields when dashboard has no entity relation widgets', async () => {
    const dashboard = ref({
      version: 1,
      columns: 12,
      rowHeight: 60,
      widgets: [
        {
          id: 'w1',
          type: 'text',
          config: { text: 'Plain Text' }
        }
      ]
    })

    const { fields, loading, error } = useEntityRelationDataLoader(dashboard)
    await Promise.resolve()

    expect(fields.value).toEqual({})
    expect(loading.value).toBe(false)
    expect(error.value).toBe(null)
    expect(mockListEntityRelations).not.toHaveBeenCalled()
  })

  it('loads relations and aggregates telemetry for metric widgets with entityRelation', async () => {
    mockListEntityRelations.mockResolvedValue({
      ok: true,
      data: {
        list: [
          { from_type: 'device', from_id: 'gw-01', relation_type: 'Contains', to_type: 'device', to_id: 'sensor-01' },
          { from_type: 'device', from_id: 'gw-01', relation_type: 'Contains', to_type: 'device', to_id: 'sensor-02' }
        ],
        total: 2
      }
    })

    mockTelemetryDataCurrent.mockImplementation(async (id: string) => {
      if (id === 'sensor-01') {
        return {
          ok: true,
          data: [{ key: 'temperature', value: 20.0 }]
        }
      }
      if (id === 'sensor-02') {
        return {
          ok: true,
          data: [{ key: 'temperature', value: 26.0 }]
        }
      }
      return { ok: true, data: [] }
    })

    const dashboard = ref({
      version: 1,
      columns: 12,
      rowHeight: 60,
      widgets: [
        {
          id: 'metric_rel',
          type: 'metric',
          config: {
            label: 'Average Child Temp',
            entityRelation: {
              enabled: true,
              rootType: 'device',
              rootId: 'gw-01',
              direction: 'from',
              relationType: 'Contains',
              targetType: 'device',
              targetKey: 'temperature',
              aggregation: 'avg'
            }
          }
        }
      ]
    })

    const { fields, refresh } = useEntityRelationDataLoader(dashboard)
    await refresh()

    const expectedKey = '__rel_device_gw-01_from_Contains_temperature'
    expect(fields.value[expectedKey]).toBe(23.0)
    expect(mockListEntityRelations).toHaveBeenCalledWith(
      expect.objectContaining({ relation_type: 'Contains', from_type: 'device', from_id: 'gw-01' })
    )
    expect(mockTelemetryDataCurrent).toHaveBeenCalledWith('sensor-01')
    expect(mockTelemetryDataCurrent).toHaveBeenCalledWith('sensor-02')
  })

  it('populates series arrays for chart widgets with entityRelation', async () => {
    mockListEntityRelations.mockResolvedValue({
      ok: true,
      data: {
        list: [
          { from_type: 'device', from_id: 'gw-01', relation_type: 'Monitors', to_type: 'device', to_id: 'meter-a' },
          { from_type: 'device', from_id: 'gw-01', relation_type: 'Monitors', to_type: 'device', to_id: 'meter-b' }
        ],
        total: 2
      }
    })

    mockTelemetryDataCurrent.mockImplementation(async (id: string) => {
      if (id === 'meter-a') {
        return {
          ok: true,
          data: [{ key: 'voltage', value: 220 }]
        }
      }
      if (id === 'meter-b') {
        return {
          ok: true,
          data: [{ key: 'voltage', value: 230 }]
        }
      }
      return { ok: true, data: [] }
    })

    const dashboard = ref({
      version: 1,
      columns: 12,
      rowHeight: 60,
      widgets: [
        {
          id: 'chart_rel',
          type: 'line-chart',
          config: {
            title: 'Voltage by Monitor',
            entityRelation: {
              enabled: true,
              rootType: 'device',
              rootId: 'gw-01',
              direction: 'from',
              relationType: 'Monitors',
              targetType: 'device',
              targetKey: 'voltage'
            }
          }
        }
      ]
    })

    const { fields, refresh } = useEntityRelationDataLoader(dashboard)
    await refresh()

    const expectedKey = '__rel_device_gw-01_from_Monitors_voltage'
    expect(fields.value[expectedKey]).toEqual([220, 230])
    expect(fields.value[`${expectedKey}_cats`]).toEqual(['meter-a', 'meter-b'])
  })

  it('handles API errors gracefully without throwing', async () => {
    mockListEntityRelations.mockRejectedValue(new Error('Network error'))

    const dashboard = ref({
      version: 1,
      columns: 12,
      rowHeight: 60,
      widgets: [
        {
          id: 'error_metric',
          type: 'metric',
          config: {
            label: 'Temp',
            entityRelation: {
              enabled: true,
              rootType: 'device',
              rootId: 'bad-gw',
              direction: 'from',
              relationType: 'Contains',
              targetType: 'device',
              targetKey: 'temp'
            }
          }
        }
      ]
    })

    const { fields, refresh } = useEntityRelationDataLoader(dashboard)
    await refresh()

    const expectedKey = '__rel_device_bad-gw_from_Contains_temp'
    expect(fields.value[expectedKey]).toBeNull()
  })
})
