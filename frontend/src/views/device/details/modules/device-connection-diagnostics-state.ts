import type { DeviceAccessGuideDiagnosticsSummary } from './device-access-guide-state'

export type DeviceConnectionDiagnosticsWarning = {
  component?: string
  reason?: string
}

export type DeviceConnectionDebugLog = {
  direction?: string
  action?: string
  outcome?: string
  error?: string
  meta?: Record<string, unknown>
}

export type DeviceConnectionDiagnosticFailure = {
  timestamp?: string | number
  direction?: string
  stage?: string
  error?: string
}

export type DeviceConnectionDiagnosticsConclusion = {
  level?: string
  code?: string
  summary?: string
  next_actions?: string[]
  evidence?: string[]
}

export type DeviceConnectionDiagnosticsResponse = {
  conclusion?: DeviceConnectionDiagnosticsConclusion
  online?: {
    is_online?: boolean
  }
  ready_check?: {
    ready?: boolean
    level?: string
    code?: string
    summary?: string
    next_actions?: string[]
    telemetry?: {
      has_recent_current?: boolean
      current_count?: number
      latest_key?: string
      latest_at?: string
      latest_value?: unknown
    }
  }
  debug?: {
    enabled?: boolean
    recent_logs?: DeviceConnectionDebugLog[]
  }
  diagnostics?: {
    recent_failures?: DeviceConnectionDiagnosticFailure[]
  }
  partial_results?: DeviceConnectionDiagnosticsWarning[]
}

export const unwrapDeviceConnectionDiagnosticsResponse = (
  response: unknown
): DeviceConnectionDiagnosticsResponse => {
  const value = response as any
  return value?.data?.data ?? value?.data ?? value ?? {}
}

const normalizeDiagnosticsFailure = (failure: DeviceConnectionDiagnosticFailure | undefined) => {
  if (!failure) return ''

  const stage = failure.stage ? `[${failure.stage}] ` : ''
  const direction = failure.direction ? `${failure.direction}: ` : ''
  const error = failure.error || ''
  return `${stage}${direction}${error}`.trim()
}

const debugLogMetaString = (log: DeviceConnectionDebugLog | undefined, key: string) => {
  const value = log?.meta?.[key]
  if (value === undefined || value === null || value === '') return ''
  return String(value)
}

const normalizeDebugLogDiagnosticIssue = (log: DeviceConnectionDebugLog | undefined) => {
  const code = debugLogMetaString(log, 'diagnostic_code')
  if (code === 'disconnect_error') {
    const error = log?.error ? `: ${log.error}` : ''
    const action = debugLogMetaString(log, 'recommended_action')
    const next = action ? ` / next=${action}` : ''
    return `Broker reported an unexpected disconnect${error}${next}`
  }
  return ''
}

const normalizeDebugLogError = (log: DeviceConnectionDebugLog | undefined) => {
  const diagnosticIssue = normalizeDebugLogDiagnosticIssue(log)
  if (diagnosticIssue) return diagnosticIssue
  if (!log?.error) return ''

  const action = log.action ? `[${log.action}] ` : ''
  const direction = log.direction ? `${log.direction}: ` : ''
  return `${action}${direction}${log.error}`.trim()
}

const normalizeDiagnosticsWarning = (warning: DeviceConnectionDiagnosticsWarning | undefined) => {
  if (!warning) return ''

  const component = warning.component || 'diagnostics'
  const reason = warning.reason || 'partial_result'
  return `${component}: ${reason}`
}

const resolveLatestConnectionIssue = (data: DeviceConnectionDiagnosticsResponse) => {
  const debugError = data.debug?.recent_logs?.map(normalizeDebugLogError).find(Boolean)
  if (debugError) return debugError

  const failures = Array.isArray(data.diagnostics?.recent_failures) ? data.diagnostics?.recent_failures : []
  const failureError = normalizeDiagnosticsFailure(failures?.[0])
  if (failureError) return failureError

  const warnings = Array.isArray(data.partial_results) ? data.partial_results : []
  return normalizeDiagnosticsWarning(warnings[0])
}

export const summarizeDeviceConnectionDiagnostics = (
  responseOrData: unknown
): DeviceAccessGuideDiagnosticsSummary => {
  const data = unwrapDeviceConnectionDiagnosticsResponse(responseOrData)
  const recentLogs = Array.isArray(data.debug?.recent_logs) ? data.debug.recent_logs : []
  const partialWarnings = Array.isArray(data.partial_results)
    ? data.partial_results.map(normalizeDiagnosticsWarning).filter(Boolean)
    : []

  return {
    isOnline: data.online?.is_online,
    ready: data.ready_check?.ready,
    readyLevel: data.ready_check?.level,
    readyCode: data.ready_check?.code,
    readySummary: data.ready_check?.summary,
    readyNextActions: Array.isArray(data.ready_check?.next_actions) ? data.ready_check.next_actions : [],
    latestTelemetryKey: data.ready_check?.telemetry?.latest_key,
    latestTelemetryAt: data.ready_check?.telemetry?.latest_at,
    latestTelemetryValue: data.ready_check?.telemetry?.latest_value,
    telemetryCurrentCount: data.ready_check?.telemetry?.current_count,
    debugEnabled: data.debug?.enabled,
    recentLogCount: recentLogs.length,
    conclusionLevel: data.conclusion?.level,
    conclusionCode: data.conclusion?.code,
    conclusionSummary: data.conclusion?.summary,
    nextActions: Array.isArray(data.conclusion?.next_actions) ? data.conclusion.next_actions : [],
    latestIssue: resolveLatestConnectionIssue(data),
    partialWarnings
  }
}

export const getReadyCheckViewEvidence = (responseOrData: unknown) => {
  const data = unwrapDeviceConnectionDiagnosticsResponse(responseOrData)
  const readyActions = Array.isArray(data.ready_check?.next_actions) ? data.ready_check.next_actions : []
  const conclusionActions = Array.isArray(data.conclusion?.next_actions) ? data.conclusion.next_actions : []
  const recentLogs = Array.isArray(data.debug?.recent_logs) ? data.debug.recent_logs : []
  const recentFailures = Array.isArray(data.diagnostics?.recent_failures) ? data.diagnostics.recent_failures : []
  const partialWarnings = Array.isArray(data.partial_results) ? data.partial_results : []

  return {
    readyCheck: data.ready_check || {},
    conclusion: data.conclusion || {},
    latestTelemetry: data.ready_check?.telemetry || {},
    debug: {
      enabled: data.debug?.enabled,
      recentLogs
    },
    recentFailures,
    partialWarnings,
    nextActions: readyActions.length ? readyActions : conclusionActions
  }
}
