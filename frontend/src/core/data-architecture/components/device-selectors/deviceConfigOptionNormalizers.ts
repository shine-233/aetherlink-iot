import type { DeviceInfo, DeviceMetric } from '@/core/data-architecture/types/device-parameter-group'

type ApiRecord = Record<string, unknown>
type MetricType = DeviceMetric['metricType']

export const extractArray = (value: unknown): unknown[] => {
  if (Array.isArray(value)) return value
  if (value && typeof value === 'object') {
    const record = value as ApiRecord
    return extractArray(record.data ?? record.list ?? record.records ?? record.items)
  }
  return []
}

export const normalizeDevice = (device: unknown): DeviceInfo | null => {
  if (!device || typeof device !== 'object') return null

  const record = device as ApiRecord
  const deviceId = String(record.deviceId ?? record.device_id ?? record.id ?? record.value ?? '')
  if (!deviceId) return null

  const deviceName = String(record.deviceName ?? record.device_name ?? record.name ?? record.label ?? deviceId)
  const deviceType = String(record.deviceType ?? record.device_type ?? record.type ?? record.product_name ?? '')
  const deviceModel = record.deviceModel ?? record.device_model ?? record.model

  return {
    deviceId,
    deviceName,
    deviceType,
    ...(deviceModel ? { deviceModel: String(deviceModel) } : {})
  }
}

const normalizeMetricType = (value: unknown): MetricType => {
  return value === 'number' || value === 'boolean' || value === 'json' || value === 'string' ? value : 'string'
}

export const normalizeMetric = (metric: unknown): DeviceMetric | null => {
  if (!metric || typeof metric !== 'object') return null

  const record = metric as ApiRecord
  const metricKey = String(record.metricKey ?? record.key ?? record.data_identifier ?? record.id ?? record.value ?? '')
  if (!metricKey) return null

  const unit = record.unit ?? record.symbol
  const description = record.description ?? record.desc

  return {
    metricKey,
    metricLabel: String(record.metricLabel ?? record.label ?? record.name ?? metricKey),
    metricType: normalizeMetricType(record.metricType ?? record.data_type ?? record.type),
    ...(unit ? { unit: String(unit) } : {}),
    ...(description ? { description: String(description) } : {})
  }
}

export const normalizeMetrics = (response: unknown): DeviceMetric[] => {
  const groupsOrMetrics = extractArray(response)
  const metrics: DeviceMetric[] = []

  for (const item of groupsOrMetrics) {
    if (item && typeof item === 'object' && Array.isArray((item as ApiRecord).options)) {
      metrics.push(
        ...(item as ApiRecord).options!.map(normalizeMetric).filter((metric): metric is DeviceMetric => Boolean(metric))
      )
      continue
    }

    const metric = normalizeMetric(item)
    if (metric) metrics.push(metric)
  }

  return metrics
}
