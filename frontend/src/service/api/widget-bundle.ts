/**
 * 文件用途：Widget Bundle（部件库，TB-04）前端 API 客户端。
 * 核心逻辑：封装部件库的增删改查、内置四部件导出描述与一键种子落库接口。
 */
import { request } from '../request'
import { createResource } from './resource'

export interface WidgetBundleItem {
  id: string
  name: string
  tenant_id: string
  /** 部件定义 JSON 数组字符串（形状对齐后端 WidgetDefinition：type/version/schema/capabilities/commands） */
  widgets: string
  description?: string
  version?: string
  type_key?: string
  created_at?: string
  updated_at?: string
}

export interface WidgetBundleListParams {
  page?: number
  page_size?: number
  search?: string
  type_key?: string
}

export interface WidgetBundleListResponse {
  list: WidgetBundleItem[]
  total: number
}

export interface CreateWidgetBundleParams {
  name: string
  widgets?: string
  description?: string
  version?: string
  type_key?: string
}

export interface UpdateWidgetBundleParams {
  id: string
  name?: string
  widgets?: string
  description?: string
  version?: string
  type_key?: string
}

export interface WidgetBundleExport {
  kind: string
  name: string
  author?: string
  version?: string
  description?: string
  type_key?: string
  widgets: string
  exported_at?: string
}

export interface WidgetBundleSeedResponse {
  bundle: WidgetBundleItem | null
  idempotent: boolean
}

const bundles = createResource<
  WidgetBundleListParams,
  WidgetBundleListResponse,
  WidgetBundleItem,
  CreateWidgetBundleParams,
  UpdateWidgetBundleParams,
  boolean
>({ collection: '/widget-bundles' })

/** 分页与条件查询部件库列表 */
export const getWidgetBundlesList = bundles.list

/** 获取部件库详情 */
export const getWidgetBundleDetail = bundles.detail

/** 创建部件库 */
export const createWidgetBundle = bundles.create

/** 更新部件库 */
export const updateWidgetBundle = bundles.update

/** 删除部件库 */
export const deleteWidgetBundle = bundles.remove

/** 内置四部件（gauge/chart/valve/twin3d）种子 bundle 导出描述 */
export const getBuiltinWidgetBundle = async () => {
  return await request.get<WidgetBundleExport>('/widget-bundles/builtin')
}

/** 一键把内置四部件落为租户可管理种子 bundle（内容一致时幂等返回） */
export const seedBuiltinWidgetBundle = async () => {
  return await request.post<WidgetBundleSeedResponse>('/widget-bundles/seed')
}
