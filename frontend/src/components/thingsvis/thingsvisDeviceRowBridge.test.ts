import { describe, expect, it } from 'vitest'

import { normalizePlatformDeviceRow } from './thingsvisDeviceRowBridge'

describe('thingsvisDeviceRowBridge', () => {
  it('normalizes nested device rows with config and template fallbacks', () => {
    const row = {
      device: {
        id: 'dev-1',
        device_name: 'Pump 1',
        device_config_id: 'cfg-1',
        group_id: 'group-a',
        is_online: 1,
        warn_status: 'warning',
        device_type: 'sensor',
        access_way: 'direct',
        protocol_type: 'mqtt',
        last_push_time: '2026-07-04 10:00:00'
      }
    }

    expect(
      normalizePlatformDeviceRow(row, {
        fallbackGroupId: '',
        fallbackGroupName: '',
        groupNameById: new Map([['group-a', 'Workshop A']]),
        configTemplateMap: new Map([['cfg-1', 'tpl-1']])
      })
    ).toEqual({
      deviceId: 'dev-1',
      deviceName: 'Pump 1',
      groupId: 'group-a',
      groupName: 'Workshop A',
      deviceConfigId: 'cfg-1',
      isOnline: 1,
      warnStatus: 'warning',
      deviceType: 'sensor',
      accessWay: 'direct',
      protocolType: 'mqtt',
      lastPushTime: '2026-07-04 10:00:00',
      templateId: 'tpl-1'
    })
  })

  it('uses fallback group name and boolean online state when source data is sparse', () => {
    const row = {
      device_id: 'dev-2',
      device_number: 'SN-2',
      is_online: true
    }

    expect(
      normalizePlatformDeviceRow(row, {
        fallbackGroupId: '__ungrouped__',
        fallbackGroupName: 'Ungrouped',
        groupNameById: new Map(),
        configTemplateMap: new Map()
      })
    ).toEqual({
      deviceId: 'dev-2',
      deviceName: 'SN-2',
      groupId: '__ungrouped__',
      groupName: 'Ungrouped',
      isOnline: true
    })
  })

  it('returns null when a row has no resolvable device id', () => {
    expect(
      normalizePlatformDeviceRow(
        { name: 'missing-id' },
        {
          fallbackGroupId: '',
          fallbackGroupName: '',
          groupNameById: new Map(),
          configTemplateMap: new Map()
        }
      )
    ).toBeNull()
  })
})
