/**
 * 文件用途：解决方案模板引擎（ROADMAP TB-19）前端 API 客户端。
 * 核心逻辑：封装行业方案的创建、分页列表、详情（含安装流水）、一键安装与删除。
 * 关键注意事项：方案只存资源引用；安装走资源中心应用管道，每次安装实例化一套新资产。
 */
import { request } from '../request'

export interface SolutionResourceRef {
  resource_type: 'device_template' | 'board_template'
  resource_id: string
  /** 可选：安装时覆盖实例名称 */
  target_name?: string
}

export interface IndustrySolutionItem {
  id: string
  tenant_id: string
  name: string
  description?: string
  resources: SolutionResourceRef[]
  status: 'active' | 'disabled'
  created_at: string
  updated_at: string
}

export interface SolutionInstallRow {
  id: string
  tenant_id: string
  solution_id: string
  solution_name: string
  item_index: number
  resource_type: string
  resource_id: string
  target_id?: string
  status: 'applied' | 'failed'
  error?: string
  created_at: string
}

export interface SolutionInstallItemResult {
  item_index: number
  resource_type: string
  resource_id: string
  status: 'applied' | 'failed'
  target_id?: string
  target_name?: string
  error?: string
}

export interface SolutionInstallResponse {
  solution_id: string
  solution_name: string
  total: number
  applied: number
  failed: number
  items: SolutionInstallItemResult[]
}

/** 创建行业方案 */
export const createIndustrySolution = async (params: {
  name: string
  description?: string
  resources: SolutionResourceRef[]
}) => {
  return await request.post<IndustrySolutionItem>('/solutions', params)
}

/** 行业方案分页列表 */
export const listIndustrySolutions = async (params?: { page?: number; page_size?: number }) => {
  return await request.get<{ total: number; list: IndustrySolutionItem[] }>('/solutions', { params })
}

/** 方案详情（含安装流水） */
export const getIndustrySolution = async (id: string) => {
  return await request.get<{ solution: IndustrySolutionItem; installs: SolutionInstallRow[] }>(
    `/solutions/${id}`
  )
}

/** 删除方案（不删除已安装的实例） */
export const deleteIndustrySolution = async (id: string) => {
  return await request.delete(`/solutions/${id}`)
}

/** 一键安装方案（逐项应用并留流水） */
export const installIndustrySolution = async (
  id: string,
  params?: { continue_on_error?: boolean }
) => {
  return await request.post<SolutionInstallResponse>(`/solutions/${id}/install`, params ?? {})
}
