/**
 * 文件用途：告警详情弹窗的跳转路由构造（审计日志、设备就绪检查）。
 * 核心逻辑：纯函数，便于单测；时间窗口以告警创建时间前 1 小时到当前后 1 小时为界。
 */
import dayjs from 'dayjs'

const ROUTE_TIME_FORMAT = 'YYYY-MM-DDTHH:mm:ssZ'

export const alarmDeviceId = (device: any) => String(device?.id || device?.device_id || device?.deviceId || '').trim()

export function buildAlarmAuditLogRoute(createAt: unknown, now = dayjs()) {
  const createdAt = createAt ? dayjs(createAt as any) : now.subtract(1, 'day')
  const start = createdAt.isValid() ? createdAt.subtract(1, 'hour') : now.subtract(1, 'day')
  const end = now.add(1, 'hour')
  return {
    name: 'system-management-user_system-log',
    query: {
      method: 'PUT',
      path: '/api/v1/alarm/info/history',
      start_time: start.format(ROUTE_TIME_FORMAT),
      end_time: end.format(ROUTE_TIME_FORMAT)
    }
  }
}

export function buildAlarmDeviceReadyCheckRoute(device: unknown, alarmHistoryId: unknown) {
  const deviceId = alarmDeviceId(device)
  if (!deviceId) return null
  return {
    path: '/device/details',
    query: {
      d_id: deviceId,
      tab: 'ready-check',
      source: 'alarm',
      alarm_history_id: String(alarmHistoryId || '')
    }
  }
}
