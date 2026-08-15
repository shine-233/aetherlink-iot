/**
 * 文件用途：提供 告警消息管理 页面内的 alarm-configuration.helpers 子组件。
 * 核心逻辑：封装局部表单、弹窗、列表或展示模块，通过 props、emit 与父页面协作。
 * 关键注意事项：保持组件边界清晰，避免在子组件中绕过父页面的数据刷新与权限控制。
 * 重构建议：后续可把重复表单规则、选项转换和弹窗状态管理抽成可复用组合函数。
 */
export type AlarmTranslate = (key: string) => string

export type AlarmOption = {
  label: string
  value: string
}

export type AlarmResolutionTimelineItem = {
  key: string
  title: string
  description: string
  time: string
  type: 'default' | 'info' | 'warning' | 'success' | 'error'
}

export type AlarmClosureNextAction = {
  key: 'acknowledge' | 'maintenance' | 'reset' | 'closed'
  status: string
  nextStep: string
  evidence: string
  type: 'default' | 'info' | 'warning' | 'success' | 'error'
}

export type AlarmClosureEvidenceDevice = {
  id?: string
  name?: string
  device_number?: string
  device_name?: string
}

export type AlarmClosureEvidenceRow = {
  name?: string
  alarm_config_name?: string
  alarm_device_list?: AlarmClosureEvidenceDevice[]
  alarm_level?: string
  alarm_status?: string
  content?: string
  create_at?: string
  description?: string
  remark?: unknown
}

export type AlarmBatchActionEvidence = {
  action: string
  generatedAt: string
  expectedCount: number
  successCount: number
  failureCount: number
  note: string
  failedItems: string[]
  summary: string
  detail: string
  copyText: string
  type: 'success' | 'warning'
}

