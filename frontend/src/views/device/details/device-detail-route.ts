/**
 * 文件用途: 设备详情页路由查询参数与告警状态的纯函数归一化。
 * 核心逻辑: d_id / tab / shared / access 查询参数可能是数组、空值或非字符串；告警字段在历史接口中有
 * warn_status / warnStatus / alarm_status / alarmStatus 多种命名和 Y/N、0/1、布尔等多种取值。
 * 关键注意事项: 这里只做无副作用归一化，供 index.vue 与 useDeviceDetailLoader 共用并单测覆盖。
 */

export function normalizeRouteQueryParam(value: unknown): string {
  if (Array.isArray(value)) return normalizeRouteQueryParam(value[0])
  if (value === null || value === undefined) return ''
  return String(value)
}

/** 分享只读入口：?shared=1|true 或 ?access=shared。 */
export function isSharedRouteQuery(query: Record<string, unknown>): boolean {
  const shared = normalizeRouteQueryParam(query.shared).toLowerCase()
  const access = normalizeRouteQueryParam(query.access).toLowerCase()
  return shared === '1' || shared === 'true' || access === 'shared'
}

const FALSY_ALARM_TEXT = new Set(['', '0', 'false', 'off', 'no', 'n'])
const TRUTHY_ALARM_TEXT = new Set(['1', 'true', 'on', 'yes', 'y'])

export function normalizeAlarmActive(raw: unknown): boolean {
  if (typeof raw === 'boolean') return raw
  if (typeof raw === 'number') return raw > 0
  if (typeof raw === 'string') {
    const normalized = raw.trim().toLowerCase()
    if (FALSY_ALARM_TEXT.has(normalized)) return false
    if (TRUTHY_ALARM_TEXT.has(normalized)) return true
  }
  return Boolean(raw)
}

export function resolveDeviceAlarmActive(deviceData: Record<string, any> | null | undefined): boolean {
  return normalizeAlarmActive(
    deviceData?.warn_status ?? deviceData?.warnStatus ?? deviceData?.alarm_status ?? deviceData?.alarmStatus
  )
}

/** 子组件 @change 可能冒泡原生 DOM 事件（radio/select 切换），这类事件不应触发整页详情重载。 */
export function isDomEventPayload(payload: unknown): boolean {
  return (
    payload instanceof Event ||
    Boolean(payload && typeof payload === 'object' && 'target' in payload && 'bubbles' in payload)
  )
}
