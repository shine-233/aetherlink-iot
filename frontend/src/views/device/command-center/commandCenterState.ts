import type { FleetCommandJobPayload, FleetCommandJobPreviewBlocker, FleetCommandJobPreviewRow } from '@/service/api/device'
import { getDeviceFilterLabel, getDeviceFilterValueLabel } from '@/views/device/shared/device-filter-summary-labels'

export type QueryValue = string | string[] | null | undefined
export type CommandCenterRouteQuery = Record<string, QueryValue>
export type FleetCommandScopeType = 'selected_devices' | 'device_filter'
export type DeviceFilterPayload = Record<string, string | number | boolean>
export const DEFAULT_FILTER_JOB_MAX_DEVICES = 200
export const DEFAULT_FILTER_JOB_SUBSET_LIMIT = 20
export const MAX_COMMAND_JOB_SCHEDULE_AHEAD_MS = 365 * 24 * 60 * 60 * 1000

export interface CommandCenterScopeContext {
  source: string
  routeScope: string
  scopeType: FleetCommandScopeType
  deviceIds: string[]
  deviceFilter: DeviceFilterPayload
  requestedTotal: number | null
  currentPageCount: number | null
  savedFilterId: string
  savedFilterName: string
}

export interface CommandCenterFilterSummaryItem {
  key: string
  label: string
  value: string
}

export interface FilteredFleetEligibilityPreview {
  coverage: 'full' | 'subset_only'
  alertType: 'success' | 'warning'
  messageKey: string
  requestedCount: number
  shownCount: number
  totalMatched: number
  subsetEligibleCount: number
  subsetBlockedCount: number
  immediateCount: number
  jobsCount: number
  blockedPathCount: number
  telemetryCount: number
}

export interface CommandJobPreviewActionPlan {
  cards: Array<{
    key: 'immediate' | 'jobs' | 'blocked' | 'telemetry'
    labelKey: string
    value: number
    type: 'default' | 'error' | 'info' | 'success' | 'warning'
  }>
  blockers: FleetCommandJobPreviewBlocker[]
  nextAction: string
}

export interface CommandJobEligibilityImpactRepresentative {
  key: string
  device: string
  reason: string
  advice: string
}

export interface CommandJobEligibilityImpactGroup {
  key: 'eligible' | 'immediate' | 'jobs' | 'blocked'
  labelKey: string
  descriptionKey: string
  count: number
  type: 'default' | 'error' | 'info' | 'success' | 'warning'
  representativeRows: CommandJobEligibilityImpactRepresentative[]
}

export interface CommandJobEligibilityImpactPreview {
  coverage: 'full' | 'subset_only'
  coverageLabelKey: string
  coverageType: 'success' | 'warning'
  requestedCount: number
  shownCount: number
  groups: CommandJobEligibilityImpactGroup[]
  nextAction: string
}

export type CommandJobEligibilityImpactGroupKey = CommandJobEligibilityImpactGroup['key']
export type CommandJobEligibilityImpactFilterKey = CommandJobEligibilityImpactGroupKey | 'all'

const FLEET_QUERY_KEYS = new Set([
  'fleet_source',
  'fleet_scope',
  'fleet_requested_total',
  'fleet_current_page_count',
  'fleet_selected_count',
  'saved_filter_id',
  'saved_filter_name',
  'device_ids',
  'first_device_id',
  'command_job_id',
  'job_id'
])

const DEVICE_FILTER_QUERY_KEYS = new Set([
  'device_number',
  'is_enabled',
  'product_id',
  'label',
  'name',
  'current_version',
  'pid_number',
  'firmware_version',
  'description',
  'shared_status',
  'group_id',
  'device_config_id',
  'device_template_id',
  'is_online',
  'last_reported_after',
  'last_reported_before',
  'never_reported',
  'lifecycle_status',
  'warn_status',
  'search',
  'access_way',
  'batch_number',
  'device_type',
  'service_identifier',
  'service_access_id'
])

const DEVICE_FILTER_NUMERIC_KEYS = new Set(['is_online', 'last_reported_after', 'last_reported_before'])
const DEVICE_FILTER_BOOLEAN_KEYS = new Set(['never_reported'])
export function normalizeQueryValue(value: QueryValue) {
  if (Array.isArray(value)) return value.join(',')
  return value || ''
}

