import { describe, expect, it } from 'vitest'

import {
  buildReadyCheckDiagnosticMarkdown,
  buildReadyCheckSupportBundle,
  readyCheckSupportFileName,
  type ReadyCheckSupportBundleInput
} from '../ready-check-support-bundle'

const t = (key: string) => key

const baseInput = (): ReadyCheckSupportBundleInput => ({
  t,
  generatedAt: '2026-07-06T12:00:00.000Z',
  device: {
    id: 'device-1',
    name: 'Pump 1',
    number: 'PUMP-1',
    online: true,
    hasConnectionIdentity: true,
    hasTemplate: true
  },
  source: {
    sourceKey: 'ota_failed_rollout',
    label: 'OTA failed rollout',
    detail: 'task=task-1 / detail=detail-1',
    otaTaskId: 'task-1',
    otaDetailId: 'detail-1',
    firstDeviceOnboarding: false
  },
  readiness: {
    ready: false,
    level: 'warning',
    code: 'command_review',
    summary: 'Command response needs review',
    evaluatedAt: '2026-07-06T11:59:00.000Z',
    connectionGuideSummary: 'Twin has a delta'
  },
  telemetry: {
    latest: 'temperature @ 2026-07-06T11:58:00.000Z',
    latestValue: '{"value":26}',
    currentCount: 1
  },
  diagnostics: {
    nextActions: ['Check command response', 'Review twin delta'],
    lastConnectionIssue: 'Broker disconnected recently',
    partialResults: 'command_delivery: log_query_partial',
    conclusion: { summary: 'Review platform evidence before retrying' },
    debug: {
      enabled: true,
      recentLogs: [
        {
          direction: 'downlink',
          action: 'command',
          outcome: 'timeout',
          error: 'no_ack',
          meta: { message_id: 'msg-1' }
        }
      ]
    },
    recentFailures: [
      {
        timestamp: '2026-07-06T11:57:00.000Z',
        direction: 'uplink',
        stage: 'mqtt',
        error: 'disconnect'
      }
    ],
    partialWarnings: [{ component: 'device_twin', reason: 'delta_open' }]
  },
  evidenceCenterItems: [
    {
      key: 'source',
      labelKey: 'custom.device_details.readyCheckEvidenceSource',
      value: 'OTA failed rollout',
      detail: 'task=task-1 / detail=detail-1'
    }
  ],
  evidenceCards: [
    {
      key: 'twin',
      titleKey: 'custom.device_details.readyCheckTwinTitle',
      descriptionKey: 'custom.device_details.readyCheckTwinDescription',
      boundaryKey: 'custom.device_details.readyCheckTwinBoundary',
      status: 'attention',
      statusKey: 'custom.device_details.readyCheckEvidenceAttention',
      summary: 'Twin delta remains open',
      metrics: [{ key: 'delta', labelKey: 'custom.device_details.readyCheckTwinDelta', value: '1', tone: 'warning' }],
      nextActions: ['Confirm reported state']
    },
    {
      key: 'command',
      titleKey: 'custom.device_details.readyCheckCommandTitle',
      descriptionKey: 'custom.device_details.readyCheckCommandDescription',
      boundaryKey: 'custom.device_details.readyCheckCommandBoundary',
      status: 'attention',
      statusKey: 'custom.device_details.readyCheckEvidenceAttention',
      summary: 'Command timed out',
      metrics: [{ key: 'latest_status', labelKey: 'custom.device_details.readyCheckCommandStatus', value: 'timeout', tone: 'danger' }],
      nextActions: ['Open command delivery']
    }
  ],
  backendNextSteps: [{ key: 'refresh', title: 'Refresh Ready Check', description: 'Retry after device reconnects.', status: 'todo' }],
  deepLinks: [
    {
      key: 'command',
      labelKey: 'custom.device_details.readyCheckLinkCommand',
      path: '/device/details',
      query: { d_id: 'device-1', tab: 'command-delivery' },
      boundaryKey: 'custom.device_details.readyCheckCommandBoundary'
    }
  ],
  collectionFailures: [],
  boundaryText: 'Platform evidence is not physical execution proof.'
})

describe('ready-check-support-bundle', () => {
  it('builds markdown with source context and anti-overclaim boundaries', () => {
    const markdown = buildReadyCheckDiagnosticMarkdown(baseInput())

    expect(markdown).toContain('source=ota_failed_rollout')
    expect(markdown).toContain('otaTaskId=task-1')
    expect(markdown).toContain('boundary=custom.device_details.readyCheckTwinBoundary')
    expect(markdown).toContain('boundary=custom.device_details.readyCheckCommandBoundary')
    expect(markdown).toContain('Platform evidence is not physical execution proof.')
  })

  it('builds a JSON bundle with translated evidence cards and markdown summary', () => {
    const bundle = buildReadyCheckSupportBundle(baseInput())

    expect(bundle.schema).toBe('aetherlink.ready-check.diagnostics.v1')
    expect(bundle.generated_at).toBe('2026-07-06T12:00:00.000Z')
    expect(bundle.evidenceCards).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: 'twin',
          boundary: 'custom.device_details.readyCheckTwinBoundary'
        }),
        expect.objectContaining({
          key: 'command',
          boundary: 'custom.device_details.readyCheckCommandBoundary'
        })
      ])
    )
    expect(bundle.markdownSummary).toContain('## 证据入口')
  })

  it('records frontend collector failures in markdown and JSON', () => {
    const input = baseInput()
    input.collectionFailures = [
      {
        key: 'commands',
        labelKey: 'custom.device_details.readyCheckCollectionCommands'
      }
    ]

    const bundle = buildReadyCheckSupportBundle(input)

    expect(bundle.markdownSummary).toContain('## 前端采集失败')
    expect(bundle.markdownSummary).toContain('commands: custom.device_details.readyCheckCollectionCommands')
    expect(bundle.collectionFailures).toEqual([
      {
        key: 'commands',
        label: 'custom.device_details.readyCheckCollectionCommands'
      }
    ])
  })

  it('sanitizes ready-check support filenames', () => {
    expect(readyCheckSupportFileName('Pump 1 / Field #2')).toBe('aetherlink-ready-check-Pump_1___Field__2-diagnostics.json')
    expect(readyCheckSupportFileName('')).toBe('aetherlink-ready-check-device-diagnostics.json')
  })
})
