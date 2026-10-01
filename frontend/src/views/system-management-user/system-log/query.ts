/**
 * 文件用途: 系统日志列表页的纯查询层（筛选默认值、路由深链解析、时间范围归一化、请求参数序列化）。
 * 核心逻辑: 时间范围 `range` 是唯一真源，请求里的 start_time/end_time 只在序列化时由它派生，
 *          保证"输入框显示的范围"与"实际查询的范围"永远一致。
 * 关键注意事项: 请求参数名（username/start_time/end_time/method/path/ip/action/entity_type/entity_id）
 *          是 /operation_logs 的 REST 契约，不能改名。
 */
import dayjs from 'dayjs'

export const LOG_TIME_FORMAT = 'YYYY-MM-DDTHH:mm:ssZ'

export type LogTimeRange = [number, number] | null

export interface SystemLogQuery {
  username: string
  range: LogTimeRange
  method: string
  path: string
  ip: string
  action: string
  entity_type: string
  entity_id: string
}

export const REQUEST_METHODS = ['POST', 'PUT', 'DELETE'] as const
export const AUDIT_ACTIONS = ['create', 'update', 'delete', 'read', 'other'] as const

/** 默认时间范围：最近一个月。每次调用重新取当前时间，重置时不会沿用挂载时刻。 */
export function defaultLogRange(now = dayjs()): [number, number] {
  return [now.subtract(1, 'month').valueOf(), now.valueOf()]
}

export function defaultSystemLogQuery(): SystemLogQuery {
  return {
    username: '',
    range: defaultLogRange(),
    method: '',
    path: '',
    ip: '',
    action: '',
    entity_type: '',
    entity_id: ''
  }
}

function firstString(value: unknown) {
  if (Array.isArray(value)) return String(value[0] || '')
  return value ? String(value) : ''
}

/**
 * 解析深链（如就绪检查跳转 ?source=ready-check&method=POST&path=...&start_time=...&end_time=...）。
 * 仅接受白名单内的 method；start/end 必须同时合法才覆盖默认范围。
 */
export function systemLogQueryFromRoute(routeQuery: Record<string, unknown>): SystemLogQuery {
  const query = defaultSystemLogQuery()
  const method = firstString(routeQuery.method).toUpperCase()
  if ((REQUEST_METHODS as readonly string[]).includes(method)) query.method = method
  query.path = firstString(routeQuery.path)

  const startRaw = firstString(routeQuery.start_time)
  const endRaw = firstString(routeQuery.end_time)
  const start = startRaw ? dayjs(startRaw) : null
  const end = endRaw ? dayjs(endRaw) : null
  if (start?.isValid() && end?.isValid()) query.range = [start.valueOf(), end.valueOf()]
  return query
}

/**
 * 用户只选了日期（结束时间恰为 00:00:00.000）时，把结束时间推到当天 23:59:59.999；
 * 明确选择了具体时间则保持不变。
 */
export function normalizeLogRange(value: LogTimeRange | undefined): LogTimeRange {
  if (!value || value.length !== 2) return null
  const end = dayjs(value[1])
  const isMidnight = end.hour() === 0 && end.minute() === 0 && end.second() === 0 && end.millisecond() === 0
  return [value[0], isMidnight ? end.endOf('day').valueOf() : value[1]]
}

export function rangeToLogParams(range: LogTimeRange | undefined) {
  if (!range || range.length !== 2) return { start_time: '', end_time: '' }
  return {
    start_time: dayjs(range[0]).format(LOG_TIME_FORMAT),
    end_time: dayjs(range[1]).format(LOG_TIME_FORMAT)
  }
}

/** 筛选状态 -> /operation_logs 请求参数（分页字段由 useListPage 追加）。 */
export function serializeSystemLogQuery(query: SystemLogQuery) {
  const { range, ...filters } = query
  return { ...filters, ...rangeToLogParams(range) }
}