export function normalizeQueryNumber(value: QueryValue) {
  const raw = normalizeQueryValue(value)
  if (!raw) return null
  const parsed = Number(raw)
  return Number.isFinite(parsed) ? parsed : null
}

export function parseDeviceIds(value: QueryValue) {
  return normalizeQueryValue(value)
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseJsonFilter(value: QueryValue): DeviceFilterPayload {
  const raw = normalizeQueryValue(value).trim()
  if (!raw) return {}

  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const filter: DeviceFilterPayload = {}
    Object.entries(parsed).forEach(([key, item]) => {
      if (!DEVICE_FILTER_QUERY_KEYS.has(key) || item === null || item === undefined || item === '') return
      if (DEVICE_FILTER_BOOLEAN_KEYS.has(key)) {
        if (typeof item === 'boolean') filter[key] = item
        else if (item === 0 || item === 1) filter[key] = item === 1
        return
      }
      if (DEVICE_FILTER_NUMERIC_KEYS.has(key)) {
        const numericValue = Number(item)
        if (Number.isFinite(numericValue)) filter[key] = numericValue
        return
      }
      if (typeof item === 'string' && item.trim()) filter[key] = item.trim()
    })
    return filter
  } catch {
    return {}
  }
}

export function buildDeviceFilterFromQuery(query: CommandCenterRouteQuery): DeviceFilterPayload {
  const filter: DeviceFilterPayload = { ...parseJsonFilter(query.device_filter) }

  Object.entries(query).forEach(([key, value]) => {
    if (FLEET_QUERY_KEYS.has(key) || !DEVICE_FILTER_QUERY_KEYS.has(key)) return
    const normalized = normalizeQueryValue(value).trim()
    if (!normalized) return
    if (DEVICE_FILTER_BOOLEAN_KEYS.has(key)) {
      if (normalized === 'true' || normalized === '1') filter[key] = true
      else if (normalized === 'false' || normalized === '0') filter[key] = false
      return
    }
    if (DEVICE_FILTER_NUMERIC_KEYS.has(key)) {
      const numericValue = Number(normalized)
      if (Number.isFinite(numericValue)) filter[key] = numericValue
      return
    }
    filter[key] = normalized
  })

  return Object.fromEntries(
    Object.entries(filter).filter(([, value]) => {
      if (typeof value === 'number') return Number.isFinite(value)
      if (typeof value === 'boolean') return true
      return Boolean(value)
    })
  )
}

export function parseCommandCenterScopeContext(query: CommandCenterRouteQuery): CommandCenterScopeContext {
  const deviceIds = parseDeviceIds(query.device_ids)
  const deviceFilter = buildDeviceFilterFromQuery(query)
  const routeScope = normalizeQueryValue(query.fleet_scope) || 'selected_devices'
  const hasDeviceFilter = Object.keys(deviceFilter).length > 0
  const scopeType: FleetCommandScopeType =
    hasDeviceFilter && routeScope !== 'selected_devices' ? 'device_filter' : 'selected_devices'

  return {
    source: normalizeQueryValue(query.fleet_source) || 'device_manage',
    routeScope,
    scopeType,
    deviceIds,
    deviceFilter,
    requestedTotal: normalizeQueryNumber(query.fleet_requested_total),
    currentPageCount: normalizeQueryNumber(query.fleet_current_page_count),
    savedFilterId: normalizeQueryValue(query.saved_filter_id),
    savedFilterName: normalizeQueryValue(query.saved_filter_name)
  }
}

export function buildCommandCenterFilterSummaryItems(
  deviceFilter?: DeviceFilterPayload | null
): CommandCenterFilterSummaryItem[] {
  if (!deviceFilter) return []

  return Object.entries(deviceFilter)
    .filter(([, value]) => value !== '' && value !== null && value !== undefined)
    .map(([key, value]) => ({
      key,
      label: getDeviceFilterLabel(key),
      value: getDeviceFilterValueLabel(key, value)
    }))
}

