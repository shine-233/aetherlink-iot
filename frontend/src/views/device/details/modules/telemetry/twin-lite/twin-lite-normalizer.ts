type ExpectedMessageLike = {
  id?: string
  label?: string
  send_type?: string
  payload?: unknown
  status?: string
  created_at?: unknown
  expiry_time?: unknown
  desired_updated_at?: unknown
  desired_expires_at?: unknown
  desired_revision?: unknown
}

type ReportedTelemetryLike = {
  key?: string
  label?: string
  value?: unknown
  ts?: unknown
}

type ReportedAttributeLike = {
  key?: string
  value?: unknown
  ts?: unknown
}

export type TwinLiteSource = 'telemetry' | 'attribute' | 'command'

export type TwinLiteRow = {
  key: string
  label: string
  source: TwinLiteSource
  desired: unknown
  reported: unknown
  comparable: boolean
  matched: boolean
  status: string
  desired_updated_at?: string
  desired_expires_at?: string
  reported_at?: string
  desired_revision?: string
  last_write_source?: 'desired' | 'reported'
}

export type TwinLiteSummary = {
  desiredCount: number
  reportedCount: number
  matchedCount: number
  deltaCount: number
  unavailableCount: number
  staleDesiredCount?: number
  convergenceStatus?: TwinLiteConvergenceStatus
  nextAction?: string
  evidenceBoundary?: string
}

export type TwinLiteState = {
  rows: TwinLiteRow[]
  summary: TwinLiteSummary
}

export type TwinLiteEvidenceBundle = {
  schema_version: 'twin-lite-evidence-v1'
  device_id: string
  exported_at: string
  status: TwinLiteConvergenceStatus
  next_action: string
  evidence_boundary: string
  scope: {
    source: 'device-twin-workbench'
    rows: number
    platform_visible_evidence_only: true
  }
  summary: TwinLiteSummary
  rows: TwinLiteRow[]
}

export type TwinLiteConvergenceStatus = 'ready' | 'waiting_reported' | 'needs_review' | 'expired_desired' | 'no_desired'

export type TwinLiteNextAction =
  | 'safe_to_continue_after_review'
  | 'wait_for_reported_state'
  | 'compare_delta_before_device_action'
  | 'review_expired_desired_state'
  | 'create_desired_state'

const twinLiteConvergenceStatuses = new Set<TwinLiteConvergenceStatus>([
  'ready',
  'waiting_reported',
  'needs_review',
  'expired_desired',
  'no_desired'
])

const twinLiteNextActions = new Set<TwinLiteNextAction>([
  'safe_to_continue_after_review',
  'wait_for_reported_state',
  'compare_delta_before_device_action',
  'review_expired_desired_state',
  'create_desired_state'
])

export function normalizeTwinLiteConvergenceStatus(
  value: unknown,
  fallback: TwinLiteConvergenceStatus
): TwinLiteConvergenceStatus {
  return typeof value === 'string' && twinLiteConvergenceStatuses.has(value as TwinLiteConvergenceStatus)
    ? (value as TwinLiteConvergenceStatus)
    : fallback
}

export function normalizeTwinLiteNextAction(value: unknown, status: TwinLiteConvergenceStatus): TwinLiteNextAction {
  return typeof value === 'string' && twinLiteNextActions.has(value as TwinLiteNextAction)
    ? (value as TwinLiteNextAction)
    : getTwinLiteNextAction(status)
}

function stableStringify(value: unknown): string {
  if (value === null || value === undefined) return String(value)
  if (typeof value !== 'object') return JSON.stringify(value)
  if (Array.isArray(value)) return `[${value.map((item) => stableStringify(item)).join(',')}]`

  const record = value as Record<string, unknown>
  return `{${Object.keys(record)
    .sort()
    .map((key) => `${JSON.stringify(key)}:${stableStringify(record[key])}`)
    .join(',')}}`
}

function parsePayload(payload: unknown): unknown {
  if (typeof payload !== 'string') return payload

  const trimmed = payload.trim()
  if (!trimmed) return ''

  try {
    return JSON.parse(trimmed)
  } catch {
    return trimmed
  }
}

