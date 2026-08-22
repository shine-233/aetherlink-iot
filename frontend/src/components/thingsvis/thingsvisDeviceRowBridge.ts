export type NormalizedPlatformDeviceRow = {
  deviceId: string
  deviceName: string
  groupId: string
  groupName: string
  deviceConfigId?: string
  deviceConfigName?: string
  isOnline?: number | string | boolean
  warnStatus?: string
  deviceType?: string
  accessWay?: string
  protocolType?: string
  lastPushTime?: string
  templateId?: string
}

export type PlatformDeviceRowContext = {
  fallbackGroupId: string
  fallbackGroupName: string
  groupNameById: Map<string, string>
  configTemplateMap: Map<string, string>
}

/** 设备嵌套对象（后端返回，字段宽松） */
type DeviceRecordLike = {
  id?: unknown
  name?: unknown
  group?: { id?: unknown; name?: unknown } | null
  device_config?: { id?: unknown; name?: unknown; device_template_id?: unknown; deviceTemplateId?: unknown } | null
  [key: string]: unknown
}

/** 平台设备行（后端返回，字段宽松，snake_case/camelCase 双写法兼容） */
type DeviceRowLike = {
  device?: unknown
  device_info?: unknown
  deviceInfo?: unknown
  device_data?: unknown
  deviceData?: unknown
  device_config?: DeviceRecordLike['device_config']
  group?: DeviceRecordLike['group']
  [key: string]: unknown
}

function firstString(...values: unknown[]): string | undefined {
  for (const value of values) {
    if (value === undefined || value === null) continue
    const normalized = String(value).trim()
    if (normalized) return normalized
  }
  return undefined
}

function firstNumber(...values: unknown[]): number | undefined {
  for (const value of values) {
    if (typeof value === 'number' && Number.isFinite(value)) return value
    if (typeof value === 'string' && value.trim()) {
      const parsed = Number(value)
      if (Number.isFinite(parsed)) return parsed
    }
  }
  return undefined
}

function asRecord(value: unknown): DeviceRecordLike {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as DeviceRecordLike) : {}
}

function unwrapDeviceRow(row: DeviceRowLike | null | undefined): DeviceRecordLike {
  return asRecord(row?.device || row?.device_info || row?.deviceInfo || row?.device_data || row?.deviceData || row)
}

function resolveDeviceId(row: DeviceRowLike | null | undefined): string | undefined {
  const nestedDevice = row?.device || row?.device_info || row?.deviceInfo || row?.device_data || row?.deviceData
  if (nestedDevice && typeof nestedDevice === 'object') {
    const device = asRecord(nestedDevice)
    return firstString(device.id, device.device_id, device.deviceId)
  }

  return firstString(row?.device_id, row?.deviceId, row?.id)
}

function resolveDeviceName(row: DeviceRowLike | null | undefined, deviceId: string): string {
  const device = unwrapDeviceRow(row)
  return firstString(
    device.device_name,
    device.deviceName,
    row?.device_name,
    row?.deviceName,
    device.name,
    row?.name,
    device.device_number,
    device.deviceNumber,
    row?.device_number,
    row?.deviceNumber,
    deviceId
  ) as string
}

function resolveDeviceConfigId(row: DeviceRowLike | null | undefined): string | undefined {
  const device = unwrapDeviceRow(row)
  return firstString(
    device.device_config_id,
    device.deviceConfigId,
    row?.device_config_id,
    row?.deviceConfigId,
    device.device_config?.id,
    row?.device_config?.id
  )
}

function resolveDeviceConfigDisplayName(row: DeviceRowLike | null | undefined): string | undefined {
  const device = unwrapDeviceRow(row)
  return firstString(
    device.device_config_name,
    device.deviceConfigName,
    row?.device_config_name,
    row?.deviceConfigName,
    device.device_config?.name,
    row?.device_config?.name
  )
}

function resolveDeviceOnlineStatus(row: DeviceRowLike | null | undefined): number | string | boolean | undefined {
  const device = unwrapDeviceRow(row)
  return (
    firstNumber(device.is_online, device.isOnline, row?.is_online, row?.isOnline) ??
    (typeof device.is_online === 'boolean' ? device.is_online : undefined) ??
    (typeof row?.is_online === 'boolean' ? row.is_online : undefined)
  )
}

function resolveDeviceWarnStatus(row: DeviceRowLike | null | undefined): string | undefined {
  const device = unwrapDeviceRow(row)
  return firstString(device.warn_status, device.warnStatus, row?.warn_status, row?.warnStatus)
}

