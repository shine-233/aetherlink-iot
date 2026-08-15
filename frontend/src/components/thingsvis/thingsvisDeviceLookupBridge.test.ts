import { describe, expect, it, vi } from 'vitest'

import { findPlatformDeviceById, loadPlatformDevicesByGroup } from './thingsvisDeviceLookupBridge'

describe('thingsvisDeviceLookupBridge', () => {
  it('finds a device from the initial search result before broader fallbacks', async () => {
    const loadDevicesByGroup = vi.fn()

    const result = await findPlatformDeviceById({
      deviceId: 'dev-1',
      loadGroups: vi.fn().mockResolvedValue([{ groupId: 'group-a', groupName: 'Workshop A' }]),
      searchDevices: vi.fn().mockResolvedValue([{ device_id: 'dev-1' }]),
      listDevices: vi.fn().mockResolvedValue([{ device_id: 'dev-2' }]),
      mapDevicesForGroup: vi.fn(async (rows) =>
        rows.map((row: any) => ({
          deviceId: row.device_id
        }))
      ),
      loadDevicesByGroup
    })

    expect(result).toEqual({ deviceId: 'dev-1' })
    expect(loadDevicesByGroup).not.toHaveBeenCalled()
  })

  it('falls back to grouped device loaders when search and list do not contain the device', async () => {
    const loadDevicesByGroup = vi
      .fn()
      .mockResolvedValueOnce([{ deviceId: 'dev-404' }])
      .mockResolvedValueOnce([{ deviceId: 'dev-2' }])

    const result = await findPlatformDeviceById({
      deviceId: 'dev-2',
      loadGroups: vi.fn().mockResolvedValue([
        { groupId: 'group-a', groupName: 'Workshop A' },
        { groupId: 'group-b', groupName: 'Workshop B' }
      ]),
      searchDevices: vi.fn().mockResolvedValue([]),
      listDevices: vi.fn().mockResolvedValue([]),
      mapDevicesForGroup: vi.fn(async (rows) =>
        rows.map((row: any) => ({
          deviceId: row.device_id
        }))
      ),
      loadDevicesByGroup
    })

    expect(result).toEqual({ deviceId: 'dev-2' })
    expect(loadDevicesByGroup).toHaveBeenCalledTimes(2)
    expect(loadDevicesByGroup).toHaveBeenNthCalledWith(1, 'group-a')
    expect(loadDevicesByGroup).toHaveBeenNthCalledWith(2, 'group-b')
  })

  it('returns null for blank device ids', async () => {
    const result = await findPlatformDeviceById({
      deviceId: '   ',
      loadGroups: vi.fn(),
      searchDevices: vi.fn(),
      listDevices: vi.fn(),
      mapDevicesForGroup: vi.fn(),
      loadDevicesByGroup: vi.fn()
    })

    expect(result).toBeNull()
  })

  it('maps grouped relation devices when the group already has direct members', async () => {
    const mapDevicesForGroup = vi.fn().mockResolvedValue([{ deviceId: 'dev-1', groupId: 'group-a' }])
    const buildFallbackDevices = vi.fn()

    const result = await loadPlatformDevicesByGroup({
      groupId: 'group-a',
      loadGroups: vi.fn().mockResolvedValue([{ groupId: 'group-a', groupName: 'Workshop A' }]),
      loadRelatedDevices: vi.fn().mockResolvedValue([{ device_id: 'dev-1' }]),
      mapDevicesForGroup,
      buildFallbackDevices,
      normalizeGroupId: (groupId) => groupId.trim(),
      normalizeGroupName: (_groupName, fallbackId) => fallbackId || 'Ungrouped'
    })

    expect(result).toEqual([{ deviceId: 'dev-1', groupId: 'group-a' }])
    expect(mapDevicesForGroup).toHaveBeenCalledWith([{ device_id: 'dev-1' }], 'group-a', 'Workshop A', [
      { groupId: 'group-a', groupName: 'Workshop A' }
    ])
    expect(buildFallbackDevices).not.toHaveBeenCalled()
  })

  it('falls back to default-group assembly when relation devices are empty', async () => {
    const result = await loadPlatformDevicesByGroup({
      groupId: 'group-root',
      loadGroups: vi.fn().mockResolvedValue([{ groupId: 'group-root', groupName: 'Root group' }]),
      loadRelatedDevices: vi.fn().mockResolvedValue([]),
      mapDevicesForGroup: vi.fn(),
      buildFallbackDevices: vi.fn().mockResolvedValue([{ deviceId: 'dev-fallback' }]),
      normalizeGroupId: (groupId) => groupId,
      normalizeGroupName: (_groupName, fallbackId) => fallbackId || 'Ungrouped'
    })

    expect(result).toEqual([{ deviceId: 'dev-fallback' }])
  })

  it('returns empty results and reports grouped-load errors', async () => {
    const onError = vi.fn()

    const result = await loadPlatformDevicesByGroup({
      groupId: 'group-a',
      loadGroups: vi.fn().mockRejectedValue(new Error('boom')),
      loadRelatedDevices: vi.fn().mockResolvedValue([]),
      mapDevicesForGroup: vi.fn(),
      buildFallbackDevices: vi.fn(),
      normalizeGroupId: (groupId) => groupId,
      normalizeGroupName: (_groupName, fallbackId) => fallbackId || 'Ungrouped',
      onError
    })

    expect(result).toEqual([])
    expect(onError).toHaveBeenCalledWith('group-a', expect.any(Error))
  })
})
