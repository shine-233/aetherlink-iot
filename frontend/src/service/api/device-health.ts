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
  /**
   * TP-21 MSET 多元状态估计特征维度。
   * 仅当后端配置开关（health.mset.enabled，默认关）开启时返回；缺省 = 维度未参与评估。
   */
  mset?: DeviceHealthMSETFeature
}

/**
 * TP-21 MSET 特征维度投影（与后端 model.DeviceHealthMSETFeature 一一对应）。
 * applied=true 为有效推理；degraded=true 为 fail-closed 降级（偏差分恒为 0 中性，原因见 degrade_reason）。
 */
export interface DeviceHealthMSETFeature {
  applied: boolean
  degraded: boolean
  /** cold_start | insufficient_samples | singular_matrix | invalid_sample | invalid_feature_config | no_feature_keys | history_fetch_failed */
  degrade_reason?: string
  /** 0~100 偏差评分（0=贴合历史基线） */
  deviation_score: number
  /** 折入 anomaly_penalty 的扣减（偏差分×权重） */
  penalty: number
  /** 马氏距离 d（非平方），仅有效推理时出现 */
  mahalanobis?: number
  /** 参与训练/推理的特征遥测键（按列序） */
  feature_keys?: string[]
  /** 训练历史完整样本行数 */
  train_samples?: number
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
  return await request.post<{ success: boolean; score: number }>(
    `/devices/${encodeURIComponent(deviceId)}/health/evaluate`
  )
}
