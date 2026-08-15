import { describe, expect, it, vi } from 'vitest'

import {
  collectPlatformSourceDeviceIds,
  groupPlatformSourceDescriptorsByDevice,
  hydratePlatformSourceDescriptorGroup,
  pickRequestedPlatformFields
} from './thingsvisFieldHydrationBridge'

describe('thingsvisFieldHydrationBridge', () => {
  it('picks only the requested fields from a payload', () => {
    expect(pickRequestedPlatformFields({ temp: 31, setpoint: 40, ignored: true }, ['setpoint', 'temp'])).toEqual({
      setpoint: 40,
      temp: 31
    })
  })

  it('groups descriptors by device and merges requested field ids', () => {
    const grouped = groupPlatformSourceDescriptorsByDevice([
      { id: 'ds-1', deviceId: 'dev-1', requestedFields: ['temp'] },
      { id: 'ds-2', deviceId: 'dev-1', requestedFields: ['setpoint'] },
      { id: 'ds-3', deviceId: 'dev-2', requestedFields: ['speed'] },
      { id: 'ds-4', requestedFields: ['ignored'] }
    ])

    expect(Array.from(grouped.keys())).toEqual(['dev-1', 'dev-2'])
    expect(Array.from(grouped.get('dev-1')!.requestedFields)).toEqual(['temp', 'setpoint'])
    expect(grouped.get('dev-1')!.descriptors.map((descriptor) => descriptor.id)).toEqual(['ds-1', 'ds-2'])
  })

  it('collects unique device ids in stable order', () => {
    expect(
      collectPlatformSourceDeviceIds([
        { id: 'ds-1', deviceId: 'dev-1', requestedFields: [] },
        { id: 'ds-2', deviceId: 'dev-1', requestedFields: [] },
        { id: 'ds-3', deviceId: 'dev-2', requestedFields: [] },
        { id: 'ds-4', requestedFields: [] }
      ])
    ).toEqual(['dev-1', 'dev-2'])
  })

  it('hydrates a grouped descriptor set and posts scoped data-source payloads', async () => {
    const ensureDeviceWs = vi.fn()
    const ensureDeviceStatusWs = vi.fn()
    const postPlatformData = vi.fn()

    await hydratePlatformSourceDescriptorGroup({
      deviceId: 'dev-1',
      group: {
        descriptors: [
          { id: 'ds-1', deviceId: 'dev-1', requestedFields: ['temp'] },
          { id: 'ds-2', deviceId: 'dev-1', requestedFields: ['setpoint'] }
        ],
        requestedFields: new Set(['temp', 'setpoint'])
      },
      ensureDeviceWs,
      ensureDeviceStatusWs,
      loadRequestedFieldData: vi.fn().mockResolvedValue({ temp: 31, setpoint: 40, ignored: true }),
      postPlatformData
    })

    expect(ensureDeviceWs).toHaveBeenCalledWith('dev-1')
    expect(ensureDeviceStatusWs).toHaveBeenCalledWith('dev-1')
    expect(postPlatformData).toHaveBeenNthCalledWith(1, { temp: 31 }, 'dev-1', 'ds-1')
    expect(postPlatformData).toHaveBeenNthCalledWith(2, { setpoint: 40 }, 'dev-1', 'ds-2')
  })

  it('skips hydration when the group has no requested fields', async () => {
    const loadRequestedFieldData = vi.fn()
    const postPlatformData = vi.fn()

    await hydratePlatformSourceDescriptorGroup({
      deviceId: 'dev-1',
      group: {
        descriptors: [{ id: 'ds-1', deviceId: 'dev-1', requestedFields: [] }],
        requestedFields: new Set()
      },
      loadRequestedFieldData,
      postPlatformData
    })

    expect(loadRequestedFieldData).not.toHaveBeenCalled()
    expect(postPlatformData).not.toHaveBeenCalled()
  })
})
