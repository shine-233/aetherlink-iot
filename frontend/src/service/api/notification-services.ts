/**
 * 文件用途: 邮件、短信和推送通知服务配置 API wrapper。
 * 核心逻辑: 读取和保存各类通知服务配置，并提供邮件测试发送入口。
 * 关键注意事项: 服务凭证、收件配置和测试发送结果影响告警通知可达性，日志中不要泄露敏感配置。
 * 重构建议: 拆分邮件、短信、推送配置类型，并补充保存和测试发送的失败分支测试。
 */
import { request } from '../request'

export interface AlarmEmailTemplate {
  id: string
  tenant_id: string
  name: string
  purpose: 'ALARM'
  subject_template: string
  body_template: string
  enabled: boolean
  is_default: boolean
  created_by: string
  created_at: string
  updated_at: string
}

export interface AlarmEmailTemplatePayload {
  name: string
  subject_template: string
  body_template: string
  enabled: boolean
  is_default: boolean
}

export interface AlarmEmailTemplatePreviewPayload {
  subject_template: string
  body_template: string
  subject?: string
  message?: string
  device_ids?: string[]
}

/** 通知服务配置 */
export const fetchNotificationServicesEmail = async () => {
  const data = await request.get<Api.NotificationServices.Email | null>(`/notification/services/config/EMAIL`)
  return data
}

export const fetchNotificationServicesSms = async () => {
  const data = await request.get<Api.NotificationServices.Sms | null>('/notification/services/config/SME_CODE')
  return data
}

/** 修改通知服务配置 */
export const editNotificationServices = async (params: any) => {
  const data = await request.post<Api.BaseApi.Data>('/notification/services/config', params)
  return data
}

/** 发送测试邮件 */
export const sendTestEmail = async (params: any) => {
  const data = await request.post<Api.BaseApi.Data>('/notification/services/config/e-mail/test', params)
  return data
}

export const fetchAlarmEmailTemplates = async (params: { page: number; page_size: number }) => {
  return await request.get<{ list: AlarmEmailTemplate[]; total: number }>('/notification/e-mail/templates', { params })
}

export const createAlarmEmailTemplate = async (params: AlarmEmailTemplatePayload) => {
  return await request.post<AlarmEmailTemplate>('/notification/e-mail/templates', params)
}

export const updateAlarmEmailTemplate = async (id: string, params: AlarmEmailTemplatePayload) => {
  return await request.put<AlarmEmailTemplate>(`/notification/e-mail/templates/${encodeURIComponent(id)}`, params)
}

export const deleteAlarmEmailTemplate = async (id: string) => {
  return await request.delete<Api.BaseApi.Data>(`/notification/e-mail/templates/${encodeURIComponent(id)}`)
}

export const setDefaultAlarmEmailTemplate = async (id: string) => {
  return await request.post<Api.BaseApi.Data>(`/notification/e-mail/templates/${encodeURIComponent(id)}/default`)
}

export const previewAlarmEmailTemplate = async (params: AlarmEmailTemplatePreviewPayload) => {
  return await request.post<{ subject: string; body: string }>('/notification/e-mail/templates/preview', params)
}

/** 推送服务配置 */
export const fetchPushNotificationServices = async () => {
  const data = await request.get<Api.NotificationServices.PushNotification>('/message_push/config')
  return data
}

/** 修改推送服务配置 */
export const editPushNotificationServices = async (params: any) => {
  const data = await request.post<Api.BaseApi.Data>('/message_push/config', params)
  return data
}
