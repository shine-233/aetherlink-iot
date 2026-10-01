/**
 * 文件用途: 告警通知记录列表页的纯查询层（筛选默认值与请求参数序列化）。
 * 核心逻辑: 时间范围 `range` 是唯一真源；请求时派生 send_time_start / send_time_stop。
 * 关键注意事项: /notification_history/list 的结束时间参数名是 send_time_stop（不是 send_time_end），
 *          这是既有 REST 契约，改名会让后端静默忽略结束时间。
 */
import dayjs from 'dayjs'

export const SEND_TIME_FORMAT = 'YYYY-MM-DDTHH:mm:ssZ'

export type SendTimeRange = [number, number] | null

export interface NotificationRecordQuery {
  notification_type: string | null
  send_target: string
  range: SendTimeRange
}

export function defaultNotificationRecordQuery(now = dayjs()): NotificationRecordQuery {
  return {
    notification_type: '',
    send_target: '',
    range: [now.subtract(1, 'month').valueOf(), now.valueOf()]
  }
}

export function rangeToSendTimeParams(range: SendTimeRange | undefined) {
  if (!range || range.length !== 2) return { send_time_start: '', send_time_stop: '' }
  return {
    send_time_start: dayjs(range[0]).format(SEND_TIME_FORMAT),
    send_time_stop: dayjs(range[1]).format(SEND_TIME_FORMAT)
  }
}

/** 筛选状态 -> /notification_history/list 请求参数（分页字段由 useListPage 追加）。 */
export function serializeNotificationRecordQuery(query: NotificationRecordQuery) {
  return {
    notification_type: query.notification_type,
    send_target: query.send_target,
    ...rangeToSendTimeParams(query.range)
  }
}
