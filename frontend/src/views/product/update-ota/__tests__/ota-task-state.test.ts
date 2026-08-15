import { describe, expect, it } from 'vitest'
import {
  buildOtaDeviceOptions,
  buildOtaFilterSummaryItems,
  buildOtaPreviewDeviceRows,
  buildOtaTaskPreflightSummary,
  buildOtaTaskPreviewPayload,
  buildOtaTaskRiskDevices,
  buildOtaTaskSavePayload,
  canSaveOtaTask,
  extractList,
  extractTotal,
  otaTaskSaveValidationKey
} from '../ota-task-state'

describe('ota-task-state', () => {
  it('extracts list and total from supported API payload shapes', () => {
    expect(extractList([1, 2])).toEqual([1, 2])
    expect(extractList({ list: [1] })).toEqual([1])
    expect(extractList({ data: { list: [1] } })).toEqual([1])
    expect(extractList({ records: [1] })).toEqual([1])
    expect(extractList({})).toEqual([])

    expect(extractTotal({ total: 10 })).toBe(10)
    expect(extractTotal({ data: { total: 5 } })).toBe(5)
    expect(extractTotal({})).toBe(0)
  })

  it('builds eligible device options from the real device-list field variants', () => {
    expect(
      buildOtaDeviceOptions([
        { id: 'dev-1', name: 'Device A' },
        { device_id: 'dev-2', device_name: 'Device B' },
        { id: '', name: 'Invalid Device' }
      ])
    ).toEqual([
      { label: 'Device A', value: 'dev-1' },
      { label: 'Device B', value: 'dev-2' }
    ])
  })

  it('keeps OTA task save validation and payload construction explicit', () => {
    const form = {
      name: '  Upgrade Batch  ',
      description: '  staged rollout  ',
      device_id_list: ['dev-1', 'dev-2']
    }

    expect(canSaveOtaTask(null, form)).toBe(false)
    expect(otaTaskSaveValidationKey(null, form)).toBe('page.product.update-package.packagePlaceholder')
    expect(otaTaskSaveValidationKey('pkg-1', { ...form, name: '  ' })).toBe('page.product.update-ota.taskNameRequired')
    expect(otaTaskSaveValidationKey('pkg-1', { ...form, device_id_list: [] })).toBe(
      'page.product.update-ota.selectDeviceRequired'
    )

    expect(canSaveOtaTask('pkg-1', form)).toBe(true)
    expect(otaTaskSaveValidationKey('pkg-1', form)).toBe('')
    expect(buildOtaTaskSavePayload('pkg-1', form)).toEqual({
      name: 'Upgrade Batch',
      ota_upgrade_package_id: 'pkg-1',
      description: 'staged rollout',
      device_id_list: ['dev-1', 'dev-2']
    })
  })

  it('builds the server-side full-filter OTA payload without pretending it is a device-id list', () => {
    const form = {
      name: '  Full Filter Rollout  ',
      description: '',
      device_id_list: []
    }

    expect(canSaveOtaTask('pkg-1', form, {})).toBe(false)
    expect(otaTaskSaveValidationKey('pkg-1', form, {})).toBe('page.product.update-ota.selectDeviceRequired')
    expect(
      buildOtaTaskSavePayload('pkg-1', form, {
        device_filter: { group_id: 'group-1', is_online: 1 },
        expected_total: 42,
        max_devices: 5000
      })
    ).toEqual({
      name: 'Full Filter Rollout',
      ota_upgrade_package_id: 'pkg-1',
      description: undefined,
      device_filter: { group_id: 'group-1', is_online: 1 },
      exclude_device_id_list: [],
      expected_total: 42,
      max_devices: 5000
    })
    expect(
      buildOtaTaskPreviewPayload('pkg-1', {
        device_filter: { group_id: 'group-1', is_online: 1 },
        max_devices: 5000
      })
    ).toEqual({
      ota_upgrade_package_id: 'pkg-1',
      device_filter: { group_id: 'group-1', is_online: 1 },
      exclude_device_id_list: [],
      max_devices: 5000
    })
  })

  it('builds full-filter operator summary and preview sample rows', () => {
    expect(
      buildOtaFilterSummaryItems({
        group_id: 'group-1',
        is_online: 1,
        lifecycle_status: 'transmitted',
        unknown_filter: 'abc'
      })
    ).toEqual([
      { key: 'group_id', label: 'Device group', value: 'group-1' },
      { key: 'is_online', label: 'Online status', value: '1' },
      { key: 'lifecycle_status', label: 'Lifecycle status', value: 'Transmission complete (reported)' },
      { key: 'unknown_filter', label: 'Unknown Filter', value: 'abc' }
    ])

    expect(
      buildOtaPreviewDeviceRows([
        { id: 'dev-1', name: 'Device A', device_number: 'DN-1', current_version: '1.0', is_online: 1 },
        { id: 'dev-2', device_name: 'Device B', current_firmware_version: '2.0', online: 0 }
      ])
    ).toEqual([
      {
        id: 'dev-1',
        label: 'Device A',
        deviceNumber: 'DN-1',
        currentVersion: '1.0',
        online: '在线'
      },
      {
        id: 'dev-2',
        label: 'Device B',
        deviceNumber: 'dev-2',
        currentVersion: '2.0',
        online: '离线'
      }
    ])
  })

  it('summarizes selected OTA rollout risks with device-level reasons', () => {
    const rows = [
      { id: 'dev-1', name: 'Offline Device', current_version: '1.0', is_online: 0 },
      { id: 'dev-2', name: 'Current Device', current_version: '2.0', is_online: 1 },
      { id: 'dev-3', device_name: 'Unknown Device', online: true },
      { id: 'dev-4', name: 'Unselected Device', current_version: '2.0' }
    ]

    expect(buildOtaTaskPreflightSummary(rows, ['dev-1', 'dev-2', 'dev-3'], '2.0')).toEqual({
      eligible: 4,
      selected: 3,
      offline: 1,
      sameVersion: 1,
      missingVersion: 1,
      riskCount: 3
    })
    expect(buildOtaTaskRiskDevices(rows, ['dev-1', 'dev-2', 'dev-3'], '2.0')).toEqual([
      {
        id: 'dev-1',
        label: 'Offline Device',
        currentVersion: '1.0',
        reasonKeys: ['page.product.update-ota.preflightReasonOffline']
      },
      {
        id: 'dev-2',
        label: 'Current Device',
        currentVersion: '2.0',
        reasonKeys: ['page.product.update-ota.preflightReasonSameVersion']
      },
      {
        id: 'dev-3',
        label: 'Unknown Device',
        currentVersion: '',
        reasonKeys: ['page.product.update-ota.preflightReasonMissingVersion']
      }
    ])
  })
})
