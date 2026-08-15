/**
 * Fleet command job and saved-filter API wrappers.
 *
 * Keep this module focused on batch command execution evidence: preview,
 * submit, progress/detail, support bundle, retry/cancel, and saved filters.
 * `device.ts` re-exports these contracts for existing callers.
 */
import { request } from '../request'

export interface FleetCommandJobPayload {
  device_ids?: string[]
  scope_type?: 'selected_devices' | 'device_filter'
  device_filter?: Record<string, string | number | boolean>
  expected_total?: number
  current_page_count?: number
  scope_source?: string
  max_devices?: number
  subset_limit?: number
  /** Compatibility field accepted by older backends. Prefer subset_limit in new callers. */
  sample_limit?: number
  identify: string
  value?: string
  timeout_seconds?: number
  scheduled_at?: string
  preview_token?: string
}

export interface FleetSavedFilterPayload {
  id?: string
  name: string
  device_filter: Record<string, unknown>
  preview_total?: number | null
  shared?: boolean
}

export interface FleetSavedFilterItem {
  id: string
  name: string
  device_filter: Record<string, unknown>
  preview_total?: number | null
  created_at?: string
  updated_at?: string
  shared?: boolean
  owned?: boolean
  owner_user_id?: string
}

export interface FleetSavedFilterListResult {
  list: FleetSavedFilterItem[]
}

export interface FleetCommandJobPreviewDevice {
  id: string
  device_number?: string
  name?: string
  is_online?: number
  warn_status?: string
  current_version?: string
  firmware_version?: string
  device_config_name?: string
}

export interface FleetCommandJobPreviewRow {
  device_id: string
  device_number?: string
  name?: string
  online: boolean
  eligible: boolean
  status: string
  recommended_path?: 'immediate' | 'jobs' | 'blocked'
  readiness?: string[]
  telemetry_current_count?: number
  latest_telemetry_key?: string
  latest_telemetry_at?: string
  advice?: string
  reason?: string
}

export interface FleetCommandJobPreviewPathCounts {
  immediate: number
  jobs: number
  blocked: number
  telemetry: number
}

export interface FleetCommandJobPreviewBlocker {
  reason: string
  advice?: string
  count: number
}

export interface FleetCommandJobPreviewResult {
  job_type: string
  scope_type: string
  preview_token: string
  total_matched?: number
  requested_count: number
  eligible_count: number
  blocked_count: number
  timeout_seconds: number
  rows: FleetCommandJobPreviewRow[]
  preview_devices?: FleetCommandJobPreviewDevice[]
  /** Compatibility field returned by older backends. Prefer preview_devices in new callers. */
  sample_devices?: FleetCommandJobPreviewDevice[]
  warnings?: string[]
  scope_limits?: string[]
  path_counts?: FleetCommandJobPreviewPathCounts
  blockers?: FleetCommandJobPreviewBlocker[]
  next_action?: string
  governance_summary?: FleetCommandJobGovernanceSummary
}

export interface FleetCommandJobSubmitRow {
  detail_id?: string
  device_id: string
  device_number?: string
  name?: string
  eligible: boolean
  status: string
  readiness?: string[]
  message_id?: string
  dispatch_attempts?: number
  max_dispatch_attempts?: number
  retry_state?: 'retryable' | 'waiting_backoff' | 'max_attempts_reached' | 'not_retryable' | string
  last_dispatch_started_at?: string
  next_retry_after?: string
  response_recorded?: boolean
  response_status?: string
  response_status_label?: string
  response_data?: string
  response_error?: string
  command_log_created_at?: string
  log_recorded?: boolean
  reason?: string
  advice?: string
  can_retry: boolean
  recommended_path?: 'immediate' | 'jobs' | 'blocked'
  telemetry_current_count?: number
  latest_telemetry_key?: string
  latest_telemetry_at?: string
  submitted_at?: string
  completed_at?: string
}

export interface FleetCommandJobEvent {
  id: string
  event_type: string
  detail_id?: string
  device_id?: string
  message?: string
  created_at?: string
}

export interface FleetCommandJobProgressHealth {
  state: 'scheduled' | 'running' | 'timeout_risk' | 'timed_out' | 'needs_attention' | 'canceled' | 'complete' | string
  pending_count: number
  terminal_count: number
  elapsed_seconds: number
  timeout_remaining_seconds: number
  next_action: string
}

export interface FleetCommandJobAuditSummary {
  event_count: number
  latest_event_type?: string
  latest_event_at?: string
  latest_message?: string
  next_action: string
}

