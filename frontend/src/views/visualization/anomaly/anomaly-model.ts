/**
 * 文件用途：P2.2 基础异常检测页面的纯逻辑（表单校验、查询构造、结果整形）。
 *
 * 关键注意事项：
 * 1. 校验规则必须与后端入参校验一致（device_ids 1–50、key 必填、window_ms≥1、
 *    rule.type ∈ {bounds,deviation}、bounds 需 min/max 至少其一且 min≤max、
 *    deviation 的 k 必须为正）。前端先拦是为了给出可读提示，
 *    不是为了替代后端——后端仍会独立校验。
 * 2. "无数据"与"无异常"是两种事实：total=0 时必须单独表达，
 *    否则用户会把"窗口内没有数据"读成"设备一切正常"。
 * 3. 单设备 error 必须独立成态：有 error 时该设备的 anomalies 不代表"无异常"。
 * 4. 本文件不依赖 Vue，便于用纯函数表驱动测试。
 */
import {
  TELEMETRY_ANOMALY_RULE_BOUNDS,
  TELEMETRY_ANOMALY_RULE_DEVIATION,
  type TelemetryAnomalyAggregate,
  type TelemetryAnomalyDeviceResult,
  type TelemetryAnomalyHit,
  type TelemetryAnomalyQuery,
  type TelemetryAnomalyResult,
  type TelemetryAnomalyRuleType
} from '@/service/api'

/** 后端 device_ids 上限（validate:max=50）。 */
export const ANOMALY_MAX_DEVICES = 50

/** deviation 规则的默认 K（与后端默认值一致）。 */
export const ANOMALY_DEFAULT_K = 3

export interface AnomalyFormState {
  /** 每行一个设备 ID（沿用报表页的输入约定）。 */
  deviceIdsText: string
  key: string
  /** 分桶宽度，以分钟表达（后端要毫秒）。 */
  windowMinutes: number
  aggregate: TelemetryAnomalyAggregate
  ruleType: TelemetryAnomalyRuleType
  boundsMin: number | null
  boundsMax: number | null
  deviationK: number | null
}

export const DEFAULT_ANOMALY_FORM: AnomalyFormState = {
  deviceIdsText: '',
  key: '',
  windowMinutes: 5,
  aggregate: 'avg',
  ruleType: TELEMETRY_ANOMALY_RULE_BOUNDS,
  boundsMin: null,
  boundsMax: null,
  deviationK: ANOMALY_DEFAULT_K
}

/**
 * 解析设备 ID 文本：支持换行或逗号分隔；去空白、丢空项、去重（保序）。
 * 去重是必要的——重复 ID 会让同一次检测把同一设备算两遍，
 * 结果里出现两条同名记录，用户会以为是两台设备。
 */
export function parseDeviceIds(text: string): string[] {
  const seen = new Set<string>()
  const ids: string[] = []
  for (const raw of text.split(/[\n,]/)) {
    const id = raw.trim()
    if (!id || seen.has(id)) continue
    seen.add(id)
    ids.push(id)
  }
  return ids
}

export type BuildQueryResult = { ok: true; query: TelemetryAnomalyQuery } | { ok: false; errorKey: string }

export interface BuildQueryInput {
  form: AnomalyFormState
  /** 时间窗，毫秒时间戳；未选择时为 null。 */
  range: [number, number] | null
}

/**
 * 校验并构造请求体。失败时返回 i18n 键而非成品文案，
 * 让页面统一走 $t()（文案不散落在逻辑层）。
 */
export function buildAnomalyQuery(input: BuildQueryInput): BuildQueryResult {
  const { form, range } = input

  const deviceIds = parseDeviceIds(form.deviceIdsText)
  if (deviceIds.length === 0) return { ok: false, errorKey: 'page.anomaly.errorDevicesRequired' }
  if (deviceIds.length > ANOMALY_MAX_DEVICES) return { ok: false, errorKey: 'page.anomaly.errorDevicesTooMany' }

  const key = form.key.trim()
  if (!key) return { ok: false, errorKey: 'page.anomaly.errorKeyRequired' }

  if (!range) return { ok: false, errorKey: 'page.anomaly.errorRangeRequired' }
  const [startTime, endTime] = range
  if (!(startTime < endTime)) return { ok: false, errorKey: 'page.anomaly.errorRangeOrder' }

  if (!Number.isFinite(form.windowMinutes) || form.windowMinutes < 1) {
    return { ok: false, errorKey: 'page.anomaly.errorWindow' }
  }

  const rule = buildRule(form)
  if ('errorKey' in rule) return { ok: false, errorKey: rule.errorKey }

  return {
    ok: true,
    query: {
      device_ids: deviceIds,
      key,
      start_time: startTime,
      end_time: endTime,
      window_ms: Math.round(form.windowMinutes * 60_000),
      aggregate: form.aggregate,
      rule: rule.rule
    }
  }
}

