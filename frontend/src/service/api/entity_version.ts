/** 实体版本控制 API（ROADMAP C7，对标 ThingsBoard 3.5+ Version Control） */
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
