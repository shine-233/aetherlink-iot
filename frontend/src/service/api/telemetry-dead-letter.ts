import { request } from '../request'

export type TelemetryDeadLetterStatus = 'pending' | 'retrying' | 'processing' | 'resolved' | 'dead'

export type TelemetryDeadLetterAction = 'replay' | 'retry' | 'resolve' | 'ignore'

export interface TelemetryDeadLetterListParams {
  page?: number
  page_size?: number
  tenant_id?: string
  device_id?: string
  key?: string
  status?: TelemetryDeadLetterStatus | ''
}

export interface TelemetryDeadLetterRow {
  id: string
  device_id: string
  tenant_id: string
  key: string
  ts: number
  bool_v?: boolean
  number_v?: number
  string_v?: string
  status: TelemetryDeadLetterStatus | string
  attempts: number
  last_error?: string
  next_retry_at?: string | null
  created_at: string
  updated_at: string
}

export interface TelemetryDeadLetterListResult {
  list: TelemetryDeadLetterRow[]
  total: number
}

export interface DrainTelemetryDeadLetterParams {
  tenant_id?: string
  device_id?: string
  key?: string
  limit?: number
}

export interface DrainTelemetryDeadLetterItem {
  id: string
  status: string
  error?: string
}

export interface DrainTelemetryDeadLetterResult {
  total_ready: number
  attempted: number
  replayed: number
  failed: number
  items: DrainTelemetryDeadLetterItem[]
}

export const getTelemetryDeadLetters = async (params: TelemetryDeadLetterListParams = {}) => {
  return await request.get<TelemetryDeadLetterListResult>('/telemetry/datas/dead-letters', { params })
}

export const updateTelemetryDeadLetterStatus = async (id: string, action: TelemetryDeadLetterAction) => {
  return await request<null>({
    url: `/telemetry/datas/dead-letters/${id}/status`,
    method: 'patch',
    data: { action }
  })
}

export const drainTelemetryDeadLetters = async (params: DrainTelemetryDeadLetterParams = {}) => {
  return await request.post<DrainTelemetryDeadLetterResult>('/telemetry/datas/dead-letters/drain', params)
}