export interface FleetCommandJobExecutionSummary {
  path_type: 'single_device_command' | 'fleet_job' | string
  path_label: string
  decision:
    | 'monitor'
    | 'retry'
    | 'wait'
    | 'wait_schedule'
    | 'support'
    | 'collect_evidence'
    | 'watch_timeout'
    | 'canceled'
    | 'close'
    | string
  can_close?: boolean
  close_blockers?: string[]
  next_action: string
  evidence?: string[]
  checklist?: FleetCommandJobExecutionChecklistItem[]
}

export interface FleetCommandJobExecutionChecklistItem {
  key: string
  label: string
  state: 'done' | 'todo' | 'watch' | 'blocked' | string
  detail?: string
}

export interface FleetCommandJobGovernanceSummary {
  level: 'success' | 'info' | 'warning' | 'error' | string
  title: string
  summary: string
  next_action: string
  items?: FleetCommandJobGovernanceItem[]
}

export interface FleetCommandJobGovernanceItem {
  key: string
  label: string
  value: string
  state: 'done' | 'watch' | 'blocked' | string
  detail?: string
}

export interface FleetCommandJobSubmitResult {
  job_id: string
  job_type: string
  scope_type: string
  preview_token?: string
  status: string
  audit_remark?: string
  requested_count: number
  eligible_count: number
  blocked_count: number
  submitted_count: number
  failed_count: number
  retryable_count?: number
  retry_ready_count?: number
  retry_waiting_count?: number
  retry_exhausted_count?: number
  log_missing_count?: number
  timeout_seconds: number
  can_cancel: boolean
  can_retry_failed: boolean
  rows: FleetCommandJobSubmitRow[]
  rows_total?: number
  rows_truncated?: boolean
  events?: FleetCommandJobEvent[]
  status_counts?: Record<string, number>
  progress_health?: FleetCommandJobProgressHealth
  handoff_summary?: string
  audit_summary?: FleetCommandJobAuditSummary
  execution_summary?: FleetCommandJobExecutionSummary
  governance_summary?: FleetCommandJobGovernanceSummary
  created_at?: string
  updated_at?: string
  scheduled_at?: string
  next_dispatch_at?: string
  timeout_at?: string
  warnings?: string[]
  scope_limits?: string[]
}

export interface FleetCommandJobRowsResult {
  total: number
  page: number
  page_size: number
  status_filter?: CommandJobRowsStatusFilter
  search?: string
  rows: FleetCommandJobSubmitRow[]
  rows_truncated: boolean
}

export type CommandJobRowsStatusFilter =
  | 'all'
  | 'needs_attention'
  | 'retryable'
  | 'retry_ready'
  | 'retry_waiting'
  | 'retry_exhausted'
  | 'device_failed'
  | 'failed'
  | 'missing_log'
  | 'in_progress'
  | 'canceled'

export interface FleetCommandJobSupportDevice {
  detail_id?: string
  device_id: string
  device_number?: string
  name?: string
  status: string
  readiness?: string[]
  message_id?: string
  dispatch_attempts?: number
  max_dispatch_attempts?: number
  retry_state?: 'retryable' | 'waiting_backoff' | 'max_attempts_reached' | 'not_retryable' | string
  next_retry_after?: string
  response_status?: string
  response_status_label?: string
  response_data?: string
  response_error?: string
  response_at?: string
  reason?: string
  advice?: string
  ready_check_url?: string
  job_detail_url?: string
  diagnostic_summary?: FleetCommandJobSupportDiagnostic
}

export interface FleetCommandJobSupportDiagnostic {
  level: 'ok' | 'info' | 'warning' | 'error' | string
  code: string
  summary: string
  evidence?: string[]
  next_actions?: string[]
}

export interface FleetCommandJobSupportBundle {
  job_id: string
  job_type: string
  scope_type: string
  identify: string
  command_value?: string
  timeout_seconds: number
  status: string
  scheduled_at?: string
  next_dispatch_at?: string
  audit_remark?: string
  requested_count: number
  eligible_count: number
  blocked_count: number
  submitted_count: number
  failed_count: number
  retryable_count: number
  retry_ready_count: number
  retry_waiting_count: number
  retry_exhausted_count: number
  log_missing_count: number
  status_counts?: Record<string, number>
  retryable_device_ids?: string[]
  missing_log_device_ids?: string[]
  failed_devices?: FleetCommandJobSupportDevice[]
  events?: FleetCommandJobEvent[]
  execution_summary?: FleetCommandJobExecutionSummary
  governance_summary?: FleetCommandJobGovernanceSummary
  next_actions: string[]
  generated_at: string
  share_hint: string
}

