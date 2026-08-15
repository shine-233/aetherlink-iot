import { describe, expect, it } from 'vitest'
import {
  buildReadyCheckEvidenceCards,
  buildDeviceAccessGuideState,
  buildDeviceAccessGuideStateFromConnectionGuide,
  splitMqttEndpoint
} from '../device-access-guide-state'
import {
  buildDeviceAccessGuideAccessPacket,
  buildDeviceAccessGuideSupportSummary,
  buildDeviceAccessGuideTriageView
} from '../device-access-guide-triage-view'

describe('device-access-guide-state', () => {
  it('builds MQTT onboarding test commands from backend connect info and saved voucher fields', () => {
    const guide = buildDeviceAccessGuideState(
      {
        server: 'mqtts://broker.example.com:8883',
        username: 'mqtt_device_001',
        report_topic: 'devices/telemetry',
        control_topic: 'devices/telemetry/control/D-001',
        remark: '{"temperature": 23}'
      },
      'D-001',
      {
        username: 'device-user',
        password: 'device-pass'
      }
    )

    expect(guide).toMatchObject({
      protocol: 'MQTT',
      endpointKind: 'mqtt',
      endpoint: 'mqtts://broker.example.com:8883',
      authMode: 'BASIC',
      username: 'device-user',
      password: 'device-pass',
      reportTopic: 'devices/telemetry',
      controlTopic: 'devices/telemetry/control/D-001',
      tlsHintKey: 'custom.device_details.accessGuideTlsEnabled'
    })
    expect(guide.commands.map((command) => command.titleKey)).toEqual([
      'custom.device_details.accessGuideMosquitto',
      'custom.device_details.accessGuideNode',
      'custom.device_details.accessGuidePython',
      'custom.device_details.accessGuideC'
    ])
    expect(guide.commands[0].code).toContain('-P "device-pass"')
    expect(guide.commands[2].code).toContain('client.tls_set()')
    expect(guide.quickstartSteps).toEqual([
      expect.objectContaining({
        titleKey: 'custom.device_details.accessGuideQuickstartEndpoint',
        copyText: 'mqtts://broker.example.com:8883'
      }),
      expect.objectContaining({
        titleKey: 'custom.device_details.accessGuideQuickstartCredential',
        copyText: 'clientId=mqtt_device_001\nusername=device-user\npassword=device-pass'
      }),
      expect.objectContaining({
        titleKey: 'custom.device_details.accessGuideQuickstartPublish',
        copyText: expect.stringContaining('mosquitto_pub')
      }),
      expect.objectContaining({
        titleKey: 'custom.device_details.accessGuideQuickstartVerify',
        actionKey: 'custom.device_details.accessGuideNextStepRunReadyCheck'
      })
    ])
  })

  it('builds HTTP onboarding samples without inventing MQTT-only topics', () => {
    const guide = buildDeviceAccessGuideState(
      {
        http_endpoint: 'https://aetherlink.local/api/v1/device/report',
        example_payload: '{"humidity": 60}',
        last_error: '401 invalid token'
      },
      'D-002',
      {
        access_token: 'token-123'
      }
    )

    expect(guide).toMatchObject({
      protocol: 'HTTP',
      endpointKind: 'http',
      endpoint: 'https://aetherlink.local/api/v1/device/report',
      username: 'token-123',
      lastError: '401 invalid token',
      tlsHintKey: 'custom.device_details.accessGuideTlsEnabled'
    })
    expect(guide.commands.map((command) => command.titleKey)).toEqual([
      'custom.device_details.accessGuideCurl',
      'custom.device_details.accessGuideNode',
      'custom.device_details.accessGuidePython',
      'custom.device_details.accessGuideC'
    ])
    expect(guide.commands[0].code).toContain('Authorization: Bearer token-123')
    expect(guide.commands.join('\n')).not.toContain('mosquitto_pub')
    expect(guide.quickstartSteps[1]).toEqual(
      expect.objectContaining({
        descriptionKey: 'custom.device_details.accessGuideQuickstartCredentialHttpDesc',
        copyText: 'token=token-123'
      })
    )
  })

  it('prefers stable backend connection profile over localized connection info maps', () => {
    const guide = buildDeviceAccessGuideStateFromConnectionGuide(
      {
        access: {
          protocol: 'MQTT',
          credential_mode: 'BASIC',
          connection_info: {
            '接入地址': 'localized-broker.example.com:1883',
            '上报Topic': 'localized/topic'
          },
          connection_profile: {
            protocol: 'MQTT',
            endpoint: 'mqtts://stable-broker.example.com:8883',
            host: 'stable-broker.example.com',
            port: '8883',
            tls_enabled: true,
            client_id: 'mqtt_device_123',
            username: 'mqtt_device_123',
            telemetry_topic: 'devices/telemetry',
            command_topic: 'devices/telemetry/control/D-123',
            sample_payload: '{"temperature":26}'
          },
          tls: { enabled: true }
        },
        readiness: {
          online: true,
          ready: true,
          level: 'ok',
          code: 'ready',
          summary: 'ready'
        }
      },
      'D-123',
      { password: 'secret' },
      {}
    )

    expect(guide.endpoint).toBe('mqtts://stable-broker.example.com:8883')
    expect(guide.clientId).toBe('mqtt_device_123')
    expect(guide.reportTopic).toBe('devices/telemetry')
    expect(guide.controlTopic).toBe('devices/telemetry/control/D-123')
    expect(guide.payload).toBe('{"temperature":26}')
    expect(guide.commands[0].code).toContain('stable-broker.example.com')
    expect(guide.sdkBundle).toContain('protocol=MQTT')
    expect(guide.sdkBundle).toContain('stable-broker.example.com')
  })

  it('builds synced twin evidence cards from camelCase backend summary fields', () => {
    const cards = buildReadyCheckEvidenceCards({
      twin_summary: {
        desiredCount: 2,
        reportedCount: 2,
        matchedCount: 2,
        deltaCount: 0,
        unavailableCount: 0
      }
    })
    const twinCard = cards.find((card) => card.key === 'twin')

    expect(twinCard).toEqual(
      expect.objectContaining({
        status: 'ready',
        statusKey: 'custom.device_details.readyCheckEvidenceSynced',
        summaryKey: 'custom.device_details.readyCheckTwinSummarySynced',
        nextActions: []
      })
    )
    expect(twinCard?.metrics).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: 'desired',
          labelKey: 'custom.device_details.twinDesired',
          value: '2',
          tone: 'success'
        }),
        expect.objectContaining({
          key: 'reported',
          labelKey: 'custom.device_details.twinReported',
          value: '2',
          tone: 'success'
        }),
        expect.objectContaining({
          key: 'delta',
          value: '0',
          tone: 'success'
        })
      ])
    )
  })

  it('marks twin evidence as review-needed when delta, unavailable, or partial collection exists', () => {
    const cards = buildReadyCheckEvidenceCards({
      twin_summary: {
        desiredCount: 3,
        reportedCount: 2,
        matchedCount: 1,
        deltaCount: 2,
        unavailableCount: 1
      },
      partial_results: [
        {
          component: 'device_twin',
          reason: 'collector_timeout'
        }
      ]
    })
    const twinCard = cards.find((card) => card.key === 'twin')

    expect(twinCard).toEqual(
      expect.objectContaining({
        status: 'attention',
        statusKey: 'custom.device_details.readyCheckEvidenceNeedsReview',
        summaryKey: 'custom.device_details.readyCheckTwinSummaryNeedsReview',
        nextActions: ['device_twin: collector_timeout'],
        nextActionKeys: ['custom.device_details.readyCheckTwinNextReview']
      })
    )
    expect(twinCard?.metrics).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: 'unavailable',
          value: '1',
          tone: 'warning'
        })
      ])
    )
  })

  it('builds command evidence cards without inventing stronger execution proof', () => {
    const cards = buildReadyCheckEvidenceCards({
      command_summary: {
        level: 'ok',
        code: 'device_ack_success',
        summary: 'Latest command was acknowledged by the device.',
        latest_status: 'device_ack_success',
        latest_message_id: 'cmd-001',
        next_actions: ['Keep the message ID for audit.']
      }
    })
    const commandCard = cards.find((card) => card.key === 'command')

    expect(commandCard).toEqual(
      expect.objectContaining({
        status: 'ready',
        statusKey: 'custom.device_details.readyCheckEvidenceConfirmed',
        summary: 'Latest command was acknowledged by the device.',
        nextActions: ['Keep the message ID for audit.']
      })
    )
    expect(commandCard?.metrics).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: 'latest-status',
          labelKey: 'custom.device_details.readyCheckCommandLatestStatus',
          value: 'device_ack_success',
          tone: 'success'
        }),
        expect.objectContaining({
          key: 'message-id',
          labelKey: 'custom.device_details.messageId',
          value: 'cmd-001'
        })
      ])
    )
  })

  it('keeps missing and partial command evidence explicit', () => {
    const cards = buildReadyCheckEvidenceCards({
      partial_results: [
        {
          component: 'command_delivery',
          reason: 'log_query_failed'
        }
      ]
    })
    const commandCard = cards.find((card) => card.key === 'command')

    expect(commandCard).toEqual(
      expect.objectContaining({
        status: 'attention',
        statusKey: 'custom.device_details.readyCheckEvidenceNeedsReview',
        summary: '',
        summaryKey: 'custom.device_details.readyCheckCommandSummaryUnknown',
        nextActions: ['command_delivery: log_query_failed']
      })
    )
  })

  it('prefers diagnostics failure text over compatible connect-info error fields', () => {
    const guide = buildDeviceAccessGuideState(
      {
        server: 'broker.example.com:1883',
        username: 'mqtt_device_003',
        last_error: 'compat connect-info error'
      },
      'D-003',
      {},
      '[processor] uplink: lua decode failed'
    )

    expect(guide.lastError).toBe('[processor] uplink: lua decode failed')
    expect(guide.diagnostics).toContainEqual(
      expect.objectContaining({
        labelKey: 'custom.device_details.accessGuideDiagnosticCurrentIssue',
        value: '[processor] uplink: lua decode failed',
        tone: 'danger'
      })
    )
  })

  it('builds customer-facing diagnostics from connection evidence', () => {
    const guide = buildDeviceAccessGuideState(
      {
        server: 'broker.example.com:1883',
        username: 'mqtt_device_004'
      },
      'D-004',
      {},
      {
        isOnline: false,
        debugEnabled: true,
        recentLogCount: 3,
        latestIssue: 'auth failed',
        partialWarnings: ['diagnostics: collector_not_initialized']
      }
    )

    expect(guide.diagnostics).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          labelKey: 'custom.device_details.accessGuideReadyCheck'
        }),
        expect.objectContaining({
          labelKey: 'custom.device_details.accessGuideLatestTelemetry'
        }),
        expect.objectContaining({
          labelKey: 'custom.device_details.accessGuideDiagnosticOnline',
          valueKey: 'custom.device_details.offline',
          tone: 'warning'
        }),
        expect.objectContaining({
          labelKey: 'custom.device_details.accessGuideDiagnosticDebug',
          valueKey: 'custom.device_details.accessGuideDiagnosticDebugOn',
          tone: 'success'
        }),
        expect.objectContaining({
          labelKey: 'custom.device_details.accessGuideDiagnosticRecentLogs',
          value: '3'
        }),
        expect.objectContaining({
          labelKey: 'custom.device_details.accessGuideDiagnosticCurrentIssue',
          value: 'auth failed',
          tone: 'danger'
        }),
        expect.objectContaining({
          labelKey: 'custom.device_details.accessGuideDiagnosticPartial',
          value: 'diagnostics: collector_not_initialized',
          tone: 'warning'
        })
      ])
    )
  })

  it('builds a redacted JSON access packet for support handoff', () => {
    const guide = buildDeviceAccessGuideState(
      {
        server: 'mqtts://broker.example.com:8883',
        username: 'mqtt_device_005',
        report_topic: 'devices/telemetry',
        control_topic: 'devices/telemetry/control/D-005',
        remark: '{"temperature": 23}'
      },
      'D-005',
      {
        username: 'device-user',
        password: 'device-secret'
      },
      {
        ready: false,
        latestIssue: 'auth failed',
        partialWarnings: ['debug: waiting_for_reconnect']
      }
    )
    const t = (key: string) => key
    const triageView = buildDeviceAccessGuideTriageView({
      accessGuide: guide,
      t
    })

    const packet = buildDeviceAccessGuideAccessPacket({
      accessGuide: guide,
      triageView,
      t,
      generatedAt: '2026-07-07T00:00:00.000Z'
    })

    expect(packet).toMatchObject({
      schema: 'aetherlink.device.access-packet.v2',
      generatedAt: '2026-07-07T00:00:00.000Z',
      credentialPolicy: {
        secretsRedacted: true,
        shareRawCredentialsSeparately: true
      },
      connection: {
        protocol: 'MQTT',
        endpoint: 'mqtts://broker.example.com:8883',
        username: 'device-user',
        password: '<redacted>',
        reportTopic: 'devices/telemetry',
        controlTopic: 'devices/telemetry/control/D-005'
      },
      verificationBoundary: {
        platformEvidenceOnly: true,
        fieldConnectionNotProven: true
      }
    })
    expect(packet.credentialPolicy.redactedFields).toContain('connection.password')
    expect(packet.testCommands[0].code).toContain('<redacted>')
    expect(packet.testCommands[0].code).not.toContain('device-secret')
    expect(packet.firstRunnableTestCommand).not.toContain('device-secret')
    expect(packet.markdownPacket).not.toContain('device-secret')
    expect(packet.supportSummary).toContain('password=<provided>')
    expect(packet.supportSummary).not.toContain('device-secret')
    expect(
      buildDeviceAccessGuideSupportSummary({
        accessGuide: guide,
        triageView,
        t
      })
    ).not.toContain('device-secret')
  })

  it('redacts HTTP access tokens from support packets and test commands', () => {
    const guide = buildDeviceAccessGuideState(
      {
        http_endpoint: 'https://aetherlink.local/api/v1/device/report',
        example_payload: '{"humidity": 60}'
      },
      'D-006',
      {
        access_token: 'token-secret-123'
      }
    )
    const t = (key: string) => key
    const triageView = buildDeviceAccessGuideTriageView({
      accessGuide: guide,
      t
    })

    const packet = buildDeviceAccessGuideAccessPacket({
      accessGuide: guide,
      triageView,
      t
    })

    expect(packet.connection.username).toBe('<redacted>')
    expect(packet.credentialPolicy.redactedFields).toContain('connection.username')
    expect(JSON.stringify(packet.testCommands)).not.toContain('token-secret-123')
    expect(packet.firstRunnableTestCommand).not.toContain('token-secret-123')
    expect(packet.markdownPacket).not.toContain('token-secret-123')
    expect(packet.supportSummary).not.toContain('token-secret-123')
  })

  it('splits MQTT endpoints while keeping safe placeholders for missing hosts', () => {
    expect(splitMqttEndpoint('mqtt://broker.example.com:1884')).toEqual({
      host: 'broker.example.com',
      port: '1884'
    })
    expect(splitMqttEndpoint(':1883')).toEqual({
      host: '<mqtt-host>',
      port: '1883'
    })
  })
})
