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

// PHASE-D-D1 BEGIN 节点级调试 trace（画布调试面板）
export interface RuleChainNodeTraceRow {
  id: string
  exec_id: string
  chain_id: string
  node_id: string
  node_type: string
  pass: boolean
  error_msg?: string
  elapsed_ms: number
  tenant_id: string
  created_at: string
}

/** 节点最近调试记录 */
export const ruleChainNodeTraces = async (id: string, nodeId: string, limit = 10) => {
  return await request.get(`/rule-chains/${id}/nodes/${nodeId}/traces`, { params: { limit } })
}
// PHASE-D-D1 END
