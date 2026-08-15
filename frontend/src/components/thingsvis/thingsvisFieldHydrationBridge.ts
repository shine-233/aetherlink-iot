/**
 * 文件说明：
 * - 封装 ThingsVis 平台数据源字段补水逻辑，负责解析节点中的字段绑定表达式并构建待补水的数据源描述。
 * - 同时提供按设备分组、按字段裁剪、按数据源回推 `tv:platform-data` 的辅助函数。
 * 维护提示：
 * - `{{ ds.xxx... }}` 表达式解析规则属于 ThingsVis schema 兼容边界，调整前要确认历史大屏配置仍可识别。
 * - 本模块只处理字段收集与补水编排，不直接读写 iframe、WebSocket 或平台接口。
 */
export type PlatformSourceDescriptor = {
  id: string
  deviceId?: string
  requestedFields: string[]
}

export type PlatformSourceDescriptorGroup = {
  descriptors: PlatformSourceDescriptor[]
  requestedFields: Set<string>
}

const FIELD_BINDING_EXPR_RE = /\{\{\s*ds\.([^.\s}]+)\.([^}]+?)\s*\}\}/g

type CollectPlatformSourceDescriptorsOptions = {
  resolveDeviceId: (dataSourceId: string, configuredDeviceId?: string) => string | undefined
}

export function pickRequestedPlatformFields(fields: Record<string, unknown>, requestedFields: string[]) {
  return requestedFields.reduce<Record<string, unknown>>((acc, fieldId) => {
    if (Object.prototype.hasOwnProperty.call(fields, fieldId)) {
      acc[fieldId] = fields[fieldId]
    }
    return acc
  }, {})
}

function collectRequestedFieldsFromValue(value: unknown, requests: Map<string, Set<string>>) {
  if (typeof value === 'string') {
    let match: RegExpExecArray | null = null
    FIELD_BINDING_EXPR_RE.lastIndex = 0
    while ((match = FIELD_BINDING_EXPR_RE.exec(value)) !== null) {
      const dataSourceId = match[1]
      const fieldPath = match[2]?.trim()
      if (!dataSourceId || !fieldPath) continue
      const normalizedPath = fieldPath.replace(/^data(?:\.|\[)/, '')
      const fieldId = normalizedPath.split(/[.[\]\s?:+\-*/=!<>&|(),]/).filter(Boolean)[0]
      if (!fieldId) continue
      const fields = requests.get(dataSourceId) ?? new Set<string>()
      fields.add(fieldId)
      requests.set(dataSourceId, fields)
    }
    return
  }

  if (Array.isArray(value)) {
    value.forEach((item) => collectRequestedFieldsFromValue(item, requests))
    return
  }

  if (!value || typeof value !== 'object') return

  Object.values(value as Record<string, unknown>).forEach((item) => {
    collectRequestedFieldsFromValue(item, requests)
  })
}

function getPlatformDataSources(config: any): any[] {
  return Array.isArray(config?.dataSources) ? config.dataSources : []
}

function isPlatformSourceDataSource(dataSource: any): boolean {
  const typeStr = typeof dataSource?.type === 'string' ? dataSource.type.toUpperCase() : ''
  return typeStr === 'PLATFORM_FIELD' || typeStr === 'PLATFORM'
}

function collectConfiguredRequestedFields(dataSource: any): string[] {
  return Array.isArray(dataSource?.config?.requestedFields)
    ? dataSource.config.requestedFields.filter((fieldId: unknown): fieldId is string => typeof fieldId === 'string')
    : []
}

function buildPlatformSourceRequestedFields(
  dataSourceId: string,
  dataSource: any,
  requests: Map<string, Set<string>>
): string[] {
  const requestedFields = new Set<string>(collectConfiguredRequestedFields(dataSource))
  const bindingFields = requests.get(dataSourceId)
  if (bindingFields) {
    bindingFields.forEach((fieldId) => requestedFields.add(fieldId))
  }
  return Array.from(requestedFields)
}

function buildPlatformSourceDescriptor(
  dataSource: any,
  requests: Map<string, Set<string>>,
  options: CollectPlatformSourceDescriptorsOptions
): PlatformSourceDescriptor {
  const dataSourceId = String(dataSource.id)

  return {
    id: dataSourceId,
    deviceId: options.resolveDeviceId(dataSourceId, dataSource?.config?.deviceId),
    requestedFields: buildPlatformSourceRequestedFields(dataSourceId, dataSource, requests)
  }
}

export function collectPlatformSourceDescriptors(
  config: any,
  options: CollectPlatformSourceDescriptorsOptions
): PlatformSourceDescriptor[] {
  const requests = new Map<string, Set<string>>()
  collectRequestedFieldsFromValue(config?.nodes, requests)

  return getPlatformDataSources(config)
    .filter((dataSource: any) => isPlatformSourceDataSource(dataSource))
    .map((dataSource: any) => buildPlatformSourceDescriptor(dataSource, requests, options))
}

export function groupPlatformSourceDescriptorsByDevice(
  descriptors: PlatformSourceDescriptor[]
): Map<string, PlatformSourceDescriptorGroup> {
  const descriptorsByDevice = new Map<string, PlatformSourceDescriptorGroup>()

  for (const descriptor of descriptors) {
    if (!descriptor.deviceId) continue

    const existing = descriptorsByDevice.get(descriptor.deviceId) ?? {
      descriptors: [],
      requestedFields: new Set<string>()
    }
    existing.descriptors.push(descriptor)
    descriptor.requestedFields.forEach((fieldId) => existing.requestedFields.add(fieldId))
    descriptorsByDevice.set(descriptor.deviceId, existing)
  }

  return descriptorsByDevice
}

export function collectPlatformSourceDeviceIds(descriptors: PlatformSourceDescriptor[]): string[] {
  return [
    ...new Set(
      descriptors.map((descriptor) => descriptor.deviceId).filter((deviceId): deviceId is string => !!deviceId)
    )
  ]
}

export async function hydratePlatformSourceDescriptorGroup(options: {
  deviceId: string
  group: PlatformSourceDescriptorGroup
  ensureDeviceWs?: (deviceId: string) => void
  ensureDeviceStatusWs?: (deviceId: string) => void
  loadRequestedFieldData: (fieldIds: string[], deviceId: string) => Promise<Record<string, unknown>>
  postPlatformData: (fields: Record<string, unknown>, deviceId?: string, dataSourceId?: string) => void
}): Promise<void> {
  const requestedFields = Array.from(options.group.requestedFields)
  if (requestedFields.length === 0) return

  options.ensureDeviceWs?.(options.deviceId)
  options.ensureDeviceStatusWs?.(options.deviceId)

  const fields = await options.loadRequestedFieldData(requestedFields, options.deviceId)

  options.group.descriptors.forEach((descriptor) => {
    options.postPlatformData(
      pickRequestedPlatformFields(fields, descriptor.requestedFields),
      options.deviceId,
      descriptor.id || undefined
    )
  })
}
