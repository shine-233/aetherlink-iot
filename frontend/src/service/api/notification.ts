/**
 * 文件用途: 通知组、通知历史和通知相关用户查询 API wrapper。
 * 核心逻辑: 封装通知组增删改查、通知历史分页和可选用户列表接口。
 * 关键注意事项: 通知组成员、告警绑定和历史筛选条件会影响告警触达证据，字段漂移要同步告警页面。
 * 重构建议: 按通知组、通知历史、用户选择拆分函数，并收紧参数类型。
 */
import { request } from '../request'

// notification-group
export const getNotificationGroupList = async (params: Api.Alarm.NotificationGroupParams) => {
  return await request.get<{
    list: Api.Alarm.NotificationGroupList[]
    total: number
  }>('/notification_group/list', { params })
}

export const getNotificationGroupDetail = async (params: { id: string }) => {
  return await request.get<Api.Alarm.NotificationGroupList>(`/notification_group/${params.id}`)
}

export const deleteNotificationGroup = async (params: { id: string }) => {
  return await request.delete<Api.BaseApi.Data>(`/notification_group/${params.id}`)
}

export const postNotificationGroup = async (params: Api.Alarm.AddNotificationGroupParams) => {
  return await request.post<Api.BaseApi.Data>('/notification_group', params)
}

export const putNotificationGroup = async (
  params: {
    description: string
    name: string
    notification_config: string
    notification_type: string
    remark?: string
    status: string
    tenant_id: string
  },
  id: string
) => {
  return await request.put<Api.BaseApi.Data>(`/notification_group/${id}`, params)
}

export const getUserList = async (params: { page: number; page_size: number; name?: string }) => {
  return await request.get('/user/selector', { params })
}

// notification-record
export const getNotificationHistoryList = async (params: Api.Alarm.NotificationHistoryParams) => {
  return await request.get<{
    list: Api.Alarm.NotificationHistoryList[]
    total: number
  }>('/notification_history/list', { params })
}
