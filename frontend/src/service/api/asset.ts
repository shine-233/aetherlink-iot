/** 资产（Asset）API（ROADMAP C2 租户客户层级） */
import { request } from '@/service/request'

export interface Asset {
  id: string
  tenant_id: string
  parent_id: string
  name: string
  asset_type: string
  meta?: string | null
  created_at?: string
  updated_at?: string
}

/** 资产树节点（含子节点，后端递归展开） */
export interface AssetTreeNode extends Asset {
  children: AssetTreeNode[]
}

/** 资产分页列表入参 */
export interface AssetListParams {
  parent_id?: string
  keyword?: string
  page?: number
  page_size?: number
}

/** 新建/更新资产入参 */
export interface AssetPayload {
  id?: string
  parent_id?: string
  name: string
  asset_type?: string
  meta?: string
}

/** 资产分页列表（parent_id 为空时列根节点） */
export const assetList = async (params: AssetListParams) => {
  return await request.get('/asset/list', { params })
}

/** 资产树（租户作用域内全量递归） */
export const assetTree = async () => {
  return await request.get('/asset/tree')
}

/** 资产详情 */
export const assetGet = async (id: string) => {
  return await request.get(`/asset/${id}`)
}

/** 新建资产 */
export const assetCreate = async (data: AssetPayload) => {
  return await request.post('/asset', data)
}

/** 更新资产 */
export const assetUpdate = async (data: AssetPayload) => {
  return await request.put('/asset', data)
}

/** 删除资产（存在子节点时后端拒绝） */
export const assetDelete = async (id: string) => {
  return await request.delete(`/asset/${id}`)
}
