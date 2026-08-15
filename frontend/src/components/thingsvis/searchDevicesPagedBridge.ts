export type SearchDevicesPagedRequest = {
  keyword: string
  groupId: string
  deviceConfigId: string
  isOnline: string
  warnStatus: string
  deviceType: string
  serviceIdentifier: string
  label: string
  page: number
  pageSize: number
  reqId: string
}

type SearchDevicesPagedGroupEntry = {
  groupId: string
  groupName: string
}

function getStringPayloadValue(payload: Record<string, unknown>, key: string, fallback = ''): string {
  const value = payload[key]
  return typeof value === 'string' ? value : fallback
}

function getNumberPayloadValue(payload: Record<string, unknown>, key: string, fallback: number): number {
  const value = payload[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

function normalizeFallbackGroupName(groupId?: unknown): string {
  const normalized = String(groupId || 'Ungrouped').trim()
  return normalized || 'Ungrouped'
}

export function normalizeSearchDevicesPagedPayload(payload: Record<string, unknown>): SearchDevicesPagedRequest {
  return {
    keyword: getStringPayloadValue(payload, 'keyword'),
    groupId: getStringPayloadValue(payload, 'groupId', '__all__'),
    deviceConfigId: getStringPayloadValue(payload, 'deviceConfigId'),
    isOnline: getStringPayloadValue(payload, 'isOnline'),
    warnStatus: getStringPayloadValue(payload, 'warnStatus'),
    deviceType: getStringPayloadValue(payload, 'deviceType'),
    serviceIdentifier: getStringPayloadValue(payload, 'serviceIdentifier'),
    label: getStringPayloadValue(payload, 'label'),
    page: getNumberPayloadValue(payload, 'page', 1),
    pageSize: getNumberPayloadValue(payload, 'pageSize', 10),
    reqId: getStringPayloadValue(payload, 'reqId')
  }
}

export function buildSearchDevicesPagedParams(request: SearchDevicesPagedRequest): Record<string, unknown> {
  const searchParams: Record<string, unknown> = {
    page: request.page,
    page_size: request.pageSize,
    search: request.keyword
  }

  if (request.groupId && request.groupId !== '__all__') {
    searchParams.group_id = request.groupId
  }
  if (request.deviceConfigId) searchParams.device_config_id = request.deviceConfigId
  if (request.isOnline) searchParams.is_online = Number(request.isOnline)
  if (request.warnStatus) searchParams.warn_status = request.warnStatus
  if (request.deviceType) searchParams.device_type = request.deviceType
  if (request.serviceIdentifier) searchParams.service_identifier = request.serviceIdentifier
  if (request.label) searchParams.label = request.label

  return searchParams
}

export function resolveSearchDevicesGroupContext(
  request: SearchDevicesPagedRequest,
  groups: SearchDevicesPagedGroupEntry[]
) {
  if (!request.groupId || request.groupId === '__all__') {
    return {
      groupId: '',
      fallbackGroupName: ''
    }
  }

  return {
    groupId: request.groupId,
    fallbackGroupName:
      groups.find((group) => group.groupId === request.groupId)?.groupName ||
      normalizeFallbackGroupName(request.groupId)
  }
}

export function buildSearchDevicesPagedResultPayload(
  request: Pick<SearchDevicesPagedRequest, 'reqId' | 'page' | 'pageSize'>,
  devices: unknown[],
  total: unknown
) {
  return {
    reqId: request.reqId,
    devices,
    total: total || 0,
    page: request.page,
    pageSize: request.pageSize
  }
}
