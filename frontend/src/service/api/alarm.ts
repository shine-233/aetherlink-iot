/**
 * 文件用途: 告警消息、告警配置、告警历史和通知对象相关 API wrapper。
 * 核心逻辑: 将告警页面的新增、编辑、删除、分页查询、处理记录和通知关系操作映射到后端接口。
 * 关键注意事项: 告警级别、处理状态、通知组和历史筛选条件会影响告警闭环判断，字段变更需同步后端和自动化测试。
 * 重构建议: 按告警规则、告警历史、告警处理、通知配置拆分函数组，并补齐错误分支与参数位置测试。
 */
/*
 * @Descripttion:
 * @version:
 * @Author: zhaoqi
 * @Date: 2024-03-18 15:57:57
 * @LastEditors: zhaoqi
 * @LastEditTime: 2024-03-19 10:08:22
 */
import { request } from '../request'

/** Create alarm configuration. */
export const addWarningMessage = async (params: object) => {
  const data = await request.post('/alarm/config', params)
  return data
}
/** List alarm configurations. */
export const warningMessageList = async (params: object) => {
  const data = await request.get('/alarm/config', {
    params
  })
  return data
}
/** Update alarm configuration, including enable/disable state. */
export const editInfo = async (params: object) => {
  const data = await request.put('/alarm/config', params)
  return data
}

/** Update alarm configuration text. */
export const editInfoText = async (params: object) => {
  const data = await request.put('/alarm/config', params)
  return data
}

/** Delete alarm configuration. */
export const delInfo = async (id: string) => {
  const data = await request.delete(`/alarm/config/${id}`)
  return data
}

/** List active alarm messages. */
export const infoList = async (params: object) => {
  const data = await request.get('/alarm/info', {
    params
  })
  return data
}
/** List alarm history. */
export const alarmHistory = async (params: object) => {
  const data = await request.get('/alarm/info/history', {
    params
  })
  return data
}

export interface AlarmHistoryMonthlyTrendPoint {
  month: number
  count: number
}

export interface AlarmHistoryMonthlyTrendData {
  year: number
  months: AlarmHistoryMonthlyTrendPoint[]
}

/** Get twelve monthly alarm occurrence buckets for a selected calendar year. */
export const alarmHistoryMonthlyTrend = async (
  year: number,
  timezone: string,
  options?: { all_tenants?: boolean }
) => {
  const data = await request.get<AlarmHistoryMonthlyTrendData>('/alarm/info/history/monthly', {
    params: {
      year,
      timezone,
      ...(options?.all_tenants ? { all_tenants: true } : {})
    }
  })
  return data
}

/** Mark alarm messages as processed. */
export const processingOperation = async (params: object) => {
  const data = await request.put('/alarm/info', params)
  return data
}
/** Batch process alarm messages. */
export const batchProcessing = async (params: object) => {
  const data = await request.put('/alarm/info/batch', params)
  return data
}

/** Acknowledge an alarm history record. */
export const acknowledgeAlarmHistory = async (id: string) => {
  const data = await request.put(`/alarm/info/history/${encodeURIComponent(id)}/acknowledge`)
  return data
}

/** Reset an alarm history record. */
export const resetAlarmHistory = async (id: string) => {
  const data = await request.put(`/alarm/info/history/${encodeURIComponent(id)}/reset`)
  return data
}

/** Batch acknowledge or reset alarm history records. */
export const batchActionAlarmHistory = async (params: {
  ids: string[]
  action: 'acknowledge' | 'reset'
  note?: string
}) => {
  const data = await request.put('/alarm/info/history/batch-action', params)
  return data
}