export function buildFleetCommandPayload(input: {
  deviceIds: string[]
  scopeType?: FleetCommandScopeType
  deviceFilter?: DeviceFilterPayload
  expectedTotal?: number | null
  currentPageCount?: number | null
  source?: string
  identify: string
  value?: string
  timeoutSeconds?: number | null
  scheduledAt?: number | null
  maxDevices?: number | null
  subsetLimit?: number | null
}): FleetCommandJobPayload {
  const scheduledAt =
    typeof input.scheduledAt === 'number' && Number.isFinite(input.scheduledAt)
      ? new Date(input.scheduledAt).toISOString()
      : undefined
  const basePayload = {
    identify: input.identify.trim(),
    value: input.value?.trim() || undefined,
    timeout_seconds: input.timeoutSeconds || 60,
    scheduled_at: scheduledAt
  }

  if (input.scopeType === 'device_filter') {
    return {
      ...basePayload,
      scope_type: 'device_filter',
      device_filter: input.deviceFilter || {},
      expected_total: input.expectedTotal ?? undefined,
      current_page_count: input.currentPageCount ?? undefined,
      scope_source: input.source || undefined,
      max_devices: input.maxDevices || DEFAULT_FILTER_JOB_MAX_DEVICES,
      subset_limit: input.subsetLimit || DEFAULT_FILTER_JOB_SUBSET_LIMIT,
      sample_limit: input.subsetLimit || DEFAULT_FILTER_JOB_SUBSET_LIMIT
    }
  }

  return {
    device_ids: input.deviceIds,
    scope_type: 'selected_devices',
    ...basePayload
  }
}

export function serializeFleetCommandPayload(payload: FleetCommandJobPayload) {
  return JSON.stringify(payload)
}

export function getFleetCommandPayloadValidationKey(input: {
  hasSelectedDevices: boolean
  hasDeviceFilter?: boolean
  scopeType?: FleetCommandScopeType
  identify: string
  scheduledAt?: number | null
  nowMs?: number
}) {
  if (input.scopeType === 'device_filter' && !input.hasDeviceFilter) return 'custom.commandCenter.noFilterScope'
  if (input.scopeType !== 'device_filter' && !input.hasSelectedDevices) return 'custom.commandCenter.noSelection'
  if (!input.identify.trim()) return 'custom.commandCenter.commandIdentifierRequired'
  if (input.scheduledAt !== null && input.scheduledAt !== undefined) {
    if (!Number.isFinite(input.scheduledAt)) return 'custom.commandCenter.scheduleInvalid'
    const nowMs = input.nowMs ?? Date.now()
    if (input.scheduledAt <= nowMs) return 'custom.commandCenter.scheduleMustBeFuture'
    if (input.scheduledAt > nowMs + MAX_COMMAND_JOB_SCHEDULE_AHEAD_MS) {
      return 'custom.commandCenter.scheduleTooFar'
    }
  }
  return ''
}

export function normalizeApiData<T>(response: T | { data?: T }): NonNullable<T> {
  const data = (response as any)?.data ?? response
  if (data === null || data === undefined) {
    // The shared request wrapper resolves `data` as nullable; a null envelope
    // means the backend returned no payload, which every caller treats as a
    // failed load. Throw so the surrounding try/catch surfaces it instead of
    // proceeding with a null result.
    throw new Error('empty API response')
  }
  return data as NonNullable<T>
}

export function buildRouteDecisionSummary(rows: FleetCommandJobPreviewRow[] = []) {
  return rows.reduce(
    (summary, row) => {
      if (row.recommended_path === 'immediate') summary.immediate += 1
      else if (row.recommended_path === 'jobs') summary.jobs += 1
      else summary.blocked += 1
      if (row.telemetry_current_count) summary.telemetry += 1
      return summary
    },
    { immediate: 0, jobs: 0, blocked: 0, telemetry: 0 }
  )
}

