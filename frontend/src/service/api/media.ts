/**
 * 文件用途：媒体库（media_files，TB-41）前端 API 客户端。
 * 核心逻辑：封装通用上传（/file/up 落盘即登记）、媒体列表/详情/删除接口，
 *   供 views/management/media 媒体库工作台使用。
 */
import { request } from '../request'
import { createResource } from './resource'

export interface MediaFileItem {
  id: string
  tenant_id: string
  /** 上传时的原始文件名 */
  file_name: string
  /** 对外访问路径（如 ./files/board/2026-09-25/xxx.png），可直接用于预览 */
  file_path: string
  /** 字节数 */
  file_size: number
  mime: string
  /** 最近一次引用扫描的引用方数量 */
  referenced_count: number
  created_at?: string
}

export interface MediaFileListParams {
  page?: number
  page_size?: number
  search?: string
  mime?: string
}

export interface MediaFileListResponse {
  list: MediaFileItem[]
  total: number
}

export interface MediaReferencer {
  /** 引用来源：board / scada_document / ota_package */
  kind: string
  id: string
  name: string
}

export interface MediaFileDetailResponse {
  file: MediaFileItem
  referencers: MediaReferencer[]
}

export interface MediaFileDeleteResponse {
  deleted: boolean
}

/** 通用上传（后端 UpFile 落盘成功即写 media_files 登记），返回对外访问路径 */
export const uploadMediaFile = async (formData: FormData) => {
  return await request.post<{ path?: string }>('/file/up', formData)
}

const mediaFiles = createResource<
  MediaFileListParams,
  MediaFileListResponse,
  MediaFileDetailResponse,
  never,
  never,
  MediaFileDeleteResponse
>({ collection: '/media/files' })

/** 分页与条件查询本租户媒体列表 */
export const getMediaFilesList = mediaFiles.list

/** 获取媒体详情（含实时引用统计） */
export const getMediaFileDetail = mediaFiles.detail

/** 删除媒体：引用计数>0 时后端拒绝并返回引用方，否则删文件+删登记行 */
export const deleteMediaFile = mediaFiles.remove
