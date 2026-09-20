import { request } from '../request'

/**
 * P2.2 基础异常检测（后端 POST /api/v1/telemetry/analysis/anomaly）。
 *
 * 规则类型的字面量与后端 model.TelemetryAnomalyRule* 常量一一对应；
 * 后端入参校验是 oneof=bounds deviation，这里用联合类型把同一约束前移到编译期，
 * 避免拼错字符串到运行时才被 100002 拒绝。
 */
export const TELEMETRY_ANOMALY_RULE_BOUNDS = 'bounds'
export const TELEMETRY_ANOMALY_RULE_DEVIATION = 'deviation'

export type TelemetryAnomalyRuleType = typeof TELEMETRY_ANOMALY_RULE_BOUNDS | typeof TELEMETRY_ANOMALY_RULE_DEVIATION

export type TelemetryAnomalyAggregate = 'avg' | 'sum' | 'min' | 'max' | 'count' | 'last'

export interface TelemetryAnomalyRule {
  /** bounds=静态上下限；deviation=均值±K 倍标准差。 */
  type: TelemetryAnomalyRuleType
  min?: number
  max?: number
  k?: number
}

export interface TelemetryAnomalyQuery {
  /** 后端限制 1–50 个设备；超限由后端拒绝，前端只做交互层约束。 */
  device_ids: string[]
  key: string
  /** 毫秒时间戳。 */
  start_time: number
  end_time: number
  /** 序列分桶宽度（毫秒）。异常判定在分桶聚合序列上进行。 */
  window_ms: number
  aggregate?: TelemetryAnomalyAggregate
  rule: TelemetryAnomalyRule
}

export interface TelemetryAnomalyHit {
  /** 序列中的位置（第几个分桶）。 */
  index: number
  value: number
  reason: string
}

export interface TelemetryAnomalyDeviceResult {
  device_id: string
  /** 单设备失败的原因；有值时该设备的 anomalies 不代表"无异常"。 */
  error?: string
  /** 参与判定的分桶数。0 表示窗口内没有数据，不等于"无异常"。 */
  total: number
  anomalies: TelemetryAnomalyHit[]
  /** 异常分桶占比。 */
  rate: number
}

export interface TelemetryAnomalyResult {
  key: string
  aggregate: string
  window_ms: number
  rule: TelemetryAnomalyRule
  devices: TelemetryAnomalyDeviceResult[]
}

export function detectTelemetryAnomalies(payload: TelemetryAnomalyQuery) {
  return request.post<TelemetryAnomalyResult>('/telemetry/analysis/anomaly', payload)
}
