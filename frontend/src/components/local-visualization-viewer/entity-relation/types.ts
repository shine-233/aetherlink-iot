/**
 * 文件用途：看板端通用实体关系图谱动态数据源类型定义（ROADMAP P1.1）。
 * 核心逻辑：对标 ThingsBoard 实体关系动态数据源机制（Entity from relations）。
 */

import type { EntityType } from '@/views/device/entity-relation/entity-relation-model'

export type EntityRelationAggregation = 'latest' | 'avg' | 'sum' | 'max' | 'min' | 'count'

export interface EntityRelationSourceConfig {
  enabled: boolean
  rootType: EntityType
  rootId: string
  direction: 'from' | 'to'
  relationType: string
  targetType: EntityType
  targetKey: string
  aggregation?: EntityRelationAggregation
}

export interface EntityRelationEdge {
  id?: string
  from_type: string
  from_id: string
  relation_type: string
  to_type: string
  to_id: string
  metadata?: string | null
}

export interface ResolvedTargetValue {
  targetId: string
  value: unknown
  numericValue?: number
}

export interface ResolveEntityRelationResult {
  matchedTargetIds: string[]
  resolvedValue: string | number | null
  targetValues: ResolvedTargetValue[]
}
