import type { PlatformDeviceGroupEntry } from './thingsvisDeviceCatalogBridge'

type DeviceGroupLike = Pick<PlatformDeviceGroupEntry, 'groupId' | 'groupName' | 'parentId'>
type DeviceLike = {
  deviceId: string
}

type MapDevicesForGroup<TDevice, TGroup extends DeviceGroupLike> = (
  rawDevices: any[],
  fallbackGroupId?: string,
  fallbackGroupName?: string,
  groups?: TGroup[]
) => Promise<TDevice[]>

function normalizeNonEmptyString(value: unknown): string | undefined {
  if (value === undefined || value === null) return undefined
  const normalized = String(value).trim()
  return normalized || undefined
}

export async function findPlatformDeviceById<TDevice extends DeviceLike, TGroup extends DeviceGroupLike>(options: {
  deviceId: string
  loadGroups: () => Promise<TGroup[]>
  searchDevices: (deviceId: string) => Promise<any[]>
  listDevices: () => Promise<any[]>
  mapDevicesForGroup: MapDevicesForGroup<TDevice, TGroup>
  loadDevicesByGroup: (groupId: string) => Promise<TDevice[]>
}): Promise<TDevice | null> {
  const normalizedDeviceId = normalizeNonEmptyString(options.deviceId)
  if (!normalizedDeviceId) return null

  const groups = await options.loadGroups()

  const findMappedDevice = async (rawDevices: any[], fallbackGroupId = '', fallbackGroupName = '') => {
    const devices = await options.mapDevicesForGroup(rawDevices, fallbackGroupId, fallbackGroupName, groups)
    return devices.find((device) => device.deviceId === normalizedDeviceId) || null
  }

  const searched = await findMappedDevice(await options.searchDevices(normalizedDeviceId))
  if (searched) return searched

  const listed = await findMappedDevice(await options.listDevices())
  if (listed) return listed

  for (const group of groups) {
    const devices = await options.loadDevicesByGroup(group.groupId)
    const matched = devices.find((device) => device.deviceId === normalizedDeviceId)
    if (matched) return matched
  }

  return null
}

export async function loadPlatformDevicesByGroup<TDevice, TGroup extends DeviceGroupLike>(options: {
  groupId: string
  loadGroups: () => Promise<TGroup[]>
  loadRelatedDevices: (groupId: string) => Promise<any[]>
  mapDevicesForGroup: MapDevicesForGroup<TDevice, TGroup>
  buildFallbackDevices: (groupId: string, groupName: string, groups: TGroup[]) => Promise<TDevice[]>
  normalizeGroupId: (groupId: string) => string
  normalizeGroupName: (groupName?: string, fallbackId?: string) => string
  onError?: (groupId: string, error: unknown) => void
}): Promise<TDevice[]> {
  const normalizedGroupId = options.normalizeGroupId(options.groupId)

  try {
    const [rawDevices, groups] = await Promise.all([
      options.loadRelatedDevices(normalizedGroupId),
      options.loadGroups()
    ])

    const groupName =
      groups.find((group) => group.groupId === normalizedGroupId)?.groupName ||
      options.normalizeGroupName(undefined, normalizedGroupId)

    if (rawDevices.length > 0) {
      return options.mapDevicesForGroup(rawDevices, normalizedGroupId, groupName, groups)
    }

    return options.buildFallbackDevices(normalizedGroupId, groupName, groups)
  } catch (error) {
    options.onError?.(normalizedGroupId, error)
    return []
  }
}
