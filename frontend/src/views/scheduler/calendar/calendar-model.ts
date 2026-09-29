/**
 * 文件用途：调度日历（TB-48）的纯函数模型——月网格、按日分桶、表单校验与请求体构建。
 * 关键注意事项：
 *  - 全部按本地时区计算；网格固定 6×7，周一为首列。
 *  - 表单按 event_type 条件取字段：rpc 只发 next_run_at（cron 必须为空），其余只发 cron。
 */
import type { SchedulerEventItem, SchedulerEventType } from '@/service/api'

export interface CalendarCell {
  date: Date
  inMonth: boolean
  isToday: boolean
  /** 'YYYY-MM-DD'（本地时区），事件分桶键。 */
  key: string
}

export interface SchedulerFormModel {
  name: string
  event_type: SchedulerEventType
  ref_id: string
  cron: string
  next_run_at: number | null
  enabled: boolean
}

export const SOURCE_TYPES: readonly SchedulerEventType[] = ['scene', 'report', 'rpc']

export const SOURCE_CLASS: Record<SchedulerEventType, string> = {
  scene: 'scheduler-event--scene',
  report: 'scheduler-event--report',
  rpc: 'scheduler-event--rpc'
}

export const SOURCE_TAG_TYPE: Record<SchedulerEventType, 'success' | 'info' | 'warning'> = {
  scene: 'success',
  report: 'info',
  rpc: 'warning'
}

/** i18n key for a source type label, e.g. scene → page.schedulerCalendar.sourceScene. */
export function sourceLabelKey(type: SchedulerEventType) {
  return `page.schedulerCalendar.source${type.charAt(0).toUpperCase()}${type.slice(1)}`
}

export function startOfMonth(date: Date) {
  return new Date(date.getFullYear(), date.getMonth(), 1)
}

export function shiftMonthDate(month: Date, delta: number) {
  return new Date(month.getFullYear(), month.getMonth() + delta, 1)
}

export function localDateKey(date: Date) {
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

/** 当月窗口（本地时区）的 epoch 毫秒边界，配合后端 from_ms/to_ms 过滤。 */
export function monthWindow(month: Date) {
  const from = new Date(month.getFullYear(), month.getMonth(), 1, 0, 0, 0, 0)
  const to = new Date(month.getFullYear(), month.getMonth() + 1, 0, 23, 59, 59, 999)
  return { fromMs: from.getTime(), toMs: to.getTime() }
}

/** 6 行 × 7 列的月网格单元格。 */
export function buildCalendarCells(month: Date, today = new Date()): CalendarCell[] {
  const first = startOfMonth(month)
  const firstWeekday = (first.getDay() + 6) % 7 // 周一=0
  const todayKey = localDateKey(today)
  const cells: CalendarCell[] = []
  for (let index = 0; index < 42; index += 1) {
    const date = new Date(first.getFullYear(), first.getMonth(), 1 - firstWeekday + index)
    const key = localDateKey(date)
    cells.push({ date, inMonth: date.getMonth() === first.getMonth(), isToday: key === todayKey, key })
  }
  return cells
}

/** 周一→周日的本地化短名（Intl 取文案，避免自维护 7 个翻译键）。 */
export function weekdayLabels(locale: string) {
  const formatter = new Intl.DateTimeFormat(locale, { weekday: 'short' })
  // 2023-01-02 是周一。
  return Array.from({ length: 7 }, (_, index) => formatter.format(new Date(2023, 0, 2 + index)))
}

/** 事件按本地日分桶（next_run_at 缺失的不可调度条目不进网格）。 */
export function bucketEventsByDay(events: SchedulerEventItem[]) {
  const buckets = new Map<string, SchedulerEventItem[]>()
  for (const event of events) {
    if (!event.next_run_at) continue
    const key = localDateKey(new Date(event.next_run_at))
    const bucket = buckets.get(key)
    if (bucket) bucket.push(event)
    else buckets.set(key, [event])
  }
  return buckets
}

export function emptyFormModel(): SchedulerFormModel {
  return { name: '', event_type: 'scene', ref_id: '', cron: '', next_run_at: null, enabled: true }
}

export function formModelFromRow(row: SchedulerEventItem): SchedulerFormModel {
  return {
    name: row.name,
    event_type: row.source_type,
    ref_id: row.ref_id,
    cron: row.cron,
    next_run_at: row.next_run_at ? new Date(row.next_run_at).getTime() : null,
    enabled: row.enabled
  }
}

/** Returns the i18n key of the first validation error, or null when the form is valid. */
export function validateFormModel(model: SchedulerFormModel): string | null {
  if (!model.name.trim()) return 'page.schedulerCalendar.formNameRequired'
  if (model.event_type === 'scene' && !model.ref_id.trim()) return 'page.schedulerCalendar.formRefIdRequired'
  if (model.event_type !== 'rpc' && !model.cron.trim()) return 'page.schedulerCalendar.formCronRequired'
  if (model.event_type === 'rpc' && !model.next_run_at) return 'page.schedulerCalendar.formRunAtRequired'
  return null
}

/** Fields shared by create and update payloads. */
export function buildEventPayload(model: SchedulerFormModel) {
  const isRpc = model.event_type === 'rpc'
  return {
    name: model.name.trim(),
    ref_id: model.ref_id.trim() || undefined,
    cron: isRpc ? undefined : model.cron.trim(),
    next_run_at: isRpc && model.next_run_at ? new Date(model.next_run_at).toISOString() : undefined,
    enabled: model.enabled
  }
}