export interface FleetCommandJobListItem {
  job_id: string
  job_type: string
  scope_type: string
  identify: string
  command_value?: string
  timeout_seconds: number
  status: string
  audit_remark?: string
  requested_count: number
  eligible_count: number
  blocked_count: number
  submitted_count: number
  failed_count: number
  retryable_count?: number
  retry_ready_count?: number
  retry_waiting_count?: number
  retry_exhausted_count?: number
  log_missing_count?: number
  device_ack_failed_count?: number
  needs_operator_action?: boolean
  needs_operator_action_count?: number
  can_cancel: boolean
  can_retry_failed: boolean
  created_at?: string
  updated_at?: string
  scheduled_at?: string
  next_dispatch_at?: string
  timeout_at?: string
}

export interface FleetCommandJobListAttentionCounts {
  retryable_count?: number
  retry_ready_count?: number
  retry_waiting_count?: number
  retry_exhausted_count?: number
  log_missing_count?: number
  device_ack_failed_count?: number
  blocked_count?: number
  needs_operator_action_count?: number
}

export interface FleetCommandJobListResult {
  total: number
  search?: string
  attention_filter?: string
  attention_counts?: FleetCommandJobListAttentionCounts
  list: FleetCommandJobListItem[]
}

export const previewFleetCommandJob = async (params: FleetCommandJobPayload) => {
  return await request.post<FleetCommandJobPreviewResult>(`/command/datas/jobs/preview`, params, {
    silentError: true
  } as any)
}

export const submitFleetCommandJob = async (params: FleetCommandJobPayload, query?: { include_rows?: boolean }) => {
  return await request.post<FleetCommandJobSubmitResult>(`/command/datas/jobs/submit`, params, {
    ...(query ? { params: query } : {}),
    silentError: true
  } as any)
}

export const listFleetCommandJobs = async (params?: {
  page?: number
  page_size?: number
  status?: string
  attention_filter?: string
  search?: string
}) => {
  return await request.get<FleetCommandJobListResult>(`/command/datas/jobs`, {
    params,
    silentError: true
  } as any)
}

export const getFleetCommandJob = async (jobId: string) => {
  return await request.get<FleetCommandJobSubmitResult>(`/command/datas/jobs/${jobId}`, {
    params: { include_rows: true },
    silentError: true
  } as any)
}

export const getFleetCommandJobSummary = async (jobId: string) => {
  return await request.get<FleetCommandJobSubmitResult>(`/command/datas/jobs/${jobId}`, {
    params: { include_rows: false },
    silentError: true
  } as any)
}

export const getFleetCommandJobRows = async (
  jobId: string,
  params?: { page?: number; page_size?: number; status_filter?: CommandJobRowsStatusFilter; search?: string }
) => {
  return await request.get<FleetCommandJobRowsResult>(`/command/datas/jobs/${jobId}/rows`, {
    params,
    silentError: true
  } as any)
}

export const getFleetCommandJobSupportBundle = async (jobId: string) => {
  return await request.get<FleetCommandJobSupportBundle>(`/command/datas/jobs/${jobId}/support-bundle`, {
    silentError: true
  } as any)
}

export const cancelFleetCommandJob = async (jobId: string, params?: { include_rows?: boolean }) => {
  return await request.post<FleetCommandJobSubmitResult>(`/command/datas/jobs/${jobId}/cancel`, {}, {
    ...(params ? { params } : {}),
    silentError: true
  } as any)
}

export const retryFleetCommandJob = async (jobId: string, params?: { include_rows?: boolean }) => {
  return await request.post<FleetCommandJobSubmitResult>(`/command/datas/jobs/${jobId}/retry`, {}, {
    ...(params ? { params } : {}),
    silentError: true
  } as any)
}

export const listFleetSavedFilters = async () => {
  return await request.get<FleetSavedFilterListResult>(`/command/datas/saved-filters`, {
    silentError: true
  } as any)
}

export const createFleetSavedFilter = async (params: FleetSavedFilterPayload) => {
  return await request.post<FleetSavedFilterItem>(`/command/datas/saved-filters`, params, {
    silentError: true
  } as any)
}

export const updateFleetSavedFilter = async (id: string, params: FleetSavedFilterPayload) => {
  return await request.put<FleetSavedFilterItem>(`/command/datas/saved-filters/${id}`, params, {
    silentError: true
  } as any)
}

export const deleteFleetSavedFilter = async (id: string) => {
  return await request.delete(`/command/datas/saved-filters/${id}`, {
    silentError: true
  } as any)
}
