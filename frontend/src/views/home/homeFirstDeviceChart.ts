/**
 * 首设备工作台 · 遥测与首图层。
 *
 * 只依赖原始遥测和浏览器在线测试状态，不知道设备、连接参数或引导步骤；
 * onboarding / proof 两层都建立在这里之上（依赖方向：chart ← onboarding ← proof）。
 */

export type FirstTelemetryPoint = {
  key: string
  value: string
  ts?: string
}

export type FirstDeviceBrowserTestStatus = 'idle' | 'sending' | 'sent' | 'confirmed' | 'failed'

export type FirstDeviceBrowserTestState = {
  status: FirstDeviceBrowserTestStatus
  message: string
  sentAt?: string
  telemetryKey?: string
  telemetryValue?: string
}

export type FirstDeviceOnlineTesterState = {
  type: 'success' | 'warning' | 'info' | 'error'
  statusLabel: string
  title: string
  description: string
  actionLabel: string
  disabledReason: string
  lastSignal: string
  echoRows: Array<{ label: string; value: string }>
}

export type FirstDeviceChartSource = 'latest_telemetry' | 'browser_test' | 'none'

export type FirstDeviceChartPoint = FirstTelemetryPoint & {
  barPercent: number
}

export type FirstDeviceChartState = {
  ready: boolean
  title: string
  summary: string
  primaryKey: string
  primaryValue: string
  generatedFrom: FirstDeviceChartSource
  points: FirstDeviceChartPoint[]
}

/** 首页只展示前 N 条最新遥测，图表和引导共用这个上限。 */
export const FIRST_DEVICE_TELEMETRY_LIMIT = 5

/** 柱状条最小宽度百分比，避免小数值完全不可见。 */
const MIN_BAR_PERCENT = 8

/** 兼容 axios 响应 / 业务包裹 / 已解包数据三种形态。 */
export const unwrapFirstDeviceResponse = (response: any) => response?.data?.data ?? response?.data ?? response ?? {}

const BROWSER_TEST_MESSAGES: Record<FirstDeviceBrowserTestStatus, string> = {
  idle: '浏览器在线测试还没有运行。',
  sending: '正在发送浏览器测试遥测。',
  sent: '浏览器在线测试已发送，正在等待最新遥测。',
  confirmed: '浏览器在线测试已被最新遥测确认。',
  failed: '浏览器在线测试失败。'
}

export const createIdleFirstDeviceBrowserTestState = (): FirstDeviceBrowserTestState => ({
  status: 'idle',
  message: BROWSER_TEST_MESSAGES.idle
})

export const buildFirstDeviceBrowserTestState = (options: {
  status: FirstDeviceBrowserTestStatus
  message?: string
  telemetry?: FirstTelemetryPoint | null
  sentAt?: string
}): FirstDeviceBrowserTestState => ({
  status: options.status,
  message: options.message || BROWSER_TEST_MESSAGES[options.status] || BROWSER_TEST_MESSAGES.idle,
  sentAt: options.sentAt,
  telemetryKey: options.telemetry?.key,
  telemetryValue: options.telemetry?.value
})

const stringifyTelemetryValue = (rawValue: unknown) =>
  typeof rawValue === 'object' ? JSON.stringify(rawValue) : String(rawValue)

const toTelemetryPoint = (item: any): FirstTelemetryPoint | null => {
  const key = String(item.key ?? item.identify ?? item.name ?? '')
  if (!key) return null
  return {
    key,
    value: stringifyTelemetryValue(item.value ?? item.val ?? item.data ?? item.y ?? ''),
    ts: item.ts ?? item.time ?? item.created_at ?? item.updated_at
  }
}

/** 接受数组行（list/data）或 `{ key: value }` 字典两种遥测返回形态。 */
export const normalizeTelemetryPoints = (response: any): FirstTelemetryPoint[] => {
  const data = unwrapFirstDeviceResponse(response)
  const source = data?.list ?? data?.data ?? data
  const rows: any[] = Array.isArray(source)
    ? source
    : Object.entries(source || {}).map(([key, value]) => ({ key, value }))

  return rows
    .map(toTelemetryPoint)
    .filter((point): point is FirstTelemetryPoint => point !== null)
    .slice(0, FIRST_DEVICE_TELEMETRY_LIMIT)
}

const toFiniteMagnitude = (value: string) => {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? Math.abs(parsed) : null
}

/** 数值点按最大绝对值归一化；非数值或全零时整条显示（100%）。 */
const toChartPoints = (telemetry: FirstTelemetryPoint[]): FirstDeviceChartPoint[] => {
  const sourcePoints = telemetry.slice(0, FIRST_DEVICE_TELEMETRY_LIMIT)
  const magnitudes = sourcePoints.map((point) => toFiniteMagnitude(point.value))
  const maxValue = Math.max(...magnitudes.filter((value): value is number => value !== null), 0)

  return sourcePoints.map((point, index) => {
    const magnitude = magnitudes[index]
    const barPercent =
      magnitude !== null && maxValue > 0 ? Math.max(MIN_BAR_PERCENT, Math.round((magnitude / maxValue) * 100)) : 100
    return { ...point, barPercent }
  })
}

/**
 * 主指标优先级：
 * 1. 浏览器测试已确认，且最新遥测里有完全相同的 key/value → 用该图表点
 * 2. 浏览器测试已确认但遥测还没刷出来 → 用测试值占位（满条）
 * 3. 否则取第一条最新遥测
 */
const pickPrimaryPoint = (
  points: FirstDeviceChartPoint[],
  browserTest?: FirstDeviceBrowserTestState
): FirstDeviceChartPoint | null => {
  if (browserTest?.status === 'confirmed' && browserTest.telemetryKey) {
    const key = browserTest.telemetryKey
    const value = browserTest.telemetryValue || '--'
    return points.find((point) => point.key === key && point.value === value) || { key, value, barPercent: 100 }
  }
  return points[0] || null
}

export const buildFirstDeviceChartState = (
  telemetry: FirstTelemetryPoint[],
  browserTest?: FirstDeviceBrowserTestState
): FirstDeviceChartState => {
  const points = toChartPoints(telemetry)
  const primaryPoint = pickPrimaryPoint(points, browserTest)
  // 只有测试回执没有真实遥测点时不算出图：图表必须来自可见遥测。
  const ready = Boolean(primaryPoint && points.length > 0)
  const generatedFrom: FirstDeviceChartSource = !ready
    ? 'none'
    : browserTest?.status === 'confirmed'
      ? 'browser_test'
      : 'latest_telemetry'

  return {
    ready,
    title: ready ? '第一张遥测图表已生成' : '等待生成第一张遥测图表',
    summary: ready
      ? `${primaryPoint?.key || 'telemetry'} = ${primaryPoint?.value || '--'}；图表已使用 ${points.length} 条最新遥测。`
      : '还没有可见的最新遥测；请发送一次浏览器测试或从设备端上报。',
    primaryKey: primaryPoint?.key || '',
    primaryValue: primaryPoint?.value || '',
    generatedFrom,
    points
  }
}
