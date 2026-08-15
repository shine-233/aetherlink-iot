/**
 * Device telemetry, simulation, and Twin Lite API wrappers.
 *
 * Keep these exports stable through `device.ts` re-exports; many pages still
 * import from the historical device API facade.
 */
import { request } from '../request'

/** 获取设备地图遥测 */
export const deviceMapTelemetry = async (id: string) => {
  return await request.get<any>(`/device/map/telemetry/${id}`)
}

/** 设备遥测当前值查询 * */
export const telemetryDataCurrent = async (id: string, requestConfig: Record<string, unknown> = {}) => {
  const url = `/telemetry/datas/current/${id}`
  return await request.get<DeviceManagement.telemetryCurrent | any>(url, requestConfig as any)
}

export type DeviceTwinSource = 'telemetry' | 'attribute' | 'command'

export type DeviceTwinRow = {
  key: string
  label: string
  source: DeviceTwinSource
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

export type DeviceTwinSummary = {
  desiredCount: number
  reportedCount: number
  matchedCount: number
  deltaCount: number
  unavailableCount: number
  staleDesiredCount?: number
  convergenceStatus?: string
  nextAction?: string
  evidenceBoundary?: string
}

export type DeviceTwinState = {
  rows: DeviceTwinRow[]
  summary: DeviceTwinSummary
}

/** 设备 Twin Lite 聚合 */
export const getDeviceTwin = async (id: string, requestConfig: Record<string, unknown> = {}) => {
  return await request.get<DeviceTwinState>(`/device/twin/${id}`, requestConfig as any)
}

export type DeviceTwinDesiredPayload = {
  source: 'telemetry' | 'attribute'
  key: string
  desired: unknown
  expiry?: string
}

/** 写入/更新设备 Twin Lite desired 条目 */
export const setDeviceTwinDesired = async (id: string, params: DeviceTwinDesiredPayload) => {
  return await request.put<any>(`/device/twin/${id}/desired`, params)
}

/**
 * @param params {device_id:string,keys:string}
 * @returns
 */
export const telemetryDataCurrentKeys = async (params: any) => {
  return await request.get<any>('/telemetry/datas/current/keys', { params })
}

const TELEMETRY_TIME_RANGE_MS: Record<string, number> = {
  last_5m: 5 * 60 * 1000,
  last_15m: 15 * 60 * 1000,
  last_30m: 30 * 60 * 1000,
  last_1h: 60 * 60 * 1000,
  last_3h: 3 * 60 * 60 * 1000,
  last_6h: 6 * 60 * 60 * 1000,
  last_12h: 12 * 60 * 60 * 1000,
  last_24h: 24 * 60 * 60 * 1000,
  last_3d: 3 * 24 * 60 * 60 * 1000,
  last_7d: 7 * 24 * 60 * 60 * 1000,
  last_15d: 15 * 24 * 60 * 60 * 1000,
  last_30d: 30 * 24 * 60 * 60 * 1000,
  last_60d: 60 * 24 * 60 * 60 * 1000,
  last_90d: 90 * 24 * 60 * 60 * 1000,
  last_6m: 180 * 24 * 60 * 60 * 1000,
  last_1y: 365 * 24 * 60 * 60 * 1000
}

type TelemetryHistoryRange = {
  start_time: number
  end_time: number
}

type TelemetryHistoryPoint = {
  key?: string
  x: number
  y: number
}

function toFiniteNumber(value: unknown, fallback: number | null = null) {
  if (value === null || value === undefined) {
    return fallback
  }

  const numericValue = Number(value)
  return Number.isFinite(numericValue) ? numericValue : null
}

function shouldFallbackTelemetryHistory(error: any) {
  const message = error?.error?.message || error?.message || ''
  return typeof message === 'string' && message.includes('failed to encode args[2]') && message.includes('OID 25')
}

function resolveTelemetryHistoryRange(params: any): TelemetryHistoryRange | null {
  const startTime = toFiniteNumber(params?.start_time)
  const endTime = toFiniteNumber(params?.end_time)

  if (params?.time_range === 'custom' && startTime !== null && endTime !== null) {
    return { start_time: startTime, end_time: endTime }
  }

  const duration = TELEMETRY_TIME_RANGE_MS[String(params?.time_range || '')]
  if (!duration) return null

  const currentTime = Date.now()
  return {
    start_time: currentTime - duration,
    end_time: currentTime
  }
}

function normalizeTelemetryHistoryPageData(payload: any): TelemetryHistoryPoint[] {
  const list = Array.isArray(payload?.list) ? payload.list : []

  return list
    .map((item: any) => {
      const x = toFiniteNumber(item?.ts ?? item?.time ?? item?.x, 0)
      const y = toFiniteNumber(item?.value ?? item?.y ?? item?.avg, 0)

      return {
        key: item?.key,
        x,
        y
      }
    })
    .filter((item: any): item is TelemetryHistoryPoint => item.x !== null && item.y !== null)
}

export const telemetryHistoryData = async (params: any, requestConfig: Record<string, unknown> = {}) => {
  return await request.get<any>(`/telemetry/datas/history/page`, {
    ...(requestConfig as any),
    params
  })
}

/**
 * @param params { device_id: string, key: string, start_time: string, end_time: string, aggregate_window: string,
 *   aggregate_function: string, time_range: string }
 * @returns
 */
export const telemetryDataHistoryList = async (params: any, requestConfig: Record<string, unknown> = {}) => {
  const normalized = { ...params }
  let statisticResponse: any
  try {
    statisticResponse = await request.get<any>('/telemetry/datas/statistic', {
      ...(requestConfig as any),
      params: normalized
    })
  } catch (error) {
    statisticResponse = error
  }

  if (!statisticResponse?.error || !shouldFallbackTelemetryHistory(statisticResponse)) {
    return statisticResponse
  }

  // Some telemetry deployments reject the aggregate-window argument type. Keep
  // the public API stable by falling back to the raw history endpoint.
  const fallbackRange = resolveTelemetryHistoryRange(normalized)
  if (!fallbackRange) {
    return statisticResponse
  }

  let historyResponse: any
  try {
    historyResponse = await telemetryHistoryData(
      {
        device_id: normalized.device_id,
        key: normalized.key,
        ...fallbackRange
      },
      requestConfig
    )
  } catch (error) {
    historyResponse = error
  }

  if (historyResponse?.error) {
    return statisticResponse
  }

  return {
    data: normalizeTelemetryHistoryPageData(historyResponse?.data),
    error: null
  }
}

/** 遥测删除数据处理 */
export const telemetryDataDel = async (params: any) => {
  return await request.delete2<Api.BaseApi.Data | any>(`/telemetry/datas`, params)
}

export const getTelemetryLogList = async (params: any) => {
  return await request.get<any>(`/telemetry/datas/set/logs`, { params })
}

export const telemetryDataPub = async (params: any) => {
  return await request.post<any>(`/telemetry/datas/pub`, params)
}

/** 获取设备获取遥测数据命令 */
export const getSimulation = async (params: any) => {
  return await request.get<any>(`/telemetry/datas/simulation`, { params })
}

/** 获取设备发送遥测数据命令 */
export const sendSimulation = async (params: any) => {
  return await request.post<any>(`/telemetry/datas/simulation`, params)
}

/** 获取模拟表单初始值 */
export const getSimulationInit = async (params: { device_id: string }) => {
  return await request.get<any>(`/telemetry/datas/simulation/init`, { params })
}

/** 发送模拟数据 */
export const sendSimulationData = async (params: {
  device_id: string
  data: string
  server?: string
  port?: number
  topic?: string
}) => {
  return await request.post<any>(`/telemetry/datas/simulation/send`, params)
}
