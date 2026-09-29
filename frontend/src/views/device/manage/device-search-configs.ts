/*
 * Search configuration for the device-management list.
 *
 * The key set is asserted by `__tests__/device-search-keys.test.ts`; adding or removing a
 * filter requires updating that authoritative list too.
 */
import type { SearchConfig } from '@/components/data-table-page/types'
import { $t } from '@/locales'
import { buildLifecycleStatusOptions } from './device-lifecycle-filter'

type OptionLoaders = {
  getDeviceGroupOptions: () => Promise<any>
  getDeviceConfigOptions: () => Promise<any>
}

export function createDeviceManageSearchConfigs(
  query: Record<string, any>,
  { getDeviceGroupOptions, getDeviceConfigOptions }: OptionLoaders
): SearchConfig[] {
  return [
    {
      key: 'group_id',
      label: 'custom.devicePage.selectGroup',
      type: 'tree-select',
      multiple: false,
      initValue: query.group_id,
      options: [{ label: $t('custom.devicePage.group'), key: '' }],
      loadOptions: getDeviceGroupOptions
    },
    {
      key: 'device_config_id',
      label: 'custom.devicePage.unlimitedDeviceConfig',
      type: 'select',
      options: [],
      initValue: query.device_config_id,
      labelField: 'name',
      valueField: 'id',
      loadOptions: getDeviceConfigOptions
    },
    {
      key: 'is_online',
      label: 'custom.devicePage.unlimitedOnlineStatus',
      type: 'select',
      initValue: query.is_online,
      options: [
        { label: () => $t('custom.devicePage.unlimitedOnlineStatus'), value: '' },
        { label: () => $t('custom.devicePage.online'), value: 1 },
        { label: () => $t('custom.devicePage.offline'), value: 0 }
      ]
    },
    {
      key: 'never_reported',
      label: 'custom.devicePage.reportHistoryStatus',
      type: 'select',
      initValue: query.never_reported,
      options: [
        { label: () => $t('custom.devicePage.allReportHistory'), value: '' },
        { label: () => $t('custom.devicePage.neverReported'), value: true },
        { label: () => $t('custom.devicePage.hasReported'), value: false }
      ]
    },
    {
      key: 'lifecycle_status',
      label: 'custom.devicePage.lifecycleStatus',
      type: 'select',
      initValue: query.lifecycle_status,
      options: buildLifecycleStatusOptions($t)
    },
    {
      key: 'last_reported_after',
      label: 'custom.devicePage.lastReportedAfter',
      type: 'date',
      initValue: query.last_reported_after
    },
    {
      key: 'last_reported_before',
      label: 'custom.devicePage.lastReportedBefore',
      type: 'date',
      initValue: query.last_reported_before
    },
    {
      key: 'warn_status',
      label: 'custom.devicePage.unlimitedAlarmStatus',
      type: 'select',
      initValue: query.warn_status,
      options: [
        { label: () => $t('custom.devicePage.unlimitedAlarmStatus'), value: '' },
        { label: () => $t('custom.devicePage.alarm'), value: 'Y' },
        { label: () => $t('custom.devicePage.noAlarm'), value: 'N' }
      ]
    },
    {
      key: 'device_type',
      label: 'custom.devicePage.unlimitedAccessType',
      initValue: query.device_type,
      type: 'select',
      options: [
        { label: $t('custom.devicePage.unlimitedAccessType'), value: '' },
        { label: $t('custom.devicePage.directConnectedDevices'), value: '1' },
        { label: $t('custom.devicePage.gateway'), value: '2' },
        { label: $t('custom.devicePage.gatewaySubEquipment'), value: '3' }
      ]
    },
    {
      key: 'service_identifier',
      label: 'card.anyProtocolService',
      type: 'select',
      initValue: query.service_identifier,
      options: [{ label: $t('card.anyProtocolService'), value: '' }]
    },
    {
      key: 'search',
      initValue: query.search,
      label: 'custom.devicePage.deviceSearch',
      type: 'input'
    },
    {
      key: 'name',
      initValue: query.name,
      label: 'custom.devicePage.deviceName',
      type: 'input'
    },
    {
      key: 'device_number',
      initValue: query.device_number,
      label: 'custom.devicePage.deviceNumber',
      type: 'input'
    },
    {
      key: 'pid_number',
      initValue: query.pid_number,
      label: 'custom.devicePage.pidNumber',
      type: 'input'
    },
    {
      key: 'firmware_version',
      initValue: query.firmware_version,
      label: 'custom.devicePage.firmwareVersion',
      type: 'input'
    },
    {
      key: 'description',
      initValue: query.description,
      label: 'custom.devicePage.description',
      type: 'input'
    },
    {
      key: 'shared_status',
      label: 'custom.devicePage.sharedStatus',
      type: 'select',
      initValue: query.shared_status,
      options: [
        { label: () => $t('custom.devicePage.allSharedStatus'), value: '' },
        { label: () => $t('custom.devicePage.shared'), value: 'shared' },
        { label: () => $t('custom.devicePage.unshared'), value: 'unshared' }
      ]
    },
    {
      key: 'label',
      initValue: query.label,
      label: 'custom.devicePage.label',
      type: 'input'
    }
  ]
}
