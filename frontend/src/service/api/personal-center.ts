/**
 * 文件用途: 个人中心资料、邮箱、告警邮箱、语言、密码和头像相关 API wrapper。
 * 核心逻辑: 封装当前用户资料读取与修改，以及账号安全和偏好设置请求。
 * 关键注意事项: 邮箱验证码、密码修改和语言偏好会影响账号安全与全局显示状态，失败提示应保持可见。
 * 重构建议: 拆分资料设置、账号安全、通知偏好和文件上传模块，并补充参数类型。
 */
/*
 * @Descripttion:
 * @version:
 * @Author: zhaoqi
 * @Date: 2024-03-18 09:47:03
 * @LastEditors: zhaoqi
 * @LastEditTime: 2024-03-18 11:34:34
 */
import { request } from '../request'

/** Get personal information */
export const fetchUserInfo = async () => {
  return await request.get<Api.BaseApi.Data>('/board/user/info', {})
}
/** Update personal basic information */
export const changeInformation = async (params: Record<string, unknown>) => {
  const data = await request.post<Api.BaseApi.Data>('/board/user/update', params)
  return data
}
/** Change account email and keep device ownership */
export const changeAccountEmail = async (params: { new_email: string; verify_code: string }) => {
  const data = await request.post<{ new_email: string; devices_migrated: number }>('/user/change-email', params)
  return data
}
/** Global warning email recipients are stored against the current tenant warning-email endpoint. */
export const fetchWarningEmails = async () => {
  const data = await request.get<string[]>('/user/warning-email')
  return data
}
export const updateWarningEmails = async (params: { emails: string[] }) => {
  const data = await request.put<string[]>('/user/warning-email', params)
  return data
}
/** Save the preferred interface language through the current account preference endpoint. */
export const savePreferredLanguage = async (params: {
  prefer_lang?: string
  default_language?: string
}) => {
  const data = await request.post<Api.BaseApi.Data>('/user/prefer-lang', params)
  return data
}
/** Change password */
export const passwordModification = async (params: Record<string, unknown>) => {
  const data = await request.post('/board/user/update/password', params)
  return data
}
/** Upload file */
export const uploadFile = async (params: FormData) => {
  const data = await request.post<{ path?: string }>('/file/up', params)
  return data
}
