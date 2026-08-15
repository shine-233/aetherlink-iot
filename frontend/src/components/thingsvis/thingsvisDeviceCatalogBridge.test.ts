import { describe, expect, it, vi } from 'vitest'

import { loadDeviceFilterOptions, loadPlatformDeviceGroups } from './thingsvisDeviceCatalogBridge'

const firstString = (...values: unknown[]) => {
  for (const value of values) {
    if (value === undefined || value === null) continue
    const normalized = String(value).trim()
    if (normalized) return normalized
  }
  return undefined
}

const asRecord = (value: unknown) =>
  value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, any>) : {}

describe('thingsvisDeviceCatalogBridge', () => {
  it('builds device filter options from config and service catalogs', async () => {
    const result = await loadDeviceFilterOptions({
      loadDeviceConfigs: vi.fn().mockResolvedValue({
        data: {
          list: [{ id: 'cfg-1', name: 'Config 1' }]
        }
      }),
      loadServiceCatalog: vi.fn().mockResolvedValue({
        data: {
          protocol: [{ service_identifier: 'mqtt', name: 'MQTT' }],
          service: [{ service_identifier: 'rdi', name: 'RDI' }]
        }
      }),
      unwrapList: (payload) => (Array.isArray(payload?.list) ? payload.list : []),
      firstString,
      asRecord,
      protocolLabelPrefix: '协议：',
      serviceLabelPrefix: '服务：'
    })

    expect(result).toEqual({
      deviceConfigs: [{ value: 'cfg-1', label: 'Config 1' }],
      serviceOptions: [
        { value: 'mqtt', label: '协议：MQTT' },
        { value: 'rdi', label: '服务：RDI' }
      ]
    })
  })

  it('prepends the all-device group entry to flattened group trees', async () => {
    const result = await loadPlatformDeviceGroups({
      loadGroupTree: vi.fn().mockResolvedValue({
        data: [{ id: 'group-a', name: 'Workshop A' }]
      }),
      flattenDeviceGroupTree: (rows) =>
        rows.map((row: any) => ({
          groupId: row.id,
          groupName: row.name
        })),
      allGroupLabel: 'All device groups'
    })

    expect(result).toEqual([
      { groupId: '__all__', groupName: 'All device groups', parentId: null },
      { groupId: 'group-a', groupName: 'Workshop A' }
    ])
  })

  it('returns empty groups and reports load errors', async () => {
    const onError = vi.fn()

    const result = await loadPlatformDeviceGroups({
      loadGroupTree: vi.fn().mockRejectedValue(new Error('boom')),
      flattenDeviceGroupTree: () => [],
      allGroupLabel: 'All device groups',
      onError
    })

    expect(result).toEqual([])
    expect(onError).toHaveBeenCalled()
  })
})
