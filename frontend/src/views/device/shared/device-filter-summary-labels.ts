import { $t } from '@/locales'

type Translator = (key: string) => string

const DEVICE_FILTER_LABEL_KEYS: Record<string, string> = {
  device_number: 'custom.deviceFilter.deviceNumber',
  name: 'custom.deviceFilter.name',
  search: 'custom.deviceFilter.search',
  group_id: 'custom.deviceFilter.group',
  device_config_id: 'custom.deviceFilter.config',
  device_template_id: 'custom.deviceFilter.template',
  product_id: 'custom.deviceFilter.product',
  firmware_version: 'custom.deviceFilter.firmwareVersion',
  current_version: 'custom.deviceFilter.currentVersion',
  pid_number: 'custom.deviceFilter.pid',
  is_online: 'custom.deviceFilter.onlineStatus',
  last_reported_after: 'custom.devicePage.lastReportedAfter',
  last_reported_before: 'custom.devicePage.lastReportedBefore',
  never_reported: 'custom.devicePage.neverReported',
  lifecycle_status: 'custom.devicePage.lifecycleStatus',
  is_enabled: 'custom.deviceFilter.enabledStatus',
  warn_status: 'custom.deviceFilter.alarmStatus',
  access_way: 'custom.deviceFilter.accessWay',
  service_identifier: 'custom.deviceFilter.serviceIdentifier',
  service_access_id: 'custom.deviceFilter.serviceAccess',
  shared_status: 'custom.deviceFilter.sharedStatus',
  batch_number: 'custom.deviceFilter.batchNumber',
  device_type: 'custom.deviceFilter.deviceType',
  label: 'custom.deviceFilter.label'
}

const LIFECYCLE_STATUS_VALUE_LABEL_KEYS: Record<string, string> = {
  activated: 'custom.devicePage.lifecycleActivatedOnly',
  inactive: 'custom.devicePage.lifecycleInactive',
  transmitted: 'custom.devicePage.lifecycleTransmitted',
  all: 'custom.devicePage.lifecycleAll'
}

export function getDeviceFilterLabel(key: string, t: Translator = $t) {
  const labelKey = DEVICE_FILTER_LABEL_KEYS[key]
  if (labelKey) return t(labelKey)

  return key
    .split('_')
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

export function getDeviceFilterValueLabel(key: string, value: unknown, t: Translator = $t) {
  if (key === 'lifecycle_status' && typeof value === 'string') {
    const labelKey = LIFECYCLE_STATUS_VALUE_LABEL_KEYS[value]
    if (labelKey) return t(labelKey)
  }
  return String(value)
}