function buildRule(form: AnomalyFormState): { rule: TelemetryAnomalyQuery['rule'] } | { errorKey: string } {
  if (form.ruleType === TELEMETRY_ANOMALY_RULE_BOUNDS) {
    const hasMin = form.boundsMin !== null && Number.isFinite(form.boundsMin)
    const hasMax = form.boundsMax !== null && Number.isFinite(form.boundsMax)
    if (!hasMin && !hasMax) return { errorKey: 'page.anomaly.errorBoundsRequired' }
    if (hasMin && hasMax && (form.boundsMin as number) > (form.boundsMax as number)) {
      return { errorKey: 'page.anomaly.errorBoundsOrder' }
    }
    return {
      rule: {
        type: TELEMETRY_ANOMALY_RULE_BOUNDS,
        min: hasMin ? (form.boundsMin as number) : undefined,
        max: hasMax ? (form.boundsMax as number) : undefined
      }
    }
  }

  // deviation：k 留空时用默认值；填了就必须为正（0 或负数无法定义容差带）。
  if (form.deviationK === null) {
    return { rule: { type: TELEMETRY_ANOMALY_RULE_DEVIATION, k: ANOMALY_DEFAULT_K } }
  }
  if (!Number.isFinite(form.deviationK) || form.deviationK <= 0) {
    return { errorKey: 'page.anomaly.errorDeviationK' }
  }
  return { rule: { type: TELEMETRY_ANOMALY_RULE_DEVIATION, k: form.deviationK } }
}

/** 单设备结果的呈现态。 */
export type AnomalyRowStatus = 'error' | 'no-data' | 'anomaly' | 'clean'

export interface AnomalyRow {
  deviceId: string
  status: AnomalyRowStatus
  /** 参与判定的分桶数。 */
  total: number
  /** 异常分桶占比（0–1）。 */
  rate: number
  hits: TelemetryAnomalyHit[]
  error?: string
}

/**
 * 结果整形。顺序即语义优先级：
 * error（本次检测没跑成） > no-data（跑了但没数据） > anomaly > clean。
 * 把 no-data 与 clean 合并，或把 error 当 clean，都会制造"假安全"。
 */
export function toAnomalyRows(result: TelemetryAnomalyResult | null): AnomalyRow[] {
  if (!result || !Array.isArray(result.devices)) return []
  return result.devices.map((device) => toAnomalyRow(device))
}

function toAnomalyRow(device: TelemetryAnomalyDeviceResult): AnomalyRow {
  const hits = Array.isArray(device.anomalies) ? device.anomalies : []
  const total = Number.isFinite(device.total) ? device.total : 0
  const base = {
    deviceId: device.device_id,
    total,
    rate: Number.isFinite(device.rate) ? device.rate : 0,
    hits
  }
  if (device.error) return { ...base, status: 'error', error: device.error }
  if (total === 0) return { ...base, status: 'no-data' }
  if (hits.length > 0) return { ...base, status: 'anomaly' }
  return { ...base, status: 'clean' }
}

/** 汇总计数，供页面顶部概览。 */
export function summarizeAnomalyRows(rows: AnomalyRow[]) {
  return {
    devices: rows.length,
    anomaly: rows.filter((row) => row.status === 'anomaly').length,
    clean: rows.filter((row) => row.status === 'clean').length,
    noData: rows.filter((row) => row.status === 'no-data').length,
    failed: rows.filter((row) => row.status === 'error').length,
    totalHits: rows.reduce((sum, row) => sum + row.hits.length, 0)
  }
}