function resolveDeviceType(row: DeviceRowLike | null | undefined): string | undefined {
  const device = unwrapDeviceRow(row)
  return firstString(device.device_type, device.deviceType, row?.device_type, row?.deviceType)
}

function resolveDeviceAccessWay(row: DeviceRowLike | null | undefined): string | undefined {
  const device = unwrapDeviceRow(row)
  return firstString(device.access_way, device.accessWay, row?.access_way, row?.accessWay)
}

function resolveDeviceProtocolType(row: DeviceRowLike | null | undefined): string | undefined {
  const device = unwrapDeviceRow(row)
  return firstString(device.protocol_type, device.protocolType, row?.protocol_type, row?.protocolType)
}

function resolveDeviceLastPushTime(row: DeviceRowLike | null | undefined): string | undefined {
  const device = unwrapDeviceRow(row)
  return firstString(
    device.ts,
    row?.ts,
    device.last_push_time,
    device.lastPushTime,
    row?.last_push_time,
    row?.lastPushTime
  )
}

function resolveDeviceTemplateId(row: DeviceRowLike | null | undefined, configTemplateMap: Map<string, string> = new Map()): string | undefined {
  const device = unwrapDeviceRow(row)
  const configId = resolveDeviceConfigId(row)
  const configName = resolveDeviceConfigDisplayName(row)
  return firstString(
    device.device_config?.device_template_id,
    device.device_config?.deviceTemplateId,
    row?.device_config?.device_template_id,
    row?.device_config?.deviceTemplateId,
    device.device_template_id,
    device.deviceTemplateId,
    row?.device_template_id,
    row?.deviceTemplateId,
    configId ? configTemplateMap.get(configId) : undefined,
    configName ? configTemplateMap.get(configName) : undefined
  )
}

function resolveDeviceGroupId(row: DeviceRowLike | null | undefined): string | undefined {
  const device = unwrapDeviceRow(row)
  return firstString(
    device.group_id,
    device.groupId,
    row?.group_id,
    row?.groupId,
    device.device_group_id,
    device.deviceGroupId,
    row?.device_group_id,
    row?.deviceGroupId,
    device.group?.id,
    row?.group?.id
  )
}

function resolveDeviceGroupName(row: DeviceRowLike | null | undefined): string | undefined {
  const device = unwrapDeviceRow(row)
  return firstString(
    device.group_name,
    device.groupName,
    row?.group_name,
    row?.groupName,
    device.device_group_name,
    device.deviceGroupName,
    row?.device_group_name,
    row?.deviceGroupName,
    device.group?.name,
    row?.group?.name
  )
}

function resolvePlatformDeviceGroupName(
  row: DeviceRowLike | null | undefined,
  groupId: string,
  fallbackGroupName: string,
  groupNameById: Map<string, string>
) {
  return resolveDeviceGroupName(row) || (groupId ? groupNameById.get(groupId) : undefined) || fallbackGroupName
}

export function normalizePlatformDeviceRow(
  row: DeviceRowLike | null | undefined,
  { fallbackGroupId, fallbackGroupName, groupNameById, configTemplateMap }: PlatformDeviceRowContext
): NormalizedPlatformDeviceRow | null {
  const deviceId = resolveDeviceId(row)
  if (!deviceId) return null

  const deviceConfigId = resolveDeviceConfigId(row)
  const deviceConfigName = resolveDeviceConfigDisplayName(row)
  const rowGroupId = resolveDeviceGroupId(row) || fallbackGroupId
  const isOnline = resolveDeviceOnlineStatus(row)
  const warnStatus = resolveDeviceWarnStatus(row)
  const deviceType = resolveDeviceType(row)
  const accessWay = resolveDeviceAccessWay(row)
  const protocolType = resolveDeviceProtocolType(row)
  const lastPushTime = resolveDeviceLastPushTime(row)
  const templateId = resolveDeviceTemplateId(row, configTemplateMap)

  return {
    deviceId,
    deviceName: resolveDeviceName(row, deviceId),
    groupId: rowGroupId,
    groupName: resolvePlatformDeviceGroupName(row, rowGroupId, fallbackGroupName, groupNameById),
    ...(deviceConfigId ? { deviceConfigId } : {}),
    ...(deviceConfigName ? { deviceConfigName } : {}),
    ...(isOnline !== undefined ? { isOnline } : {}),
    ...(warnStatus ? { warnStatus } : {}),
    ...(deviceType ? { deviceType } : {}),
    ...(accessWay ? { accessWay } : {}),
    ...(protocolType ? { protocolType } : {}),
    ...(lastPushTime ? { lastPushTime } : {}),
    ...(templateId ? { templateId } : {})
  }
}
