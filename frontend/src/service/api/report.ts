import { request } from '../request'

export type ReportGenerationStatus = 'pending' | 'processing' | 'retrying' | 'succeeded' | 'failed'
export type ReportDeliveryStatus = 'pending' | 'processing' | 'retrying' | 'accepted' | 'failed' | 'ambiguous'
export type ReportProjectedStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'ambiguous'
export type ReportRunStatus = ReportGenerationStatus | ReportDeliveryStatus | ReportProjectedStatus

export interface ReportSchedule {
  id: string
  tenant_id?: string
  name: string
  cron_expr: string
  timezone: string
  recipients: string
  device_ids: string[]
  keys: string[]
  lookback_hours: number
  format: 'csv' | string
  enabled: boolean
  next_run_at?: string | null
  revision: number
  last_run_id?: string | null
  last_run_at?: string | null
  last_status?: ReportRunStatus | null
  schedule_error_code?: 'invalid_schedule' | null
  created_at: string
  updated_at: string
}

export interface CreateReportSchedulePayload {
  name: string
  cron_expr: string
  timezone: string
  recipients: string
  device_ids: string[]
  keys: string[]
  lookback_hours: number
  format: 'csv'
  enabled?: boolean
}

export interface UpdateReportSchedulePayload {
  revision: number
  name?: string
  cron_expr?: string
  timezone?: string
  recipients?: string
  device_ids?: string[]
  keys?: string[]
  lookback_hours?: number
  format?: 'csv'
  enabled?: boolean
}

/** @deprecated Use the operation-specific create/update payload types. */
export type ReportSchedulePayload = Omit<CreateReportSchedulePayload, 'enabled'> & {
  enabled: boolean
  revision?: number
}

export interface ReportScheduleListParams {
  page?: number
  page_size?: number
  search?: string
  enabled?: boolean
}

export interface ReportScheduleListResult {
  list: ReportSchedule[]
  total: number
  page?: number
  page_size?: number
}

export interface ReportRun {
  run_id: string
  schedule_id: string
  trigger: 'scheduled' | 'manual' | 'retry' | string
  parent_run_id?: string | null
  overall_status: ReportProjectedStatus
  generation_status: ReportGenerationStatus
  delivery_status: ReportDeliveryStatus
  window_start_at: string
  window_end_at: string
  generation_attempts: number
  delivery_attempts: number
  generation_error_code?: string
  delivery_error_code?: string
  duplicate_delivery_risk: boolean
  created_at: string
  generation_completed_at?: string | null
  delivery_completed_at?: string | null
}

export interface ReportRunListResult {
  list: ReportRun[]
  total: number
  page?: number
  page_size?: number
}

export interface ReportRunActionResult {
  run_id: string
  schedule_id: string
  status: ReportProjectedStatus
  status_url: string
  idempotent_replay: boolean
  duplicate_delivery_risk: boolean
}

export interface ReportRunListParams {
  page?: number
  page_size?: number
}

export interface ReportActionRequestOptions {
  idempotencyKey: string
  silentError?: boolean
}

const reportPath = (value: string) => encodeURIComponent(value)

const actionConfig = ({ idempotencyKey, silentError = true }: ReportActionRequestOptions) => ({
  headers: { 'Idempotency-Key': idempotencyKey },
  silentError
})

export const listReportSchedules = (params: ReportScheduleListParams = {}) =>
  request.get<ReportScheduleListResult>('/report/schedules', { params, silentError: true })

export const getReportSchedule = (id: string) =>
  request.get<ReportSchedule>(`/report/schedules/${reportPath(id)}`, { silentError: true })

export const createReportSchedule = (payload: CreateReportSchedulePayload) =>
  request.post<ReportSchedule>('/report/schedules', payload, { silentError: true })

export const updateReportSchedule = (id: string, payload: UpdateReportSchedulePayload) =>
  request.put<ReportSchedule>(`/report/schedules/${reportPath(id)}`, payload, { silentError: true })

export const deleteReportSchedule = (id: string, revision: number) =>
  request.delete<{ id: string }>(`/report/schedules/${reportPath(id)}`, {
    params: { revision },
    silentError: true
  })

export const runReportSchedule = (id: string, options: ReportActionRequestOptions) =>
  request.post<ReportRunActionResult>(`/report/schedules/${reportPath(id)}/run`, {}, actionConfig(options))

export const listReportRuns = (scheduleId: string, params: ReportRunListParams = {}) =>
  request.get<ReportRunListResult>(`/report/schedules/${reportPath(scheduleId)}/runs`, {
    params,
    silentError: true
  })

export const getReportRun = (scheduleId: string, runId: string) =>
  request.get<ReportRun>(`/report/schedules/${reportPath(scheduleId)}/runs/${reportPath(runId)}`, {
    silentError: true
  })

export const retryReportRun = (scheduleId: string, runId: string, options: ReportActionRequestOptions) =>
  request.post<ReportRunActionResult>(
    `/report/schedules/${reportPath(scheduleId)}/runs/${reportPath(runId)}/retry`,
    {},
    actionConfig(options)
  )