export function sanitizeAlarmEvidenceFileToken(value: unknown) {
  const token = String(value || '')
    .trim()
    .replace(/[^a-zA-Z0-9._-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 80)

  return token || 'current-page'
}

export function buildAlarmEvidenceRow(options: {
  row: any
  severityOptions: AlarmOption[]
  t: AlarmTranslate
  formatTime: (value: unknown) => string
}) {
  const row = options.row || {}
  return {
    id: row.id || '',
    name: row.name || '',
    alarmConfigName: row.alarm_config_name || '',
    status: row.alarm_status || '',
    severity: alarmSeverityValue(row),
    severityLabel: alarmSeverityLabel(alarmSeverityValue(row), options.severityOptions),
    alarmType: alarmTypeLabel(row, options.t),
    content: row.content || '',
    description: row.description || '',
    createdAt: options.formatTime(row.create_at),
    acknowledged: isAcknowledged(row),
    acknowledgedBy: alarmActionField(row, 'acknowledged_by'),
    acknowledgedAt: alarmActionField(row, 'acknowledged_at'),
    reset: isReset(row),
    resetBy: alarmActionField(row, 'reset_by'),
    resetAt: alarmActionField(row, 'reset_at'),
    devices: (Array.isArray(row.alarm_device_list) ? row.alarm_device_list : []).slice(0, 12).map((device: any) => ({
      id: device?.id || device?.device_id || device?.deviceId || '',
      number: device?.device_number || '',
      name: device?.name || device?.device_name || ''
    }))
  }
}

export function buildAlarmClosureEvidenceBundle(options: {
  tableData: any[]
  queryData: any
  pagination: { page?: number; pageSize?: number; itemCount?: number }
  selectedRowKeys: Array<string | number>
  infoData: any
  detailClosureNextAction: unknown
  detailTimelineItems: unknown[]
  alarmClosureEvidencePacket: unknown
  lastSingleClosureEvidence: unknown
  lastBatchActionEvidence: unknown
  focusedDeviceId: string
  hasRouteDeviceContext: boolean
  fleetDeviceCount: number
  currentFleetPageCount: number
  requestedFleetTotal: number
  boundary: string
  severityOptions: AlarmOption[]
  t: AlarmTranslate
  formatTime: (value: unknown) => string
}) {
  const generatedAt = new Date().toISOString()
  const selectedRowKeys = options.selectedRowKeys.map(key => String(key))
  const selectedRowKeySet = new Set(selectedRowKeys)
  const selectedLoadedRows = options.tableData.filter(row => selectedRowKeySet.has(String(row.id)))
  const evidenceRow = (row: any) =>
    buildAlarmEvidenceRow({
      row,
      severityOptions: options.severityOptions,
      t: options.t,
      formatTime: options.formatTime
    })

  return {
    schema: 'aetherlink.alarm.closure-evidence-bundle.v1',
    generatedAt,
    boundary: options.boundary,
    pageContext: {
      scope: options.t('custom.alarmPage.evidenceBundleCurrentPageScope'),
      filters: {
        alarmStatus: options.queryData.alarm_status || '',
        alarmType: options.queryData.alarm_type || '',
        focusedDeviceId: options.focusedDeviceId || '',
        startTime: options.queryData.start_time || '',
        endTime: options.queryData.end_time || ''
      },
      pagination: {
        page: options.pagination.page,
        pageSize: options.pagination.pageSize,
        totalRowsReportedByApi: options.pagination.itemCount || 0,
        loadedRowCount: options.tableData.length
      },
      routeContext: {
        hasDeviceContext: options.hasRouteDeviceContext,
        fleetDeviceCount: options.fleetDeviceCount,
        currentFleetPageCount: options.currentFleetPageCount,
        requestedFleetTotal: options.requestedFleetTotal
      },
      selection: {
        selectedRowKeys,
        selectedLoadedRowCount: selectedLoadedRows.length,
        selectedLoadedRows: selectedLoadedRows.map(evidenceRow)
      }
    },
    loadedPageEvidence: options.tableData.map(evidenceRow),
    currentSingleClosureEvidence: options.infoData?.id
      ? {
          row: evidenceRow(options.infoData),
          nextAction: options.detailClosureNextAction,
          timeline: options.detailTimelineItems,
          copyPacket: options.alarmClosureEvidencePacket
        }
      : null,
    latestSingleActionEvidence: options.lastSingleClosureEvidence,
    latestBatchActionEvidence: options.lastBatchActionEvidence,
    verificationBoundary: {
      platformEvidenceOnly: true,
      fieldRecoveryNotProven: true,
      message: options.boundary
    }
  }
}

export function buildAlarmClosureEvidenceFileName(options: {
  id: unknown
  generatedAt: string
  formatTimestamp: (value: string) => string
}) {
  const fileToken = sanitizeAlarmEvidenceFileToken(options.id)
  return `aetherlink-alarm-closure-evidence-${fileToken}-${options.formatTimestamp(options.generatedAt)}.json`
}

export function parseAlarmRemark(raw: unknown) {
  if (!raw) return {} as Record<string, unknown>
  if (typeof raw === 'object') return raw as Record<string, unknown>
  if (typeof raw !== 'string') return {} as Record<string, unknown>
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, unknown>) : {}
  } catch {
    return {} as Record<string, unknown>
  }
}

export function alarmActionField(row: { remark?: unknown }, key: string) {
  const remark = parseAlarmRemark(row.remark)
  const value = remark[key]
  return value === undefined || value === null || value === '' ? '-' : String(value)
}

export function isAcknowledged(row: { remark?: unknown }) {
  return parseAlarmRemark(row.remark).acknowledged === true
}

export function isReset(row: { alarm_status?: string }) {
  return row.alarm_status === 'N'
}

