import {
  buildReadyCheckEvidenceDeepLinks,
  formatReadyCheckDeepLink,
  normalizeRouteQueryForLink
} from '../ready-check-deep-links'

describe('ready-check-deep-links', () => {
  it('preserves current device context for device detail evidence links', () => {
    const links = buildReadyCheckEvidenceDeepLinks({
      routeQuery: {
        source: 'ota',
        tab: 'ready-check',
        empty: '',
        unexpected: 'drop-me',
        arrayValue: ['first', 'second']
      },
      deviceId: 'device-1',
      isOtaFailureSource: false
    })

    expect(links.map((link) => link.key)).toEqual(['telemetry', 'device-twin', 'command-delivery', 'audit-log'])
    expect(links[0]).toMatchObject({
      path: '/device/details',
      query: {
        source: 'ota',
        tab: 'telemetry',
        d_id: 'device-1'
      }
    })
    expect(formatReadyCheckDeepLink(links[0])).toContain('/device/details?')
    expect(formatReadyCheckDeepLink(links[0])).toContain('d_id=device-1')
    expect(formatReadyCheckDeepLink(links[0])).toContain('tab=telemetry')
  })

  it('adds OTA evidence only for OTA failure entrypoints', () => {
    const links = buildReadyCheckEvidenceDeepLinks({
      routeQuery: {
        source: 'ota',
        ota_task_id: 'task-from-route'
      },
      deviceId: 'device-1',
      isOtaFailureSource: true,
      otaTaskId: 'task-1',
      otaDetailId: 'detail-1'
    })

    expect(links.map((link) => link.key)).toEqual(['telemetry', 'device-twin', 'command-delivery', 'ota', 'audit-log'])
    expect(links.find((link) => link.key === 'ota')).toMatchObject({
      path: '/product/update-ota',
      query: {
        source: 'ready-check',
        ota_task_id: 'task-1',
        ota_detail_id: 'detail-1'
      }
    })
  })

  it('normalizes route query text before it is copied into evidence links', () => {
    expect(
      normalizeRouteQueryForLink({
        source: 'value',
        unexpected: 'drop-me',
        empty: '',
        nullValue: null,
        arrayValue: ['first', 'second']
      })
    ).toEqual({ source: 'value' })
  })
})
