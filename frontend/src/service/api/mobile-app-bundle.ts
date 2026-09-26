/**
 * 文件用途：移动应用中心（mobile_app_bundles，TB-23）前端 API 客户端。
 * 核心逻辑：封装应用包上传登记（multipart：file+platform+version+release_notes）、
 *   租户内分页列表、详情、draft 发布说明更新、publish/archive 状态机流转、删除与下载地址解析。
 * 关键注意事项：后端状态机为 draft→published→archived（非法流转返回 202005），
 *   同租户同平台同版本重复登记返回 202006；本模块只透传，不做状态推断——
 *   按钮禁用等状态推导由页面依据 status 字段判定，与后端 CanTransitionAppBundle 保持同源口径。
 * 重构建议：uniapp 对接阶段若开放终端拉取面（published 专用下载），另立客户端函数，
 *   不要复用本管理面上传/删除入口。
 */
import { request } from '../request'

export type MobileAppBundlePlatform = 'android' | 'ios' | 'h5'
export type MobileAppBundleStatus = 'draft' | 'published' | 'archived'

export interface MobileAppBundleItem {
  id: string
  tenant_id: string
  version: string
  platform: MobileAppBundlePlatform
  /** 上传时的原始文件名（已清洗） */
  file_name: string
  /** 对外访问路径（./files/apps/<platform>/<日期>/<哈希>.<ext>） */
  file_path: string
  /** 字节数 */
  file_size: number
  /** 文件 SHA-256 十六进制（上传时计算，入库后只读） */
  checksum: string
  release_notes: string
  status: MobileAppBundleStatus
  /** 发布时间（draft 为 null；归档后保留） */
  published_at?: string | null
  created_at?: string
  updated_at?: string
}

export interface MobileAppBundleListParams {
  page?: number
  page_size?: number
  platform?: MobileAppBundlePlatform
  status?: MobileAppBundleStatus
}

export interface MobileAppBundleListResponse {
  list: MobileAppBundleItem[]
  total: number
}

export interface MobileAppBundleUpdatePayload {
  release_notes?: string
}

/** 上传并登记应用包（状态固定落 draft）；同租户同平台同版本重复时返回 202006 */
export const uploadMobileAppBundle = async (formData: FormData) => {
  return await request.post<MobileAppBundleItem>('/mobile/app_bundles/upload', formData)
}

/** 分页查询本租户应用包列表（platform/status 精确过滤） */
export const getMobileAppBundles = async (params?: MobileAppBundleListParams) => {
  return await request.get<MobileAppBundleListResponse>('/mobile/app_bundles', { params })
}

/** 获取应用包详情 */
export const getMobileAppBundle = async (id: string) => {
  return await request.get<MobileAppBundleItem>(`/mobile/app_bundles/${encodeURIComponent(id)}`)
}

/** 更新应用包：仅 draft 可改发布说明（published/archived 返回 202005） */
export const updateMobileAppBundle = async (id: string, payload: MobileAppBundleUpdatePayload) => {
  return await request.put<MobileAppBundleItem>(`/mobile/app_bundles/${encodeURIComponent(id)}`, payload)
}

/** 发布：draft→published（重复发布/已归档返回 202005），落 published_at */
export const publishMobileAppBundle = async (id: string) => {
  return await request.post<MobileAppBundleItem>(`/mobile/app_bundles/${encodeURIComponent(id)}/publish`)
}

/** 归档：published→archived（draft/已归档返回 202005），published_at 保留作发布履历 */
export const archiveMobileAppBundle = async (id: string) => {
  return await request.post<MobileAppBundleItem>(`/mobile/app_bundles/${encodeURIComponent(id)}/archive`)
}

/** 删除应用包：仅 draft/archived（published 须先归档），后端连文件本体一起删除 */
export const deleteMobileAppBundle = async (id: string) => {
  return await request.delete<{ deleted: boolean }>(`/mobile/app_bundles/${encodeURIComponent(id)}`)
}
