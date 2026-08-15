/**
 * 文件用途：覆盖 alarm-configuration.helpers 在 告警消息管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { describe, expect, it } from 'vitest'

import {
  alarmActionField,
  alarmSeverityLabel,
  alarmSeverityTagType,
  alarmSeverityValue,
  alarmTypeLabel,
  buildAlarmBatchActionEvidence,
  buildAlarmClosureEvidenceBundle,
  buildAlarmClosureEvidenceFileName,
  buildAlarmClosureEvidencePacket,
  buildAlarmClosureNextAction,
  buildAlarmEvidenceRow,
  buildAlarmResolutionTimeline,
  buildAlarmTriageSummary,
  createAlarmStatusOptions,
  isAcknowledged,
  parseAlarmRemark,
  sanitizeAlarmEvidenceFileToken
} from '../alarm-configuration.helpers'

const t = (key: string) => key
const formatTime = (value: unknown) => (value ? String(value) : '-')

describe('alarm-configuration helpers', () => {
  it('covers timeline states and evidence fallbacks for sparse alarm rows', () => {
    const pendingTimeline = buildAlarmResolutionTimeline(
      {
        alarm_status: 'Y',
        content: '',
        create_at: undefined,
        description: '',
        remark: '{}'
      },
      t,
      formatTime
    )

    expect(pendingTimeline).toEqual([
      expect.objectContaining({ key: 'created', description: 'custom.alarmPage.timelineCreatedDesc', time: '-' }),
      expect.objectContaining({ key: 'acknowledged', type: 'warning', time: '-' }),
      expect.objectContaining({ key: 'reset', type: 'info', time: '-' }),
      expect.objectContaining({ key: 'maintenance', type: 'default', time: '-' })
    ])

    const closedTimeline = buildAlarmResolutionTimeline(
      {
        alarm_status: 'N',
        content: 'temperature alarm',
        create_at: '2026-07-06T12:00:00Z',
        description: 'checked breaker',
        remark: JSON.stringify({
          acknowledged: true,
          acknowledged_by: 'ops',
          acknowledged_at: '2026-07-06 12:05:00',
          reset_by: 'ops',
          reset_at: '2026-07-06 12:30:00'
        })
      },
      t,
      formatTime
    )

    expect(closedTimeline).toEqual([
      expect.objectContaining({ key: 'created', description: 'temperature alarm', time: '2026-07-06T12:00:00Z' }),
      expect.objectContaining({ key: 'acknowledged', type: 'success', time: '2026-07-06 12:05:00' }),
      expect.objectContaining({ key: 'reset', type: 'success', time: '2026-07-06 12:30:00' }),
      expect.objectContaining({ key: 'maintenance', type: 'info', description: 'checked breaker' })
    ])
  })

  it('normalizes sparse evidence rows and bundle contexts without inventing values', () => {
    expect(sanitizeAlarmEvidenceFileToken('   ')).toBe('current-page')
    expect(sanitizeAlarmEvidenceFileToken(null)).toBe('current-page')

    const sparseRow = buildAlarmEvidenceRow({
      row: {},
      severityOptions: [],
      t,
      formatTime
    })
    expect(sparseRow).toMatchObject({
      id: '',
      name: '',
      alarmConfigName: '',
      severity: '',
      severityLabel: '-',
      alarmType: '-',
      createdAt: '-',
      acknowledged: false,
      acknowledgedBy: '-',
      reset: false,
      resetBy: '-',
      devices: []
    })

    const sparseBundle = buildAlarmClosureEvidenceBundle({
      tableData: [],
      queryData: {},
      pagination: {},
      selectedRowKeys: [],
      infoData: {},
      detailClosureNextAction: null,
      detailTimelineItems: [],
      alarmClosureEvidencePacket: null,
      lastSingleClosureEvidence: null,
      lastBatchActionEvidence: null,
      focusedDeviceId: '',
      hasRouteDeviceContext: false,
      fleetDeviceCount: 0,
      currentFleetPageCount: 0,
      requestedFleetTotal: 0,
      boundary: 'platform only',
      severityOptions: [],
      t,
      formatTime
    })

    expect(sparseBundle.pageContext).toMatchObject({
      filters: { alarmStatus: '', alarmType: '', focusedDeviceId: '', startTime: '', endTime: '' },
      pagination: { totalRowsReportedByApi: 0, loadedRowCount: 0 },
      selection: { selectedRowKeys: [], selectedLoadedRowCount: 0 }
    })
    expect(sparseBundle.currentSingleClosureEvidence).toBeNull()
  })

  it('emits no-device and empty-timeline evidence text and handles empty batch responses', () => {
    const packet = buildAlarmClosureEvidencePacket(
      {
        remark: null
      },
      [],
      t,
      formatTime
    )
    expect(packet).toContain('custom.alarmPage.closureEvidenceNoDevices')
    expect(packet).toContain('- custom.alarmPage.timelineDesc')

    const evidence = buildAlarmBatchActionEvidence({
      response: {},
      expectedCount: 2,
      action: 'acknowledge',
      note: '   ',
      t
    })
    expect(evidence).toMatchObject({
      expectedCount: 2,
      successCount: 2,
      failureCount: 0,
      note: '-',
      failedItems: [],
      type: 'success'
    })
    expect(evidence.generatedAt).toEqual(expect.any(String))
  })

  it('counts alarm triage states through active, acknowledged, reset, and high-severity branches', () => {
    expect(
      buildAlarmTriageSummary([
        { alarm_status: 'Y', alarm_level: 'H', remark: '{}' },
        { alarm_status: 'N', alarm_level: 'L', remark: '{"acknowledged":true}' }
      ])
    ).toEqual({ total: 2, high: 1, active: 1, acknowledged: 1, unacknowledged: 1, reset: 1 })
  })

  it('parses valid remark JSON and falls back to an empty object for invalid input', () => {
    expect(parseAlarmRemark('{"acknowledged":true,"acknowledged_at":"2026-06-20"}')).toEqual({
      acknowledged: true,
      acknowledged_at: '2026-06-20'
    })
    expect(parseAlarmRemark('{')).toEqual({})
    expect(parseAlarmRemark(123)).toEqual({})
  })

  it('reads action fields and acknowledgment state from the remark payload', () => {
    const row = {
      remark: '{"acknowledged":true,"acknowledged_by":"tech@example.com","reset_at":"2026-06-20T00:00:00Z"}'
    }

    expect(isAcknowledged(row)).toBe(true)
    expect(alarmActionField(row, 'acknowledged_by')).toBe('tech@example.com')
    expect(alarmActionField(row, 'reset_at')).toBe('2026-06-20T00:00:00Z')
    expect(alarmActionField(row, 'missing')).toBe('-')
  })

  it('derives severity and labels from row fields and remark fallbacks', () => {
    const options = createAlarmStatusOptions(t)

    expect(alarmSeverityValue({ alarm_level: 'H' })).toBe('H')
    expect(alarmSeverityValue({ alarm_status: 'M' })).toBe('M')
    expect(alarmSeverityValue({ remark: '{"severity":"L"}' })).toBe('L')
    expect(alarmSeverityLabel('H', options)).toBe('common.highAlarm')
    expect(alarmSeverityLabel('unknown', options)).toBe('unknown')
    expect(alarmSeverityTagType('H')).toBe('error')
    expect(alarmSeverityTagType('M')).toBe('warning')
    expect(alarmSeverityTagType('L')).toBe('info')
    expect(alarmSeverityTagType('N')).toBe('success')
    expect(alarmSeverityTagType('')).toBe('default')
  })

  it('maps known alarm event types and falls back to row labels', () => {
    expect(alarmTypeLabel({ remark: '{"event_type":"temperature_alarm"}' }, t)).toBe(
      'rdi.overview.temperatureAlarm'
    )
    expect(alarmTypeLabel({ remark: '{"event_type":"PT"}' }, t)).toBe('rdi.overview.pressureAlarm')
    expect(alarmTypeLabel({ alarm_config_name: 'Fallback name' }, t)).toBe('Fallback name')
    expect(alarmTypeLabel({ name: 'Custom name' }, t)).toBe('Custom name')
    expect(alarmTypeLabel({}, t)).toBe('-')
  })

  it('derives the customer-facing closure next action from existing alarm evidence', () => {
    expect(
      buildAlarmClosureNextAction(
        {
          create_at: '2026-06-20 12:00:00',
          alarm_status: 'Y',
          remark: '{}'
        },
        t,
        formatTime
      )
    ).toMatchObject({
      key: 'acknowledge',
      status: 'custom.alarmPage.closureNeedsAcknowledgeStatus',
      type: 'warning'
    })

    expect(
      buildAlarmClosureNextAction(
        {
          alarm_status: 'Y',
          remark: '{"acknowledged":true,"acknowledged_by":"ops","acknowledged_at":"2026-06-20 12:30:00"}'
        },
        t,
        formatTime
      )
    ).toMatchObject({
      key: 'maintenance',
      evidence: 'custom.alarmPage.closureAcknowledgedEvidence',
      type: 'info'
    })

    expect(
      buildAlarmClosureNextAction(
        {
          alarm_status: 'Y',
          description: 'checked breaker',
          remark: '{"acknowledged":true}'
        },
        t,
        formatTime
      )
    ).toMatchObject({
      key: 'reset',
      evidence: 'custom.alarmPage.closureMaintenanceEvidence',
      type: 'error'
    })

    expect(
      buildAlarmClosureNextAction(
        {
          alarm_status: 'N',
          description: 'recovered',
          remark: '{"acknowledged":true,"reset_by":"ops","reset_at":"2026-06-20 13:00:00"}'
        },
        t,
        formatTime
      )
    ).toMatchObject({
      key: 'closed',
      status: 'custom.alarmPage.closureClosedStatus',
      type: 'success'
    })
  })

  it('builds a copyable closure evidence packet with timeline and device rows', () => {
    const packet = buildAlarmClosureEvidencePacket(
      {
        name: 'High temperature',
        alarm_config_name: 'Temperature rule',
        alarm_status: 'H',
        content: 'Temperature too high',
        description: 'Checked cabinet fan',
        create_at: '2026-07-06T12:00:00Z',
        remark:
          '{"acknowledged":true,"acknowledged_by":"ops@example.com","acknowledged_at":"2026-07-06 12:05:00","reset_by":"ops@example.com","reset_at":"2026-07-06 12:30:00"}',
        alarm_device_list: [{ id: 'dev-1', name: 'Pump 1' }]
      },
      [
        {
          key: 'acknowledged',
          title: 'Acknowledged',
          description: 'Acknowledged by ops@example.com.',
          time: '2026-07-06 12:05:00',
          type: 'success'
        }
      ],
      t,
      formatTime
    )

    expect(packet).toContain('# custom.alarmPage.closureEvidenceTitle')
    expect(packet).toContain('High temperature')
    expect(packet).toContain('ops@example.com')
    expect(packet).toContain('- Acknowledged: 2026-07-06 12:05:00 - Acknowledged by ops@example.com.')
    expect(packet).toContain('1. dev-1 Pump 1')
    expect(packet).toContain('custom.alarmPage.auditBoundaryHint')
  })

  it('builds a downloadable closure evidence bundle for the current page context', () => {
    const severityOptions = createAlarmStatusOptions(t)
    const bundle = buildAlarmClosureEvidenceBundle({
      tableData: [
        {
          id: 'alarm-1',
          name: 'High temperature',
          alarm_config_name: 'Temperature rule',
          alarm_status: 'Y',
          alarm_level: 'H',
          content: 'Temperature too high',
          create_at: '2026-07-06T12:00:00Z',
          alarm_device_list: [{ id: 'dev-1', name: 'Pump 1' }]
        }
      ],
      queryData: {
        alarm_status: 'Y',
        alarm_type: 'temperature',
        start_time: '2026-07-06T00:00:00+08:00',
        end_time: '2026-07-06T23:59:59+08:00'
      },
      pagination: { page: 2, pageSize: 10, itemCount: 21 },
      selectedRowKeys: ['alarm-1'],
      infoData: { id: 'alarm-1', name: 'High temperature' },
      detailClosureNextAction: { key: 'acknowledge' },
      detailTimelineItems: [{ key: 'created' }],
      alarmClosureEvidencePacket: '# evidence',
      lastSingleClosureEvidence: { action: 'acknowledge' },
      lastBatchActionEvidence: { action: 'reset' },
      focusedDeviceId: 'dev-1',
      hasRouteDeviceContext: true,
      fleetDeviceCount: 3,
      currentFleetPageCount: 2,
      requestedFleetTotal: 5,
      boundary: 'platform evidence only',
      severityOptions,
      t,
      formatTime
    })

    expect(bundle).toMatchObject({
      schema: 'aetherlink.alarm.closure-evidence-bundle.v1',
      boundary: 'platform evidence only',
      pageContext: {
        filters: {
          alarmStatus: 'Y',
          alarmType: 'temperature',
          focusedDeviceId: 'dev-1'
        },
        pagination: {
          page: 2,
          pageSize: 10,
          totalRowsReportedByApi: 21,
          loadedRowCount: 1
        },
        selection: {
          selectedRowKeys: ['alarm-1'],
          selectedLoadedRowCount: 1
        }
      }
    })
    expect(bundle.loadedPageEvidence[0]).toMatchObject({
      id: 'alarm-1',
      severity: 'H',
      devices: [{ id: 'dev-1', name: 'Pump 1', number: '' }]
    })
    expect(bundle.currentSingleClosureEvidence?.copyPacket).toBe('# evidence')
    expect(bundle.verificationBoundary.fieldRecoveryNotProven).toBe(true)
  })

  it('builds stable evidence bundle file names', () => {
    expect(
      buildAlarmClosureEvidenceFileName({
        id: 'alarm 1/#',
        generatedAt: '2026-07-06T12:30:00.000Z',
        formatTimestamp: () => '20260706-123000'
      })
    ).toBe('aetherlink-alarm-closure-evidence-alarm-1-20260706-123000.json')
  })

  it('builds a persistent batch action evidence packet from response counts and failed rows', () => {
    const evidence = buildAlarmBatchActionEvidence({
      response: {
        data: {
          success_count: 2,
          failure_count: 1,
          results: [
            { id: 'alarm-1', ok: true },
            { id: 'alarm-2', ok: false, error: 'already reset' },
            { id: 'alarm-3', ok: true }
          ]
        }
      },
      expectedCount: 3,
      action: 'reset',
      note: 'night shift batch',
      t,
      generatedAt: '2026-07-06T10:00:00.000Z'
    })

    expect(evidence).toMatchObject({
      action: 'reset',
      expectedCount: 3,
      successCount: 2,
      failureCount: 1,
      note: 'night shift batch',
      type: 'warning'
    })
    expect(evidence.failedItems).toEqual(['alarm-2: already reset'])
    expect(evidence.copyText).toContain('note=night shift batch')
    expect(evidence.copyText).toContain('alarm-2: already reset')
    expect(evidence.copyText).toContain('custom.alarmPage.batchActionEvidenceBoundary')
  })

  it('derives batch action evidence counts from result rows when explicit counts are missing', () => {
    const evidence = buildAlarmBatchActionEvidence({
      response: {
        data: {
          results: [
            { id: 'alarm-1', ok: true },
            { id: 'alarm-2', ok: true }
          ]
        }
      },
      expectedCount: 2,
      action: 'acknowledge',
      t,
      generatedAt: '2026-07-06T11:00:00.000Z'
    })

    expect(evidence.successCount).toBe(2)
    expect(evidence.failureCount).toBe(0)
    expect(evidence.type).toBe('success')
    expect(evidence.copyText).toContain('- custom.alarmPage.batchActionNoFailedRows')
  })
})