export function buildCommandJobPreviewActionPlan(input: {
  previewResult?: {
    rows: FleetCommandJobPreviewRow[]
    path_counts?: {
      immediate: number
      jobs: number
      blocked: number
      telemetry: number
    }
    blockers?: FleetCommandJobPreviewBlocker[]
    next_action?: string
  } | null
  fallbackNextAction: string
}): CommandJobPreviewActionPlan | null {
  const preview = input.previewResult
  if (!preview) return null

  const fallbackCounts = buildRouteDecisionSummary(preview.rows)
  const counts = preview.path_counts || fallbackCounts
  const fallbackBlockers = preview.rows
    .filter(row => !row.eligible || row.recommended_path === 'blocked')
    .reduce<FleetCommandJobPreviewBlocker[]>((items, row) => {
      const reason = row.reason || row.status || 'Preview row is blocked.'
      const existing = items.find(item => item.reason === reason && item.advice === row.advice)
      if (existing) existing.count += 1
      else items.push({ reason, advice: row.advice, count: 1 })
      return items
    }, [])
    .sort((left, right) => right.count - left.count || left.reason.localeCompare(right.reason))
    .slice(0, 5)

  return {
    cards: [
      { key: 'immediate', labelKey: 'custom.commandCenter.previewPlanImmediate', value: counts.immediate, type: 'success' },
      { key: 'jobs', labelKey: 'custom.commandCenter.previewPlanJobs', value: counts.jobs, type: 'info' },
      { key: 'blocked', labelKey: 'custom.commandCenter.previewPlanBlocked', value: counts.blocked, type: counts.blocked > 0 ? 'warning' : 'default' },
      { key: 'telemetry', labelKey: 'custom.commandCenter.previewPlanTelemetry', value: counts.telemetry, type: 'default' }
    ],
    blockers: preview.blockers?.length ? preview.blockers.slice(0, 5) : fallbackBlockers,
    nextAction: preview.next_action || input.fallbackNextAction
  }
}

const commandJobPreviewRowKey = (row: FleetCommandJobPreviewRow, index: number) =>
  row.device_id || row.device_number || row.name || `preview-row-${index}`

const commandJobPreviewRowDevice = (row: FleetCommandJobPreviewRow) =>
  row.device_number || row.name || row.device_id || '-'

const commandJobPreviewRowReason = (row: FleetCommandJobPreviewRow) =>
  row.reason || row.status || row.readiness?.filter(Boolean).join(', ') || '-'

const commandJobPreviewRowAdvice = (row: FleetCommandJobPreviewRow) => row.advice || '-'

const commandJobPreviewRepresentatives = (rows: FleetCommandJobPreviewRow[]): CommandJobEligibilityImpactRepresentative[] =>
  rows.slice(0, 3).map((row, index) => ({
    key: commandJobPreviewRowKey(row, index),
    device: commandJobPreviewRowDevice(row),
    reason: commandJobPreviewRowReason(row),
    advice: commandJobPreviewRowAdvice(row)
  }))

export function filterCommandJobPreviewRowsByImpactGroup(
  rows: FleetCommandJobPreviewRow[],
  groupKey: CommandJobEligibilityImpactFilterKey
): FleetCommandJobPreviewRow[] {
  if (groupKey === 'all') return rows
  if (groupKey === 'eligible') return rows.filter(row => row.eligible)
  if (groupKey === 'immediate') return rows.filter(row => row.eligible && row.recommended_path === 'immediate')
  if (groupKey === 'jobs') return rows.filter(row => row.eligible && row.recommended_path === 'jobs')
  return rows.filter(row => !row.eligible || row.recommended_path === 'blocked')
}

export function buildCommandJobEligibilityImpactSummaryText(
  preview: CommandJobEligibilityImpactPreview | null | undefined,
  translate: (key: string) => string = key => key
): string {
  if (!preview) return ''

  const lines = [
    translate('custom.commandCenter.impactPreviewTitle'),
    `${translate(preview.coverageLabelKey)}: ${preview.shownCount}/${preview.requestedCount}`,
    `${translate('custom.commandCenter.nextAction')}: ${preview.nextAction}`
  ]

  preview.groups.forEach(group => {
    lines.push(`${translate(group.labelKey)}: ${group.count}`)
    group.representativeRows.forEach(row => {
      lines.push(`- ${row.device}: ${row.reason}; ${row.advice}`)
    })
  })

  return lines.join('\n')
}

