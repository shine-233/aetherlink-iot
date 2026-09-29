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
/** 节点最近调试记录 */
export const ruleChainNodeTraces = async (id: string, nodeId: string, limit = 10) => {
  return await request.get(`/rule-chains/${id}/nodes/${nodeId}/traces`, { params: { limit } })
}
// PHASE-D-D1 END

// TB-18 BEGIN 设备生效规则链解析（档案绑定链优先、租户级链兜底；125.sql）
export interface EffectiveRuleChainRef {
  id: string
  name: string
  /** profile=档案绑定链（优先）；tenant=租户级启用链（兜底） */
  source: 'profile' | 'tenant'
}

export interface DeviceEffectiveRuleChains {
  device_id: string
  device_config_id?: string | null
  default_rule_chain_id?: string | null
  chains: EffectiveRuleChainRef[]
}

/** 解析单设备生效规则链（档案绑定链优先、租户级启用链兜底） */
export const ruleChainDeviceEffective = async (deviceId: string) => {
  return await request.get<DeviceEffectiveRuleChains>(`/rule-chains/device-effective/${deviceId}`)
}
// TB-18 END
