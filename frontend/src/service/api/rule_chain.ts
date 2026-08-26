/** 规则链 API（ROADMAP B2） */
import { request } from '@/service/request'

export interface RuleChainRow {
  id: string
  tenant_id: string
  name: string
  description?: string
  enabled: boolean
  graph: string
  created_at?: string
  updated_at?: string
}

/** 规则链分页列表 */
export const ruleChainList = async (params: object) => {
  return await request.get('/rule-chains/list', { params })
}

/** 规则链详情 */
export const ruleChainGet = async (id: string) => {
  return await request.get(`/rule-chains/${id}`)
}

/** 新建规则链 */
export const ruleChainCreate = async (data: object) => {
  return await request.post('/rule-chains', data)
}

/** 更新规则链 */
export const ruleChainUpdate = async (data: object) => {
  return await request.put('/rule-chains', data)
}

/** 删除规则链 */
export const ruleChainDelete = async (id: string) => {
  return await request.delete(`/rule-chains/${id}`)
}