export function buildAlarmResolutionTimeline(
  row: {
    alarm_status?: string
    content?: string
    create_at?: string
    description?: string
    remark?: unknown
  },
  t: AlarmTranslate,
  formatTime: (value: unknown) => string
): AlarmResolutionTimelineItem[] {
  const acknowledged = isAcknowledged(row)
  const reset = isReset(row)
  const acknowledgedBy = alarmActionField(row, 'acknowledged_by')
  const resetBy = alarmActionField(row, 'reset_by')
  const maintenanceNote = row.description || ''

  return [
    {
      key: 'created',
      title: t('custom.alarmPage.timelineCreatedTitle'),
      description: row.content || t('custom.alarmPage.timelineCreatedDesc'),
      time: formatTime(row.create_at),
      type: 'error'
    },
    {
      key: 'acknowledged',
      title: acknowledged
        ? t('custom.alarmPage.timelineAcknowledgedTitle')
        : t('custom.alarmPage.timelineAcknowledgePendingTitle'),
      description: acknowledged
        ? t('custom.alarmPage.timelineAcknowledgedDesc').replace('{operator}', acknowledgedBy)
        : t('custom.alarmPage.timelineAcknowledgePendingDesc'),
      time: acknowledged ? alarmActionField(row, 'acknowledged_at') : '-',
      type: acknowledged ? 'success' : 'warning'
    },
    {
      key: 'reset',
      title: reset ? t('custom.alarmPage.timelineResetTitle') : t('custom.alarmPage.timelineResetPendingTitle'),
      description: reset
        ? t('custom.alarmPage.timelineResetDesc').replace('{operator}', resetBy)
        : t('custom.alarmPage.timelineResetPendingDesc'),
      time: reset ? alarmActionField(row, 'reset_at') : '-',
      type: reset ? 'success' : 'info'
    },
    {
      key: 'maintenance',
      title: t('custom.alarmPage.timelineMaintenanceTitle'),
      description: maintenanceNote || t('custom.alarmPage.timelineMaintenanceEmpty'),
      time: maintenanceNote ? t('custom.alarmPage.timelineMaintenanceRecorded') : '-',
      type: maintenanceNote ? 'info' : 'default'
    }
  ]
}

export function buildAlarmClosureNextAction(
  row: {
    alarm_status?: string
    create_at?: string
    description?: string
    remark?: unknown
  },
  t: AlarmTranslate,
  formatTime: (value: unknown) => string
): AlarmClosureNextAction {
  const acknowledged = isAcknowledged(row)
  const reset = isReset(row)
  const hasMaintenanceNote = Boolean(row.description?.trim())

  if (!acknowledged) {
    return {
      key: 'acknowledge',
      status: t('custom.alarmPage.closureNeedsAcknowledgeStatus'),
      nextStep: t('custom.alarmPage.closureNeedsAcknowledgeNext'),
      evidence: t('custom.alarmPage.closureCreatedEvidence').replace('{time}', formatTime(row.create_at)),
      type: 'warning'
    }
  }

  if (!hasMaintenanceNote && !reset) {
    return {
      key: 'maintenance',
      status: t('custom.alarmPage.closureNeedsMaintenanceStatus'),
      nextStep: t('custom.alarmPage.closureNeedsMaintenanceNext'),
      evidence: t('custom.alarmPage.closureAcknowledgedEvidence')
        .replace('{operator}', alarmActionField(row, 'acknowledged_by'))
        .replace('{time}', alarmActionField(row, 'acknowledged_at')),
      type: 'info'
    }
  }

  if (!reset) {
    return {
      key: 'reset',
      status: t('custom.alarmPage.closureNeedsResetStatus'),
      nextStep: t('custom.alarmPage.closureNeedsResetNext'),
      // The no-maintenance/no-reset case returns above, so reaching this
      // branch guarantees a maintenance note is present.
      evidence: t('custom.alarmPage.closureMaintenanceEvidence'),
      type: 'error'
    }
  }

  return {
    key: 'closed',
    status: t('custom.alarmPage.closureClosedStatus'),
    nextStep: t('custom.alarmPage.closureClosedNext'),
    evidence: t('custom.alarmPage.closureResetEvidence')
      .replace('{operator}', alarmActionField(row, 'reset_by'))
      .replace('{time}', alarmActionField(row, 'reset_at')),
    type: 'success'
  }
}

