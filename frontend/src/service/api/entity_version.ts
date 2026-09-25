/** 实体版本控制 API（ROADMAP C7/TB-25，对标 ThingsBoard 3.5+ Version Control） */
import { request } from '@/service/request'

/** 后端 resolveEntityTable 白名单：board / rule_chain / device_config / calculated_field */
export type EntityVersionEntityType = 'board' | 'rule_chain' | 'device_config' | 'calculated_field'

export interface EntityVersion {
  id: string
  tenant_id: string
  entity_type: string
  entity_id: string
  version_number: number
  /** 快照 JSON 字符串（后端以 jsonb 存储，接口按 string 返回） */
  snapshot: string
  remark?: string | null
  created_by?: string | null
  created_at?: string
}

export interface EntityVersionListParams {
  entity_type: EntityVersionEntityType | string
  entity_id: string
  page?: number
  page_size?: number
}

export interface EntityVersionCreatePayload {
  entity_type: EntityVersionEntityType | string
  entity_id: string
  remark?: string
}

export interface EntityVersionRestoreResult {
  dry_run: boolean
  fields: Record<string, unknown> | null
}

/** 版本历史分页列表（entity_type + entity_id 共同定位一个实体） */
export const entityVersionList = async (params: EntityVersionListParams) => {
  return await request.get('/entity_versions', { params })
}

/** 为实体当前状态创建一条快照 */
export const entityVersionCreate = async (data: EntityVersionCreatePayload) => {
  return await request.post('/entity_versions', data)
}

/** 版本详情（含完整快照内容） */
export const entityVersionGet = async (id: string) => {
  return await request.get(`/entity_versions/${id}`)
}

/** 恢复版本；dry_run=true 时只回显将写入的字段，不落库 */
export const entityVersionRestore = async (id: string, dryRun = false) => {
  return await request.post(`/entity_versions/${id}/restore`, { dry_run: dryRun })
}

/** 单条路径变更：kind ∈ added|removed|modified（路径以点号表达，数组下标为路径段） */
export interface EntityVersionDiffChange {
  path: string
  kind: 'added' | 'removed' | 'modified'
  old_value?: unknown
  new_value?: unknown
}

/** 两份快照的语义差异汇总：三类路径列表 + 同序明细 + 总数 */
export interface EntityVersionDiffResult {
  added: string[]
  removed: string[]
  modified: string[]
  changes: EntityVersionDiffChange[]
  total: number
}

/** 版本对比响应：source 为基准版本，target 为对比版本，diff 为语义差异 */
export interface EntityVersionDiffPayload {
  source: EntityVersion
  target: EntityVersion
  diff: EntityVersionDiffResult
}

/** 对比两个版本快照的 JSON 语义差异（id 为基准版本，targetId 为对比版本） */
export const entityVersionDiff = async (id: string, targetId: string) => {
  return await request.get(`/entity_versions/${id}/diff/${targetId}`)
}
