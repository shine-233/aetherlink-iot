/**
 * 文件用途: 通用实体关系 API wrapper（ROADMAP P1.1）。
 * 核心逻辑: 关系创建、查询、单条删除、按实体删除（protect/cascade）。
 * 关键注意事项:
 *  1. **不传 tenant_id**：租户一律由后端从 claims 推导，前端传了也会被忽略。
 *     这里刻意不提供该参数，避免出现"前端以为能切租户"的错觉。
 *  2. **按实体删除必须显式带 policy**。后端默认 protect（有关系即拒绝），
 *     所以"删除实体"失败不是 bug，是保护生效——错误必须能区分这两种情形，
 *     否则用户会以为接口坏了。
 *  3. 跨租户与不存在都返回 404，前端不做 403 分支——后端刻意不泄露 ID 是否存在。
 */
import { request } from '../request'
import type {
  DeletionPolicy,
  EntityRelation,
  EntityType,
  RelationDirection
} from '@/views/device/entity-relation/entity-relation-model'

export interface CreateEntityRelationPayload {
  from_type: EntityType
  from_id: string
  relation_type: string
  to_type: EntityType
  to_id: string
  metadata?: string
}

export interface ListEntityRelationsParams {
  from_type?: string
  from_id?: string
  to_type?: string
  to_id?: string
  relation_type?: string
  entity_type?: string
  entity_id?: string
  direction?: RelationDirection
  limit?: number
  offset?: number
}

export interface EntityRelationList {
  list: EntityRelation[]
  total: number
}

export interface DeleteByEntityPayload {
  entity_type: EntityType
  entity_id: string
  policy: DeletionPolicy
}

/** 创建关系。幂等：相同端点与类型重复创建返回既有记录，不产生第二条。 */
export function createEntityRelation(data: CreateEntityRelationPayload) {
  return request<EntityRelation>({
    url: '/entity-relations',
    method: 'post',
    data
  })
}

/** 查询关系。空参数即查当前租户可视范围内的全部关系。 */
export function listEntityRelations(params: ListEntityRelationsParams = {}) {
  return request<EntityRelationList>({
    url: '/entity-relations',
    method: 'get',
    params
  })
}

/** 删除单条关系。返回被删除的行数。 */
export function deleteEntityRelation(id: string) {
  return request<{ deleted: number }>({
    url: `/entity-relations/${id}`,
    method: 'delete'
  })
}

/**
 * 删除某实体挂载的全部关系。
 * policy 必须显式声明：protect 会在仍存在关系时拒绝，cascade 才真正删除。
 */
export function deleteEntityRelationsForEntity(data: DeleteByEntityPayload) {
  return request<{ deleted: number }>({
    url: '/entity-relations/by-entity',
    method: 'delete',
    data
  })
}