export function buildAlarmClosureEvidencePacket(
  row: AlarmClosureEvidenceRow,
  timelineItems: AlarmResolutionTimelineItem[],
  t: AlarmTranslate,
  formatTime: (value: unknown) => string
) {
  const devices = Array.isArray(row.alarm_device_list) ? row.alarm_device_list : []
  const deviceLines =
    devices.length > 0
      ? devices
          .slice(0, 8)
          .map((device, index) => {
            const id = device.id || device.device_number || '-'
            const name = device.name || device.device_name || ''
            return `${index + 1}. ${id}${name ? ` ${name}` : ''}`
          })
      : [t('custom.alarmPage.closureEvidenceNoDevices')]

  const timelineLines =
    timelineItems.length > 0
      ? timelineItems.map(item => `- ${item.title}: ${item.time} - ${item.description}`)
      : [`- ${t('custom.alarmPage.timelineDesc')}`]

  return [
    `# ${t('custom.alarmPage.closureEvidenceTitle')}`,
    `${t('generate.alarmConfugName')}: ${row.name || '-'}`,
    `${t('generate.sceneLinkageName')}: ${row.alarm_config_name || '-'}`,
    `${t('common.alarm_time')}: ${formatTime(row.create_at)}`,
    `${t('common.alarm_level')}: ${alarmSeverityValue(row) || '-'}`,
    `${t('rdi.overview.alarmType')}: ${alarmTypeLabel(row, t)}`,
    `${t('generate.alarmReason')}: ${row.content || '-'}`,
    `${t('generate.alarm-description')}: ${row.description || '-'}`,
    `${t('rdi.overview.acknowledgedBy')}: ${alarmActionField(row, 'acknowledged_by')}`,
    `${t('rdi.overview.acknowledgedAt')}: ${alarmActionField(row, 'acknowledged_at')}`,
    `${t('rdi.overview.resetBy')}: ${alarmActionField(row, 'reset_by')}`,
    `${t('rdi.overview.resetAt')}: ${alarmActionField(row, 'reset_at')}`,
    '',
    `## ${t('custom.alarmPage.timelineTitle')}`,
    ...timelineLines,
    '',
    `## ${t('generate.alarmDevices')}`,
    ...deviceLines,
    '',
    t('custom.alarmPage.auditBoundaryHint')
  ].join('\n')
}

export function buildAlarmBatchActionEvidence(options: {
  response: any
  expectedCount: number
  action: string
  note?: string
  t: AlarmTranslate
  generatedAt?: string
}): AlarmBatchActionEvidence {
  const payload = options.response?.data || options.response || {}
  const results = Array.isArray(payload.results) ? payload.results : []
  const responseSuccessCount = Number(payload.success_count)
  const responseFailureCount = Number(payload.failure_count)
  const successCount = Number.isFinite(responseSuccessCount)
    ? responseSuccessCount
    : results.length
      ? results.filter((item: any) => item.ok).length
      : options.expectedCount
  const failureCount = Number.isFinite(responseFailureCount)
    ? responseFailureCount
    : results.length
      ? results.filter((item: any) => !item.ok).length
      : 0
  const summary = options.t('custom.alarmPage.batchActionSummary')
    .replace('{success}', String(successCount))
    .replace('{failure}', String(failureCount))
  const failedItems = results
    .filter((item: any) => !item.ok)
    .slice(0, 5)
    .map((item: any) => `${item.id || '-'}: ${item.error || '-'}`)
  const failureDetail = failedItems.length
    ? ` ${options.t('custom.alarmPage.batchActionFailureDetails').replace('{items}', failedItems.join('; '))}`
    : ''
  const detail = `${summary} ${options.t('custom.alarmPage.batchActionNextStep')}${failureDetail}`
  const generatedAt = options.generatedAt || new Date().toISOString()
  const note = options.note?.trim() || '-'
  const failedLines = failedItems.length
    ? failedItems.map(item => `- ${item}`)
    : [`- ${options.t('custom.alarmPage.batchActionNoFailedRows')}`]
  const copyText = [
    `# ${options.t('custom.alarmPage.batchActionEvidenceTitle')}`,
    `action=${options.action}`,
    `generatedAt=${generatedAt}`,
    `expected=${options.expectedCount}`,
    `success=${successCount}`,
    `failure=${failureCount}`,
    `note=${note}`,
    '',
    `## ${options.t('custom.alarmPage.batchActionFailedRows')}`,
    ...failedLines,
    '',
    options.t('custom.alarmPage.batchActionEvidenceBoundary')
  ].join('\n')

  return {
    action: options.action,
    generatedAt,
    expectedCount: options.expectedCount,
    successCount,
    failureCount,
    note,
    failedItems,
    summary,
    detail,
    copyText,
    type: failureCount > 0 ? 'warning' : 'success'
  }
}

