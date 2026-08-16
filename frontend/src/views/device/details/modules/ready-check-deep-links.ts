export type ReadyCheckDeepLink = {
  key: string
  labelKey: string
  descriptionKey: string
  path: string
  query: Record<string, string>
  boundaryKey: string
}

export type BuildReadyCheckEvidenceDeepLinksOptions = {
  routeQuery: Record<string, unknown>
  deviceId: string
  isOtaFailureSource: boolean
  otaTaskId?: string
  otaDetailId?: string
}

const READY_CHECK_ROUTE_QUERY_KEYS = new Set([
  'source',
  'tab',
  'onboarding',
  'fleet_source',
  'command_source',
  'ready_check',
  'ota_task_id',
  'ota_detail_id'
])

export const normalizeRouteQueryText = (value: unknown) => {
  if (Array.isArray(value)) return String(value[0] || '')
  return typeof value === 'string' ? value : ''
}

export const normalizeRouteQueryForLink = (routeQuery: Record<string, unknown>) => {
  const query: Record<string, string> = {}
  Object.entries(routeQuery).forEach(([key, value]) => {
    if (!READY_CHECK_ROUTE_QUERY_KEYS.has(key)) return
    const text = normalizeRouteQueryText(value)
    if (text) query[key] = text
  })
  return query
}

const deviceDetailsLink = (
  tab: 'telemetry' | 'device-twin' | 'command-delivery',
  deviceId: string,
  routeQuery: Record<string, unknown>
): ReadyCheckDeepLink => ({
  key: tab,
  labelKey:
    tab === 'telemetry'
      ? 'custom.device_details.readyCheckDeepLinkTelemetry'
      : tab === 'device-twin'
        ? 'custom.device_details.readyCheckDeepLinkTwin'
        : 'custom.device_details.readyCheckDeepLinkCommand',
  descriptionKey:
    tab === 'telemetry'
      ? 'custom.device_details.readyCheckDeepLinkTelemetryDesc'
      : tab === 'device-twin'
        ? 'custom.device_details.readyCheckDeepLinkTwinDesc'
        : 'custom.device_details.readyCheckDeepLinkCommandDesc',
  path: '/device/details',
  query: {
    ...normalizeRouteQueryForLink(routeQuery),
    d_id: deviceId,
    tab
  },
  boundaryKey: 'custom.device_details.readyCheckDeepLinkDeviceTabBoundary'
})

export const buildReadyCheckEvidenceDeepLinks = ({
  routeQuery,
  deviceId,
  isOtaFailureSource,
  otaTaskId = '',
  otaDetailId = ''
}: BuildReadyCheckEvidenceDeepLinksOptions): ReadyCheckDeepLink[] => {
  const links: ReadyCheckDeepLink[] = [
    deviceDetailsLink('telemetry', deviceId, routeQuery),
    deviceDetailsLink('device-twin', deviceId, routeQuery),
    deviceDetailsLink('command-delivery', deviceId, routeQuery),
    {
      key: 'audit-log',
      labelKey: 'custom.device_details.readyCheckDeepLinkAudit',
      descriptionKey: 'custom.device_details.readyCheckDeepLinkAuditDesc',
      path: '/system-management-user/system-log',
      query: {
        source: 'ready-check',
        path: deviceId ? `/device/${deviceId}` : '/device'
      },
      boundaryKey: 'custom.device_details.readyCheckDeepLinkAuditBoundary'
    }
  ]

  if (isOtaFailureSource) {
    links.splice(3, 0, {
      key: 'ota',
      labelKey: 'custom.device_details.readyCheckDeepLinkOta',
      descriptionKey: 'custom.device_details.readyCheckDeepLinkOtaDesc',
      path: '/product/update-ota',
      query: {
        source: 'ready-check',
        ...(otaTaskId ? { ota_task_id: otaTaskId } : {}),
        ...(otaDetailId ? { ota_detail_id: otaDetailId } : {})
      },
      boundaryKey: 'custom.device_details.readyCheckDeepLinkOtaBoundary'
    })
  }

  return links
}

export const routeQueryString = (query: Record<string, string>) => {
  const params = new URLSearchParams()
  Object.entries(query).forEach(([key, value]) => {
    if (value !== '') params.set(key, value)
  })
  const text = params.toString()
  return text ? `?${text}` : ''
}

export const formatReadyCheckDeepLink = (link: ReadyCheckDeepLink) => `${link.path}${routeQueryString(link.query)}`