export function buildCommandJobEligibilityImpactPreview(input: {
  isDeviceFilterScope: boolean
  previewResult?: {
    requested_count: number
    rows: FleetCommandJobPreviewRow[]
    next_action?: string
  } | null
  fallbackNextAction: string
}): CommandJobEligibilityImpactPreview | null {
  const preview = input.previewResult
  if (!preview) return null

  const rows = Array.isArray(preview.rows) ? preview.rows : []
  const requestedCount = preview.requested_count || rows.length
  const shownCount = rows.length
  const coverage = !input.isDeviceFilterScope || shownCount >= requestedCount ? 'full' : 'subset_only'
  const eligibleRows = rows.filter(row => row.eligible)
  const immediateRows = rows.filter(row => row.eligible && row.recommended_path === 'immediate')
  const jobRows = rows.filter(row => row.eligible && row.recommended_path === 'jobs')
  const blockedRows = rows.filter(row => !row.eligible || row.recommended_path === 'blocked')

  return {
    coverage,
    coverageLabelKey:
      coverage === 'full'
        ? 'custom.commandCenter.impactPreviewFullCoverage'
        : 'custom.commandCenter.impactPreviewSubsetCoverage',
    coverageType: coverage === 'full' ? 'success' : 'warning',
    requestedCount,
    shownCount,
    nextAction: preview.next_action || input.fallbackNextAction,
    groups: [
      {
        key: 'eligible',
        labelKey: 'custom.commandCenter.impactPreviewEligible',
        descriptionKey: 'custom.commandCenter.impactPreviewEligibleDesc',
        count: eligibleRows.length,
        type: eligibleRows.length > 0 ? 'success' : 'warning',
        representativeRows: commandJobPreviewRepresentatives(eligibleRows)
      },
      {
        key: 'immediate',
        labelKey: 'custom.commandCenter.impactPreviewImmediate',
        descriptionKey: 'custom.commandCenter.impactPreviewImmediateDesc',
        count: immediateRows.length,
        type: immediateRows.length > 0 ? 'success' : 'default',
        representativeRows: commandJobPreviewRepresentatives(immediateRows)
      },
      {
        key: 'jobs',
        labelKey: 'custom.commandCenter.impactPreviewJobs',
        descriptionKey: 'custom.commandCenter.impactPreviewJobsDesc',
        count: jobRows.length,
        type: jobRows.length > 0 ? 'info' : 'default',
        representativeRows: commandJobPreviewRepresentatives(jobRows)
      },
      {
        key: 'blocked',
        labelKey: 'custom.commandCenter.impactPreviewBlocked',
        descriptionKey: 'custom.commandCenter.impactPreviewBlockedDesc',
        count: blockedRows.length,
        type: blockedRows.length > 0 ? 'error' : 'success',
        representativeRows: commandJobPreviewRepresentatives(blockedRows)
      }
    ]
  }
}

export function buildFilteredFleetEligibilityPreview(input: {
  isDeviceFilterScope: boolean
  previewResult?: {
    total_matched?: number
    requested_count: number
    rows: FleetCommandJobPreviewRow[]
  } | null
}): FilteredFleetEligibilityPreview | null {
  const preview = input.previewResult
  if (!preview) return null

  const rows = Array.isArray(preview.rows) ? preview.rows : []
  const requestedCount = preview.requested_count || rows.length
  const shownCount = rows.length
  const totalMatched = typeof preview.total_matched === 'number' ? preview.total_matched : requestedCount
  const routeDecision = buildRouteDecisionSummary(rows)
  const coverage = !input.isDeviceFilterScope || shownCount >= requestedCount ? 'full' : 'subset_only'

  return {
    coverage,
    alertType: coverage === 'full' ? 'success' : 'warning',
    messageKey:
      coverage === 'full'
        ? 'custom.commandCenter.filteredPreviewFullScope'
        : 'custom.commandCenter.filteredPreviewSubsetOnlyScope',
    requestedCount,
    shownCount,
    totalMatched,
    subsetEligibleCount: rows.filter(row => row.eligible).length,
    subsetBlockedCount: rows.filter(row => !row.eligible).length,
    immediateCount: routeDecision.immediate,
    jobsCount: routeDecision.jobs,
    blockedPathCount: routeDecision.blocked,
    telemetryCount: routeDecision.telemetry
  }
}

export function getRecommendedPathLabelKey(path?: FleetCommandJobPreviewRow['recommended_path']) {
  if (path === 'immediate') return 'custom.commandCenter.pathImmediate'
  if (path === 'jobs') return 'custom.commandCenter.pathJobs'
  return 'custom.commandCenter.pathBlocked'
}