type TwinLiteReportedEntry = {
  value: unknown
  reportedAt?: string
}

function normalizeTwinTimestamp(value: unknown): string | undefined {
  if (value instanceof Date && !Number.isNaN(value.getTime())) return value.toISOString()
  if (typeof value === 'number' && Number.isFinite(value)) {
    const milliseconds = Math.abs(value) < 1_000_000_000_000 ? value * 1000 : value
    const parsed = new Date(milliseconds)
    return Number.isNaN(parsed.getTime()) ? undefined : parsed.toISOString()
  }
  if (typeof value !== 'string' || !value.trim()) return undefined
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? undefined : parsed.toISOString()
}

function normalizeTwinRevision(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

function resolveLastWriteSource(
  desiredUpdatedAt: string | undefined,
  reportedAt: string | undefined,
  hasReported: boolean
): 'desired' | 'reported' | undefined {
  if (!hasReported) return desiredUpdatedAt ? 'desired' : undefined
  if (!reportedAt) return undefined
  if (!desiredUpdatedAt) return 'reported'
  const desiredTime = Date.parse(desiredUpdatedAt)
  const reportedTime = Date.parse(reportedAt)
  if (!Number.isFinite(desiredTime) || !Number.isFinite(reportedTime) || desiredTime === reportedTime) return undefined
  return desiredTime > reportedTime ? 'desired' : 'reported'
}

function normalizeReportedTelemetryMap(items: ReportedTelemetryLike[]): Map<string, TwinLiteReportedEntry> {
  const map = new Map<string, TwinLiteReportedEntry>()
  for (const item of items) {
    const entry = { value: item?.value, reportedAt: normalizeTwinTimestamp(item?.ts) }
    if (item?.key) map.set(item.key, entry)
    if (item?.label && !map.has(item.label)) map.set(item.label, entry)
  }
  return map
}

function normalizeReportedAttributeMap(items: ReportedAttributeLike[]): Map<string, TwinLiteReportedEntry> {
  const map = new Map<string, TwinLiteReportedEntry>()
  for (const item of items) {
    if (item?.key) map.set(item.key, { value: item.value, reportedAt: normalizeTwinTimestamp(item.ts) })
  }
  return map
}

function appendObjectEntries(
  result: TwinLiteRow[],
  payload: Record<string, unknown>,
  source: TwinLiteSource,
  status: string,
  metadata: Pick<TwinLiteRow, 'desired_updated_at' | 'desired_expires_at' | 'desired_revision'>
) {
  for (const [key, value] of Object.entries(payload)) {
    result.push({
      key,
      label: key,
      source,
      desired: value,
      reported: null,
      comparable: source !== 'command',
      matched: false,
      status,
      ...metadata
    })
  }
}

function normalizeExpectedRows(items: ExpectedMessageLike[]): TwinLiteRow[] {
  const rows: TwinLiteRow[] = []

  for (const item of items) {
    const source = (item?.send_type || 'command') as TwinLiteSource
    const status = item?.status || 'pending'
    const parsedPayload = parsePayload(item?.payload)
    const metadata = {
      desired_updated_at: normalizeTwinTimestamp(item?.desired_updated_at ?? item?.created_at),
      desired_expires_at: normalizeTwinTimestamp(item?.desired_expires_at ?? item?.expiry_time),
      desired_revision: normalizeTwinRevision(item?.desired_revision ?? item?.id)
    }

    if (
      (source === 'telemetry' || source === 'attribute') &&
      parsedPayload &&
      typeof parsedPayload === 'object' &&
      !Array.isArray(parsedPayload)
    ) {
      appendObjectEntries(rows, parsedPayload as Record<string, unknown>, source, status, metadata)
      continue
    }

    const fallbackKey = item?.label || item?.id || `${source}-${rows.length + 1}`
    rows.push({
      key: fallbackKey,
      label: item?.label || fallbackKey,
      source,
      desired: parsedPayload,
      reported: null,
      comparable: source !== 'command',
      matched: false,
      status,
      ...metadata
    })
  }

  return rows
}

export function buildTwinLiteState(
  expectedMessages: ExpectedMessageLike[],
  reportedTelemetry: ReportedTelemetryLike[],
  reportedAttributes: ReportedAttributeLike[]
): TwinLiteState {
  const desiredRows = normalizeExpectedRows(expectedMessages)
  const telemetryMap = normalizeReportedTelemetryMap(reportedTelemetry)
  const attributeMap = normalizeReportedAttributeMap(reportedAttributes)

  const rows = desiredRows.map((row) => {
    let reportedEntry: TwinLiteReportedEntry | undefined
    if (row.source === 'telemetry') reportedEntry = telemetryMap.get(row.key) ?? telemetryMap.get(row.label)
    if (row.source === 'attribute') reportedEntry = attributeMap.get(row.key)
    const reported = reportedEntry?.value ?? null
    const reportedAt = reportedEntry?.reportedAt

    const matched = row.comparable && stableStringify(row.desired) === stableStringify(reported)
    return {
      ...row,
      reported,
      matched,
      reported_at: reportedAt,
      last_write_source: resolveLastWriteSource(row.desired_updated_at, reportedAt, Boolean(reportedEntry))
    }
  })

  const matchedCount = rows.filter((row) => row.matched).length
  const unavailableCount = rows.filter((row) => row.comparable && row.reported === null).length
  const deltaCount = rows.filter((row) => row.comparable && !row.matched).length
  const reportedCount = new Set([...Array.from(telemetryMap.keys()), ...Array.from(attributeMap.keys())]).size
  const convergenceStatus = getTwinLiteConvergenceStatus(rows.length, deltaCount, unavailableCount)

  return {
    rows,
    summary: {
      desiredCount: rows.length,
      reportedCount,
      matchedCount,
      deltaCount,
      unavailableCount,
      staleDesiredCount: 0,
      convergenceStatus,
      nextAction: getTwinLiteNextAction(convergenceStatus),
      evidenceBoundary: 'platform_visible_evidence_only'
    }
  }
}

export function getTwinLiteConvergenceStatus(
  desiredCount: number,
  deltaCount: number,
  unavailableCount: number
): TwinLiteConvergenceStatus {
  if (desiredCount === 0) return 'no_desired'
  if (deltaCount > 0) return 'needs_review'
  if (unavailableCount > 0) return 'waiting_reported'
  return 'ready'
}

export function getTwinLiteNextAction(status: TwinLiteConvergenceStatus): TwinLiteNextAction {
  if (status === 'no_desired') return 'create_desired_state'
  if (status === 'expired_desired') return 'review_expired_desired_state'
  if (status === 'needs_review') return 'compare_delta_before_device_action'
  if (status === 'waiting_reported') return 'wait_for_reported_state'
  return 'safe_to_continue_after_review'
}

export function buildTwinLiteEvidenceBundle(input: {
  deviceId: string
  exportedAt: string
  state: TwinLiteState
  status: TwinLiteConvergenceStatus
  nextAction: string
  evidenceBoundary: string
}): TwinLiteEvidenceBundle {
  return {
    schema_version: 'twin-lite-evidence-v1',
    device_id: input.deviceId,
    exported_at: input.exportedAt,
    status: input.status,
    next_action: input.nextAction,
    evidence_boundary: input.evidenceBoundary,
    scope: {
      source: 'device-twin-workbench',
      rows: input.state.rows.length,
      platform_visible_evidence_only: true
    },
    summary: { ...input.state.summary },
    rows: input.state.rows.map((row) => ({ ...row }))
  }
}

export function buildTwinLiteEvidenceFileName(deviceId: string, exportedAt: string) {
  const safeDeviceId = (deviceId || 'device').replace(/[^a-zA-Z0-9_-]+/g, '-').replace(/^-+|-+$/g, '') || 'device'
  const safeTimestamp = exportedAt.replace(/[:.]/g, '-')
  return `device-twin-evidence-${safeDeviceId}-${safeTimestamp}.json`
}
