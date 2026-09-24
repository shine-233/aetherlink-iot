/**
 * 文件用途：Device Health Score（对标 ThingsBoard PE / ThingsPanel 算法中心）前端 API 客户端。
 * 核心逻辑：获取租户设备健康度汇总大盘、单设备健康度诊断详情与触发评估。
 */
import { request } from '../request'

export type HealthStatus = 'HEALTHY' | 'SUB_HEALTHY' | 'WARNING' | 'CRITICAL'

export interface DeviceHealthScoreItem {
  device_id: string
  device_name: string
  device_number?: string
  score: number
  health_status: HealthStatus
  alarm_penalty: number
  offline_penalty: number
  anomaly_penalty: number
  is_online: boolean
  evaluated_at: string
}

export interface DeviceHealthSummaryResponse {
  total_devices: number
  healthy_count: number
  sub_healthy_count: number
  warning_count: number
  critical_count: number
  average_score: number
  unhealthy_devices: DeviceHealthScoreItem[]
  evaluated_at: string
}

export interface DeviceHealthDetailResponse {
  device_id: string
  device_name: string
  device_number: string
  score: number
  health_status: HealthStatus
  alarm_penalty: number
  offline_penalty: number
  anomaly_penalty: number
  is_online: boolean
  offline_duration_seconds: number
  active_alarm_count: number
  active_alarms: Array<Record<string, any>>
  suggestions: string[]
  evaluated_at: string
}

/** 获取租户级设备健康度汇总大盘 */
export const getDeviceHealthSummary = async () => {
  return await request.get<DeviceHealthSummaryResponse>('/devices/health/summary')
}

/** 触发租户级批量设备健康度重新评估 */
export const evaluateTenantDeviceHealth = async () => {
  return await request.post<{ success: boolean; evaluated_count: number }>('/devices/health/evaluate')
}

/** 获取单设备健康度深度诊断剖析 */
export const getDeviceHealthDetail = async (deviceId: string) => {
  return await request.get<DeviceHealthDetailResponse>(`/devices/${encodeURIComponent(deviceId)}/health`)
}

/** 触发单设备健康度即时重新评估 */
export const evaluateDeviceHealth = async (deviceId: string) => {
  return await request.post<{ success: boolean; score: number }>(`/devices/${encodeURIComponent(deviceId)}/health/evaluate`)
}
