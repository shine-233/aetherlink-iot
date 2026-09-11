import type { ReportRun, ReportRunStatus } from '@/service/api/report'

const ACTIVE = new Set(['queued', 'running', 'pending', 'processing', 'retrying'])
const SUCCESS = new Set(['succeeded', 'accepted'])

export type ReportTagType = 'default' | 'success' | 'warning' | 'error' | 'info'

export const normalizeReportStatus = (status?: ReportRunStatus | null) => String(status || 'unknown').toLowerCase()

export const reportStatusTagType = (status?: ReportRunStatus | null): ReportTagType => {
  const normalized = normalizeReportStatus(status)
  if (SUCCESS.has(normalized)) return 'success'
  if (ACTIVE.has(normalized)) return 'info'
  if (normalized === 'ambiguous') return 'warning'
  if (normalized === 'failed') return 'error'
  return 'default'
}

export const canRetryReportRun = (run?: ReportRun | null, scheduleEnabled = true) =>
  Boolean(scheduleEnabled && run && ['failed', 'ambiguous'].includes(normalizeReportStatus(run.overall_status)))

type ReportRequestError = {
  code?: string
  request?: unknown
  response?: unknown
  isAxiosError?: boolean
}

const UNCERTAIN_TRANSPORT_CODES = new Set(['ERR_NETWORK', 'ECONNABORTED', 'ETIMEDOUT'])

export const isUncertainReportTransportError = (error: unknown) => {
  if (!error || typeof error !== 'object') return false
  const requestError = error as ReportRequestError
  if (requestError.response != null) return false
  return Boolean(
    requestError.request != null ||
      requestError.isAxiosError ||
      (requestError.code && UNCERTAIN_TRANSPORT_CODES.has(requestError.code))
  )
}

export const createReportIdempotencyKey = () => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return `report-${Date.now()}-${Math.random().toString(36).slice(2)}`
}
