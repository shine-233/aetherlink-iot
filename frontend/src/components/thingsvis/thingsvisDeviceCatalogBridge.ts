/**
 * 文件说明：
 * - 封装 ThingsVis 宿主侧设备目录数据转换，包括设备筛选项、设备分组树扁平化与字段容错读取。
 * - AppFrame 只负责调用平台接口，本模块负责把后端返回的多种字段命名统一成 iframe 侧可消费的结构。
 * 维护提示：
 * - 分组 ID、设备配置 ID 和服务标识会被 ThingsVis 侧缓存或回传，改动时要保持兼容别名。
 * - 后续设备目录相关的纯转换逻辑应优先沉淀在这里，避免重新回流到 ThingsVisAppFrame.vue。
 */
export type DeviceFilterOption = {
  value: string
  label: string
}

export type PlatformDeviceGroupEntry = {
  groupId: string
  groupName: string
  deviceCount?: number
  parentId?: string | null
}

type LoadDeviceFilterOptionsOptions = {
  loadDeviceConfigs: () => Promise<any>
  loadServiceCatalog: () => Promise<any>
  unwrapList: (payload: any) => any[]
  firstString: (...values: unknown[]) => string | undefined
  asRecord: (value: unknown) => Record<string, any>
  protocolLabelPrefix: string
  serviceLabelPrefix: string
}

type LoadPlatformDeviceGroupsOptions = {
  loadGroupTree: () => Promise<any>
  flattenDeviceGroupTree: (rows: any[]) => PlatformDeviceGroupEntry[]
  allGroupLabel: string
  onError?: (error: unknown) => void
}

export function firstString(...values: unknown[]): string | undefined {
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

export function asRecord(value: unknown): Record<string, any> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, any>) : {}
}

export function normalizeEditorGroupId(groupId?: unknown, fallbackName?: unknown): string {
  const normalized = String(groupId || fallbackName || '__ungrouped__').trim()
  return normalized || '__ungrouped__'
}

export function normalizeEditorGroupName(groupName?: unknown, fallbackId?: unknown): string {
  const normalized = String(groupName || fallbackId || 'Ungrouped').trim()
  return normalized || 'Ungrouped'
}

export function flattenDeviceGroupTree(
  nodes: unknown[],
  groups = new Map<string, PlatformDeviceGroupEntry>()
): PlatformDeviceGroupEntry[] {
  nodes.forEach((node) => {
    if (!node || typeof node !== 'object') return
    const treeNode = asRecord(node)
    const rawGroup = asRecord(treeNode.group || treeNode.data || treeNode)
    const groupId = normalizeEditorGroupId(
      firstString(rawGroup.id, rawGroup.group_id, rawGroup.groupId, rawGroup.value, treeNode.id, treeNode.group_id),
      firstString(rawGroup.name, rawGroup.group_name, rawGroup.groupName, rawGroup.label, treeNode.name, treeNode.label)
    )
    const groupName = normalizeEditorGroupName(
      firstString(
        rawGroup.name,
        rawGroup.group_name,
        rawGroup.groupName,
        rawGroup.label,
        treeNode.name,
        treeNode.label
      ),
      groupId
    )
    const parentId = firstString(rawGroup.parent_id, rawGroup.parentId, treeNode.parent_id, treeNode.parentId)
    const deviceCount = firstNumber(
      treeNode.deviceCount,
      treeNode.device_count,
      rawGroup.deviceCount,
      rawGroup.device_count
    )
    groups.set(groupId, {
      groupId,
      groupName,
      ...(deviceCount !== undefined ? { deviceCount } : {}),
      ...(parentId !== undefined ? { parentId } : {})
    })

    const children = Array.isArray(treeNode.children)
      ? treeNode.children
      : Array.isArray(treeNode.child)
        ? treeNode.child
        : Array.isArray(treeNode.list)
          ? treeNode.list
          : []

    if (children.length > 0) {
      flattenDeviceGroupTree(children, groups)
    }
  })

  return Array.from(groups.values()).sort((a, b) => a.groupName.localeCompare(b.groupName))
}

function resolveDeviceConfigName(
  config: any,
  configId: string,
  firstString: (...values: unknown[]) => string | undefined
): string {
  return (
    firstString(
      config?.name,
      config?.device_config_name,
      config?.deviceConfigName,
      config?.config_name,
      config?.configName,
      configId
    ) || configId
  )
}

export async function loadDeviceFilterOptions(
  options: LoadDeviceFilterOptionsOptions
): Promise<{ deviceConfigs: DeviceFilterOption[]; serviceOptions: DeviceFilterOption[] }> {
  const [configRes, serviceRes] = await Promise.allSettled([options.loadDeviceConfigs(), options.loadServiceCatalog()])

  const deviceConfigs =
    configRes.status === 'fulfilled'
      ? options
          .unwrapList(configRes.value?.data)
          .map((config: any): DeviceFilterOption | null => {
            const configId = options.firstString(config?.id, config?.device_config_id, config?.deviceConfigId)
            if (!configId) return null
            return {
              value: configId,
              label: resolveDeviceConfigName(config, configId, options.firstString)
            }
          })
          .filter((item): item is DeviceFilterOption => Boolean(item))
      : []

  const serviceData = serviceRes.status === 'fulfilled' ? options.asRecord(serviceRes.value?.data) : {}
  const protocolOptions = Array.isArray(serviceData.protocol)
    ? serviceData.protocol.map((item: any): DeviceFilterOption | null => {
        const value = options.firstString(item?.service_identifier, item?.serviceIdentifier, item?.id)
        const label = options.firstString(item?.name, item?.label, value)
        return value && label ? { value, label: `${options.protocolLabelPrefix}${label}` } : null
      })
    : []
  const serviceOptions = Array.isArray(serviceData.service)
    ? serviceData.service.map((item: any): DeviceFilterOption | null => {
        const value = options.firstString(item?.service_identifier, item?.serviceIdentifier, item?.id)
        const label = options.firstString(item?.name, item?.label, value)
        return value && label ? { value, label: `${options.serviceLabelPrefix}${label}` } : null
      })
    : []

  return {
    deviceConfigs,
    serviceOptions: [...protocolOptions, ...serviceOptions].filter((item): item is DeviceFilterOption => Boolean(item))
  }
}

export async function loadPlatformDeviceGroups(
  options: LoadPlatformDeviceGroupsOptions
): Promise<PlatformDeviceGroupEntry[]> {
  try {
    const res = await options.loadGroupTree()
    const groups = options.flattenDeviceGroupTree(Array.isArray(res?.data) ? res.data : [])
    groups.unshift({
      groupId: '__all__',
      groupName: options.allGroupLabel,
      parentId: null
    })
    return groups
  } catch (error) {
    options.onError?.(error)
    return []
  }
}