export function buildAlarmTriageSummary<T extends { alarm_status?: string; remark?: unknown }>(rows: T[]) {
  return rows.reduce(
    (summary, row) => {
      summary.total += 1
      if (alarmSeverityValue(row) === 'H') summary.high += 1
      if (isAcknowledged(row)) summary.acknowledged += 1
      else summary.unacknowledged += 1
      if (isReset(row)) summary.reset += 1
      else summary.active += 1
      return summary
    },
    {
      total: 0,
      high: 0,
      active: 0,
      acknowledged: 0,
      unacknowledged: 0,
      reset: 0
    }
  )
}

export function alarmSeverityValue(row: { alarm_level?: string; alarm_status?: string; remark?: unknown }) {
  const remark = parseAlarmRemark(row.remark)
  return (
    row.alarm_level ||
    row.alarm_status ||
    String(remark.alarm_level || remark.level || remark.severity || remark.alarm_status || '')
  )
}

export function createAlarmStatusOptions(t: AlarmTranslate): AlarmOption[] {
  return [
    {
      label: t('common.allStatus'),
      value: ''
    },
    {
      label: t('common.highAlarm'),
      value: 'H'
    },
    {
      label: t('common.intermediateAlarm'),
      value: 'M'
    },
    {
      label: t('common.lowAlarm'),
      value: 'L'
    },
    {
      label: t('common.normal'),
      value: 'N'
    }
  ]
}

export function alarmSeverityLabel(value: string | undefined, options: AlarmOption[]) {
  const option = options.find(data => data.value === value)
  return option?.label || value || '-'
}

export function alarmSeverityTagType(value?: string) {
  if (value === 'H') return 'error'
  if (value === 'M') return 'warning'
  if (value === 'L') return 'info'
  if (value === 'N') return 'success'
  return 'default'
}

export function createAlarmTypeOptions(t: AlarmTranslate): AlarmOption[] {
  return [
    {
      label: t('rdi.overview.allAlarmTypes'),
      value: ''
    },
    {
      label: t('rdi.overview.temperatureAlarm'),
      value: 'temperature_alarm'
    },
    {
      label: t('rdi.overview.switchAlarm'),
      value: 'switch_alarm'
    },
    {
      label: t('rdi.overview.warrantyAlarm'),
      value: 'warranty_alarm'
    },
    {
      label: t('rdi.overview.pressureAlarm'),
      value: 'PT'
    }
  ]
}

export function alarmTypeLabel(row: { name?: string; alarm_config_name?: string; remark?: unknown }, t: AlarmTranslate) {
  const remark = parseAlarmRemark(row.remark)
  const eventType = String(remark.event_type || '')
  const labels: Record<string, string> = {
    temperature_alarm: t('rdi.overview.temperatureAlarm'),
    switch_alarm: t('rdi.overview.switchAlarm'),
    warranty_alarm: t('rdi.overview.warrantyAlarm'),
    pressure_alarm: t('rdi.overview.pressureAlarm'),
    PT: t('rdi.overview.pressureAlarm')
  }
  return labels[eventType] || row.alarm_config_name || row.name || '-'
}
