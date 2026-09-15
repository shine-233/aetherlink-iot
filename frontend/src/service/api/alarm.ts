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

// ROADMAP TB-1 第一片：告警评论。
// 评论挂 alarm_history（现代告警记录），不是已废弃的 alarm_info。
// 三条端点都在 history/:id/comment 下；路径形状受后端 Gin 路由树约束
// （同一段不能既有 :id 又有静态串），不要改成 history/comment/:id。

/** 一条告警评论。 */
export interface AlarmComment {
  id: string
  tenant_id: string
  alarm_history_id: string
  content: string
  author_user_id: string
  created_at: string
}

/** 列出某条告警历史的评论（时间正序）。 */
export const listAlarmComments = async (alarmHistoryId: string) => {
  const data = await request.get<{ list: AlarmComment[] }>(
    `/alarm/info/history/${encodeURIComponent(alarmHistoryId)}/comment`
  )
  return data
}

/** 新增一条告警评论。 */
export const createAlarmComment = async (alarmHistoryId: string, content: string) => {
  const data = await request.post<AlarmComment>(
    `/alarm/info/history/${encodeURIComponent(alarmHistoryId)}/comment`,
    { content }
  )
  return data
}

/** 删除一条告警评论（仅作者本人或租户管理员可删）。 */
export const deleteAlarmComment = async (alarmHistoryId: string, commentId: string) => {
  const data = await request.delete(
    `/alarm/info/history/${encodeURIComponent(alarmHistoryId)}/comment/${encodeURIComponent(commentId)}`
  )
  return data
}

// ROADMAP TB-1 第二片：告警指派（处理人）审计流水。
// 与评论同源，都挂 alarm_history；路径形状受后端 Gin 路由树约束
// （同一段不能既有 :id 又有静态串），必须写成 history/:id/assignment，
// 不要改成 history/assignment/:id。
// 语义（迁移 103 约定）：流水是 append-only 审计记录，不提供 UPDATE/DELETE；
// assignee_user_id 为 NULL 表示"取消指派"，当前处理人 = 最新一条的 assignee_user_id。

/** 一条告警指派流水。assignee_user_id 为 null 表示"取消指派"。 */
export interface AlarmAssignment {
  id: string
  tenant_id: string
  alarm_history_id: string
  assignee_user_id: string | null
  operator_user_id: string
  remark: string
  created_at: string
}

/** 列出某条告警历史的指派流水（created_at 倒序，最新在前）。 */
export const listAlarmAssignments = async (alarmHistoryId: string) => {
  const data = await request.get<{ list: AlarmAssignment[] }>(
    `/alarm/info/history/${encodeURIComponent(alarmHistoryId)}/assignment`
  )
  return data
}

/**
 * 写入一条指派流水：assignee_user_id 为 null 即"取消指派"。
 * 因为是 append-only，改派与取消都走同一个 POST，不提供改/删接口。
 */
export const assignAlarm = async (
  alarmHistoryId: string,
  params: { assignee_user_id: string | null; remark?: string }
) => {
  const data = await request.post<AlarmAssignment>(
    `/alarm/info/history/${encodeURIComponent(alarmHistoryId)}/assignment`,
    params
  )
  return data
}
