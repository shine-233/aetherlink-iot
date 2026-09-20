/**
 * 文件用途：资源中心（TP-5）API 接口包装。
 * 核心逻辑：封装分类目录查询、跨形态综合检索、统一资源包导出/导入与模板一键应用。
 */
import { request } from '../request'
import type { MarketBundleImportResult } from './market'

export interface ResourceCenterItem {
  id: string
  resource_type: 'device_template' | 'board_template'
  name: string
  version: string
  author: string
  description: string
  type_key: string
  path: string
  vis_type?: string
  download_count: number
  created_at: string
  updated_at: string
}

export interface ResourceCenterCatalogEntry {
  type_key: string
  name: string
  device_count: number
  board_count: number
  total_count: number
  download_count: number
}

export interface ResourceCenterListParams {
  page: number
  page_size: number
  resource_type?: 'all' | 'device_template' | 'board_template' | string
  type_key?: string
  keyword?: string
}

export interface ResourceCenterListResult {
  total: number
  page: number
  page_size: number
  list: ResourceCenterItem[]
}

export interface ResourceCenterApplyPayload {
  resource_type: 'device_template' | 'board_template'
  resource_id: string
  target_name?: string
}

export interface ResourceCenterApplyResult {
  resource_type: string
  target_id: string
  target_name: string
  message: string
  resource?: unknown
}

/** 获取资源中心综合分类目录 */
export const getResourceCenterCatalog = async () => {
  return await request.get<ResourceCenterCatalogEntry[]>('/resource/center/catalog')
}

/** 跨形态综合分页检索 */
export const getResourceCenterList = async (params: ResourceCenterListParams) => {
  return await request.get<ResourceCenterListResult>('/resource/center/list', { params })
}

/** 统一资源包导出 */
export const exportResourceBundle = async (params: { type_key?: string; resource_type?: string }) => {
  return await request.get<{ file_name: string; content_base64: string; count: number; bundle: unknown }>(
    '/resource/center/bundle',
    { params }
  )
}

/** 统一资源包导入与冲突预览 */
export const importResourceBundle = async (data: {
  bundle: unknown
  preview?: boolean
  confirm_overwrite?: boolean
}) => {
  return await request.post<MarketBundleImportResult>('/resource/center/bundle/import', data)
}

/** 一键应用/安装资源 */
export const applyResource = async (data: ResourceCenterApplyPayload) => {
  return await request.post<ResourceCenterApplyResult>('/resource/center/apply', data)
}

/** 导出单个看板模板 */
export const exportBoardTemplate = async (boardId: string) => {
  return await request.get<Record<string, unknown>>(`/board/export/${encodeURIComponent(boardId)}`)
}

/** 导入单个看板模板 */
export const importBoardTemplate = async (data: Record<string, unknown>) => {
  return await request.post<Record<string, unknown>>('/board/import', data)
}
