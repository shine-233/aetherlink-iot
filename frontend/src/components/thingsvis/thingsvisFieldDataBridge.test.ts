import { describe, expect, it, vi } from 'vitest'

import {
  buildRequestedAlarmStatusData,
  buildRequestedFieldData,
  loadCurrentFieldValueMap
} from './thingsvisFieldDataBridge'

const silentRequestConfig = { silentError: true }

describe('thingsvisFieldDataBridge', () => {
  it('loads current telemetry, attributes, and RDI metadata into one kv map', async () => {
    const result = await loadCurrentFieldValueMap({
      deviceId: 'dev-1',
      currentFieldIds: ['temp', 'setpoint', 'firmware_version'],
      rdiMetaFieldIds: new Set(['firmware_version']),
      silentRequestConfig,
      loadTelemetryCurrent: vi.fn().mockResolvedValue({ data: [{ key: 'temp', value: 31 }] }),
      loadAttributeDataSet: vi.fn().mockResolvedValue({ data: [{ key: 'setpoint', value: 40, label: 'SP' }] }),
      loadRdiDeviceConfig: vi
        .fn()
        .mockResolvedValue({ data: { device: { firmware_version: '1.2.3', pid_number: 'PID-9' } } })
    })

    expect(result).toEqual({
      temp: 31,
      setpoint: 40,
      SP: 40,
      firmware_version: '1.2.3',
      pid_number: 'PID-9'
    })
  })

  it('throws when current field loading fails so host bridges can surface structured errors', async () => {
    await expect(
      loadCurrentFieldValueMap({
        deviceId: 'dev-1',
        currentFieldIds: ['temp'],
        rdiMetaFieldIds: new Set(['firmware_version']),
        silentRequestConfig,
        loadTelemetryCurrent: vi.fn().mockRejectedValue(new Error('telemetry failed')),
        loadAttributeDataSet: vi.fn().mockResolvedValue({ data: [] }),
        loadRdiDeviceConfig: vi.fn().mockResolvedValue({ data: { device: {} } })
      })
    ).rejects.toThrow('telemetry failed')
  })

  it('builds alarm-status fields from the latest and highest active rows', async () => {
    const result = await buildRequestedAlarmStatusData({
      deviceId: 'dev-1',
      fieldIds: ['device_alarm_count', 'device_alarm_highest_level', 'latest_device_alarm_title'],
      loadDeviceAlarmStatus: vi.fn().mockResolvedValue({
        data: {
          total: 2,
          list: [
            { alarm_status: 'active', alarm_level: 'warning', alarm_name: 'Recent warning' },
            { alarm_status: 'active', alarm_level: 'critical', alarm_name: 'Older critical' }
          ]
        }
      })
    })

    expect(result).toEqual({
      device_alarm_count: 2,
      device_alarm_highest_level: 'critical',
      latest_device_alarm_title: 'Recent warning'
    })
  })

  it('returns empty alarm fields and reports load errors', async () => {
    const onError = vi.fn()

    const result = await buildRequestedAlarmStatusData({
      deviceId: 'dev-1',
      fieldIds: ['device_alarm_count'],
      loadDeviceAlarmStatus: vi.fn().mockRejectedValue(new Error('boom')),
      onError
    })

    expect(result).toEqual({})
    expect(onError).toHaveBeenCalledWith('dev-1', expect.any(Error))
  })

  it('builds requested field data by merging alarm and current values', async () => {
    const result = await buildRequestedFieldData({
      fieldIds: ['temp', 'setpoint', 'device_alarm_count', 'firmware_version'],
      deviceId: 'dev-1',
      alarmStatusFieldIds: new Set(['device_alarm_count']),
      rdiMetaFieldIds: new Set(['firmware_version']),
      historyFieldSuffix: '__history',
      silentRequestConfig,
      loadTelemetryCurrent: vi.fn().mockResolvedValue({ data: [{ key: 'temp', value: 31 }] }),
      loadAttributeDataSet: vi.fn().mockResolvedValue({ data: [{ key: 'setpoint', value: 40 }] }),
      loadRdiDeviceConfig: vi.fn().mockResolvedValue({ data: { device: { firmware_version: '1.2.3' } } }),
      loadDeviceAlarmStatus: vi.fn().mockResolvedValue({
        data: {
          total: 1,
          list: [{ alarm_status: 'active', alarm_level: 'warning', alarm_name: 'Recent warning' }]
        }
      })
    })

    expect(result).toEqual({
      temp: 31,
      setpoint: 40,
      device_alarm_count: 1,
      firmware_version: '1.2.3'
    })
  })

  it('returns empty field data when device id or normalized fields are empty', async () => {
    const result = await buildRequestedFieldData({
      fieldIds: [],
      deviceId: undefined,
      alarmStatusFieldIds: new Set(['device_alarm_count']),
      rdiMetaFieldIds: new Set(['firmware_version']),
      historyFieldSuffix: '__history',
      silentRequestConfig,
      loadTelemetryCurrent: vi.fn(),
      loadAttributeDataSet: vi.fn(),
      loadRdiDeviceConfig: vi.fn(),
      loadDeviceAlarmStatus: vi.fn()
    })

    expect(result).toEqual({})
  })
})
